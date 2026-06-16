// Package sms 提供短信下发能力。
//
// 当前内置一个 LogSender，会把验证码写入日志（开发/测试用）。
// 生产环境请按 SMSConfig.Provider 实现 Aliyun/Tencent 等第三方 Sender。
package sms

import (
	"context"
	"log/slog"

	"spring-slumber-server/internal/config"
)

// Sender 短信下发接口。
type Sender interface {
	// Send 异步下发验证码；返回 nil 即视为「请求已被网关接受」。
	Send(ctx context.Context, phoneNumber, code string) error
}

// NewSender 根据配置构造 Sender。
func NewSender(cfg config.SMSConfig, logger *slog.Logger) Sender {
	switch cfg.Provider {
	case "mock", "":
		return &LogSender{logger: logger}
	default:
		// 接入真实厂商时在此扩展。
		return &LogSender{logger: logger}
	}
}

// LogSender 直接将验证码写入 slog（仅供开发/测试环境使用）。
type LogSender struct {
	logger *slog.Logger
}

// Send 实现 Sender.Send。
func (s *LogSender) Send(_ context.Context, phone, code string) error {
	s.logger.Info("sms verification code (mock provider)", "phone", maskPhone(phone), "code", code)
	return nil
}

// maskPhone 13812345678 -> 138****5678。
func maskPhone(p string) string {
	if len(p) < 7 {
		return p
	}
	return p[:3] + "****" + p[len(p)-4:]
}
