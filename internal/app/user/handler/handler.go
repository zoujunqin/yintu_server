// Package handler 提供 user feature 的 HTTP handler。
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"spring-slumber-server/internal/app/user/service"
	"spring-slumber-server/internal/response"
)

// Handler 聚合 user feature 的 endpoint（发送验证码、登录）。
type Handler struct {
	svc    *service.Service
	logger *slog.Logger
}

// NewHandler 构造 Handler。
func NewHandler(svc *service.Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// sendCodeRequest 发送验证码请求体。
type sendCodeRequest struct {
	PhoneNumber string `json:"phoneNumber" example:"13800138000" binding:"required"`
}

// sendCodeResponse 发送验证码响应体。
type sendCodeResponse struct {
	ExpireSeconds int `json:"expireSeconds" example:"300"`
}

// SendCode 下发短信验证码。
// @Summary      发送短信验证码
// @Description  向指定手机号下发 6 位数字验证码。同 IP / 同手机号默认 1 分钟内只能请求 1 次。
// @Description  验证码以 SHA256 摘要落库，明文不入库。
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        body body sendCodeRequest true "手机号"
// @Success      200 {object} sendCodeResponse
// @Failure      400 {object} response.Error "参数无效（invalid_argument）"
// @Failure      429 {object} response.Error "发送过于频繁（rate_limited）"
// @Failure      500 {object} response.Error "服务异常"
// @Router       /api/v1/user/send-code [post]
func (h *Handler) SendCode(c *gin.Context) {
	var req sendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Problem(c, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	res, err := h.svc.SendCode(c.Request.Context(), req.PhoneNumber, clientIP(c))
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, sendCodeResponse{ExpireSeconds: res.ExpireSeconds})
}

// loginRequest 登录请求体。
type loginRequest struct {
	PhoneNumber      string `json:"phoneNumber"      example:"13800138000"  binding:"required"`
	VerificationCode string `json:"verificationCode" example:"123456"        binding:"required"`
}

// loginResponse 登录响应体。
type loginResponse struct {
	Token       string `json:"token"       example:"eyJhbGciOiJIUzI1NiIs..."`
	PhoneNumber string `json:"phoneNumber" example:"13800138000"`
	UID         int64  `json:"uid"         example:"10001"`
}

// Login 手机号 + 短信验证码登录。
// @Summary      手机号 + 验证码登录
// @Description  校验通过后返回 JWT（HS256，默认 2 小时有效）。用户不存在时自动注册。
// @Description  连续错误次数超阈值会触发账号临时锁定（默认 10 分钟）。
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        body body loginRequest true "登录凭据"
// @Success      200 {object} loginResponse
// @Failure      400 {object} response.Error "参数无效 / 验证码过期（invalid_argument | code_expired）"
// @Failure      403 {object} response.Error "账号被锁定（account_locked）"
// @Failure      429 {object} response.Error "登录过于频繁（rate_limited）"
// @Failure      500 {object} response.Error "服务异常"
// @Router       /api/v1/user/login [post]
// @Security     BearerAuth
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Problem(c, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	res, err := h.svc.Login(c.Request.Context(), req.PhoneNumber, req.VerificationCode, clientIP(c))
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, loginResponse{
		Token:       res.Token,
		PhoneNumber: res.PhoneNumber,
		UID:         res.UID,
	})
}

// writeAuthError 把业务错误映射到 HTTP 状态码 + 统一响应。
func writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidPhone),
		errors.Is(err, service.ErrInvalidCode):
		response.Problem(c, http.StatusBadRequest, "invalid_argument", err.Error())
	case errors.Is(err, service.ErrSendTooFrequent),
		errors.Is(err, service.ErrLoginTooFrequent):
		response.Problem(c, http.StatusTooManyRequests, "rate_limited", err.Error())
	case errors.Is(err, service.ErrAccountLocked):
		response.Problem(c, http.StatusForbidden, "account_locked", err.Error())
	case errors.Is(err, service.ErrCodeExpiredOrUsed):
		response.Problem(c, http.StatusBadRequest, "code_expired", err.Error())
	default:
		response.Problem(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// clientIP 优先 X-Forwarded-For，其次 X-Real-IP，最后 RemoteAddr。
func clientIP(c *gin.Context) string {
	if v := c.GetHeader("X-Forwarded-For"); v != "" {
		return v
	}
	if v := c.GetHeader("X-Real-IP"); v != "" {
		return v
	}
	return c.Request.RemoteAddr
}
