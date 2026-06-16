// Package service 实现 user feature 的核心业务（手机号 + 短信验证码登录）。
//
// 业务策略：
//   - 发送验证码：同 IP / 同手机号 1 分钟内只能请求 1 次；明文不入库，存 SHA256 摘要。
//   - 登录：校验摘要后立即将记录置为已使用（一次性）。
//   - 用户不存在时自动注册（首次登录即注册），见 EnsureUser 的注释。
//   - 单手机号连续 AUTH_MAX_VERIFY_ATTEMPTS 次错误，锁定 AUTH_LOCK_DURATION。
//   - 业务日志中：手机号脱敏、验证码仅记录哈希前 4 / 后 4 位。
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"spring-slumber-server/internal/app/user/dao"
	"spring-slumber-server/internal/app/user/model"
	"spring-slumber-server/internal/auth"
	"spring-slumber-server/internal/config"
	"spring-slumber-server/internal/service/ratelimit"
	"spring-slumber-server/internal/service/sms"
)

// 业务级错误。
var (
	ErrInvalidPhone      = errors.New("invalid phone number")
	ErrInvalidCode       = errors.New("invalid verification code")
	ErrSendTooFrequent   = errors.New("send code too frequent")
	ErrLoginTooFrequent  = errors.New("login too frequent")
	ErrAccountLocked     = errors.New("account is locked")
	ErrCodeExpiredOrUsed = errors.New("verification code expired or already used")
)

// PhoneRe 中国大陆手机号（11 位）；如需支持海外，请替换为 libphonenumber。
var PhoneRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

// Service 业务编排。
type Service struct {
	cfg     config.Config
	authCfg config.AuthConfig
	jwtCfg  config.JWTConfig
	logger  *slog.Logger
	users   *dao.UserDAO
	codes   *dao.VerificationCodeDAO
	sms     sms.Sender
	limiter ratelimit.Limiter
	issuer  *auth.Issuer
}

// NewService 构造业务 Service。
func NewService(
	cfg config.Config,
	logger *slog.Logger,
	users *dao.UserDAO,
	codes *dao.VerificationCodeDAO,
	sender sms.Sender,
	limiter ratelimit.Limiter,
	issuer *auth.Issuer,
) *Service {
	return &Service{
		cfg:     cfg,
		authCfg: cfg.Auth,
		jwtCfg:  cfg.JWT,
		logger:  logger,
		users:   users,
		codes:   codes,
		sms:     sender,
		limiter: limiter,
		issuer:  issuer,
	}
}

// SendCodeResult 发送验证码结果。
type SendCodeResult struct {
	ExpireSeconds int `json:"expireSeconds"`
}

// SendCode 下发验证码。
func (s *Service) SendCode(ctx context.Context, phone, ip string) (*SendCodeResult, error) {
	if !IsValidPhone(phone) {
		return nil, ErrInvalidPhone
	}
	cooldown := s.authCfg.SendCodeCooldown
	if !s.limiter.Cooldown("send:ip:"+ip, cooldown) || !s.limiter.Cooldown("send:phone:"+phone, cooldown) {
		return nil, ErrSendTooFrequent
	}

	code := generateCode(6)
	hash := hashCode(phone, code, s.authCfg.CodeSalt)

	vc := &model.VerificationCode{
		PhoneNumber: phone,
		CodeHash:    hash,
		Purpose:     model.PurposeLogin,
		Used:        false,
		ExpireAt:    time.Now().Add(s.authCfg.CodeTTL),
	}
	if err := s.codes.Insert(ctx, vc); err != nil {
		return nil, fmt.Errorf("insert verification code: %w", err)
	}

	// 短信下发失败不影响主流程（验证码已落库，前端可提示重试）。
	if err := s.sms.Send(ctx, phone, code); err != nil {
		s.logger.Warn("sms send failed",
			"phone", maskPhone(phone),
			"code_hash_prefix", safePrefix(hash),
			"error", err.Error(),
		)
	}

	s.logger.Info("verification code sent",
		"phone", maskPhone(phone),
		"ip", ip,
		"code_hash_prefix", safePrefix(hash),
		"expire_at", vc.ExpireAt,
	)

	return &SendCodeResult{ExpireSeconds: int(s.authCfg.CodeTTL.Seconds())}, nil
}

// LoginResult 登录成功数据。
type LoginResult struct {
	Token       string `json:"token"`
	PhoneNumber string `json:"phoneNumber"`
	UID         int64  `json:"uid"`
}

