// Package response 提供统一的 JSON 信封与错误响应辅助函数。
//
// 所有 API 响应使用相同信封：
//
//	{ "data": ..., "error": { "code": "...", "message": "..." } }
//
// Gin 适配：所有辅助函数接收 *gin.Context，由调用方负责中止链路与设置状态码。
package response

import (
	"github.com/gin-gonic/gin"
)

// Envelope 统一响应信封。
type Envelope struct {
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

// Error 错误对象。
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON 写入成功响应：{ "data": payload }。
func JSON(c *gin.Context, status int, payload any) {
	c.JSON(status, Envelope{Data: payload})
}

// Problem 写入错误响应：{ "error": { code, message } }。
func Problem(c *gin.Context, status int, code, message string) {
	c.JSON(status, Envelope{
		Error: &Error{
			Code:    code,
			Message: message,
		},
	})
}

// NoContent 写入 204 响应并中止后续 handler。
func NoContent(c *gin.Context) {
	c.Status(204)
}
