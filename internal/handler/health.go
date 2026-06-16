package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"spring-slumber-server/internal/config"
	"spring-slumber-server/internal/response"
)

// HealthHandler 健康检查 handler。
type HealthHandler struct {
	cfg config.Config
}

// HealthResponse 健康检查响应。
type HealthResponse struct {
	Service   string    `json:"service" example:"spring-slumber-server"`
	Status    string    `json:"status"  example:"ok"`
	Version   string    `json:"version" example:"dev"`
	Timestamp time.Time `json:"timestamp"`
}

// NewHealthHandler 构造 HealthHandler。
func NewHealthHandler(cfg config.Config) *HealthHandler {
	return &HealthHandler{cfg: cfg}
}

// WelcomeResponse 根路由响应。
type WelcomeResponse struct {
	Service string `json:"service" example:"spring-slumber-server"`
	Status  string `json:"status"  example:"ok"`
}

// Welcome 根路由：返回服务名与状态，用于最简连通性自检。
//
// @Summary      服务欢迎页
// @Description  返回服务名与状态，用于最简连通性自检。
// @Tags         system
// @Produce      json
// @Success      200 {object} WelcomeResponse
// @Router       / [get]
func (h *HealthHandler) Welcome(c *gin.Context) {
	response.JSON(c, http.StatusOK, WelcomeResponse{
		Service: h.cfg.App.Name,
		Status:  "ok",
	})
}

// Healthz 存活探针：进程是否在运行。
// @Summary      健康检查（liveness）
// @Description  进程存活探针；不依赖任何外部依赖（DB / 短信网关等）。
// @Tags         system
// @Produce      json
// @Success      200 {object} HealthResponse
// @Router       /healthz [get]
func (h *HealthHandler) Healthz(c *gin.Context) {
	response.JSON(c, http.StatusOK, HealthResponse{
		Service:   h.cfg.App.Name,
		Status:    "ok",
		Version:   h.cfg.App.Version,
		Timestamp: time.Now().UTC(),
	})
}

// Readyz 就绪探针：进程是否可以接收流量。
// @Summary      健康检查（readiness）
// @Description  就绪探针；当前实现等同于 liveness，预留用于后续接入 DB 健康检查。
// @Tags         system
// @Produce      json
// @Success      200 {object} HealthResponse
// @Router       /readyz [get]
func (h *HealthHandler) Readyz(c *gin.Context) {
	response.JSON(c, http.StatusOK, HealthResponse{
		Service:   h.cfg.App.Name,
		Status:    "ready",
		Version:   h.cfg.App.Version,
		Timestamp: time.Now().UTC(),
	})
}