// Login 手机号 + 验证码登录。
//
// 用户不存在时自动注册（首次登录即注册）——若需改成「用户不存在即报错」，
// 请把 EnsureUser 的 autoRegister 参数置为 false 并调整对应错误返回。
func (s *Service) Login(ctx context.Context, phone, code, ip string) (*LoginResult, error) {
	if !IsValidPhone(phone) {
		return nil, ErrInvalidPhone
	}
	if !isValidCodeFormat(code) {
		return nil, ErrInvalidCode
	}
	if !s.limiter.Allow("login:ip:"+ip, s.authCfg.LoginIPRateLimit, s.authCfg.LoginIPRateWindow) {
		return nil, ErrLoginTooFrequent
	}

	user, err := s.users.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, dao.ErrUserNotFound) {
		return nil, fmt.Errorf("query user: %w", err)
	}

	// 用户不存在：自动注册（首次登录即注册）。
	if user == nil {
		user, err = s.EnsureUser(ctx, phone, true)
		if err != nil {
			return nil, fmt.Errorf("ensure user: %w", err)
		}
	}

	// 账号锁定中：先看是否已过解锁时间。
	if user.Status == model.UserStatusLocked {
		if user.LockTime == nil || time.Now().After(*user.LockTime) {
			_ = s.users.UpdateLoginMeta(ctx, user.ID)
			user.Status = model.UserStatusActive
		} else {
			s.logger.Warn("login blocked by lock", "phone", maskPhone(phone), "lock_time", user.LockTime)
			return nil, ErrAccountLocked
		}
	}

	// 验证码校验。
	vc, err := s.codes.LatestActive(ctx, phone, model.PurposeLogin)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCodeExpiredOrUsed
		}
		return nil, fmt.Errorf("query verification code: %w", err)
	}
	if hashCode(phone, code, s.authCfg.CodeSalt) != vc.CodeHash {
		s.handleVerifyFailure(ctx, user, phone)
		return nil, ErrInvalidCode
	}

	// 校验通过：消费验证码 + 更新登录元信息。
	if err := s.codes.MarkUsed(ctx, vc.ID); err != nil {
		return nil, fmt.Errorf("mark code used: %w", err)
	}
	if err := s.users.UpdateLoginMeta(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("update login meta: %w", err)
	}

	token, _, err := s.issuer.Issue(user.ID, user.PhoneNumber)
	if err != nil {
		return nil, fmt.Errorf("issue jwt: %w", err)
	}

	s.logger.Info("login success", "uid", user.ID, "phone", maskPhone(phone))
	return &LoginResult{Token: token, PhoneNumber: user.PhoneNumber, UID: user.ID}, nil
}

// EnsureUser 查询或创建用户。
//
// autoRegister=true：用户不存在时自动注册（首次登录即注册）。
// autoRegister=false：返回 dao.ErrUserNotFound，调用方决定如何响应。
func (s *Service) EnsureUser(ctx context.Context, phone string, autoRegister bool) (*model.User, error) {
	u, err := s.users.GetByPhone(ctx, phone)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, dao.ErrUserNotFound) {
		return nil, err
	}
	if !autoRegister {
		return nil, dao.ErrUserNotFound
	}
	nick := "用户" + phone[len(phone)-4:]
	created, err := s.users.Create(ctx, &model.User{
		PhoneNumber: phone,
		Nickname:    nick,
		Avatar:      "",
		Status:      model.UserStatusActive,
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("user auto-registered", "uid", created.ID, "phone", maskPhone(phone))
	return created, nil
}

// handleVerifyFailure 单手机号连续错误处理：超过阈值则锁定。
//
// 实现：使用 ratelimit.Allow 在「10 分钟窗口」内累计失败次数；
// 第 1 次返回 true、第 N+1 次超出 N>limit 时返回 false。
func (s *Service) handleVerifyFailure(ctx context.Context, user *model.User, phone string) {
	const window = 10 * time.Minute
	key := "verify_fail:phone:" + phone
	if s.limiter.Allow(key, s.authCfg.MaxVerifyAttempts, window) {
		return
	}
	lockUntil := time.Now().Add(s.authCfg.LockDuration)
	if err := s.users.Lock(ctx, user.ID, lockUntil); err != nil {
		s.logger.Error("lock user failed", "uid", user.ID, "error", err.Error())
		return
	}
	s.logger.Warn("user locked due to too many verify failures",
		"uid", user.ID, "phone", maskPhone(phone), "lock_until", lockUntil)
}

// IsValidPhone 校验手机号。
func IsValidPhone(phone string) bool {
	return PhoneRe.MatchString(strings.TrimSpace(phone))
}

func isValidCodeFormat(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// generateCode 生成 n 位数字验证码。
func generateCode(n int) string {
	const digits = "0123456789"
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		// 加密随机，无偏置。
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			// 极端情况下回退时间戳末位。
			out[i] = digits[time.Now().UnixNano()%int64(len(digits))]
			continue
		}
		out[i] = digits[nBig.Int64()]
	}
	return string(out)
}

// hashCode 计算 SHA256(phone + code + salt)。
func hashCode(phone, code, salt string) string {
	h := sha256.New()
	h.Write([]byte(phone))
	h.Write([]byte{0x00})
	h.Write([]byte(code))
	h.Write([]byte{0x00})
	h.Write([]byte(salt))
	return hex.EncodeToString(h.Sum(nil))
}

func maskPhone(p string) string {
	if len(p) < 7 {
		return p
	}
	return p[:3] + "****" + p[len(p)-4:]
}

func safePrefix(hash string) string {
	if len(hash) <= 8 {
		return hash
	}
	return hash[:4] + "..." + hash[len(hash)-4:]
}
