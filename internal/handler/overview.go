package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"spring-slumber-server/internal/config"
	"spring-slumber-server/internal/response"
)

// OverviewHandler 服务概览 handler。
type OverviewHandler struct {
	cfg config.Config
}

// OverviewResponse 服务概览响应。
type OverviewResponse struct {
	Service     string    `json:"service"     example:"spring-slumber-server"`
	Environment string    `json:"environment" example:"development"`
	Message     string    `json:"message"     example:"Spring Slumber API is running."`
	Timestamp   time.Time `json:"timestamp"`
}

// NewOverviewHandler 构造 OverviewHandler。
func NewOverviewHandler(cfg config.Config) *OverviewHandler {
	return &OverviewHandler{cfg: cfg}
}

// Get 服务概览。
// @Summary      服务概览
// @Description  返回当前服务名、运行环境、欢迎语与服务器时间，用于连通性自检。
// @Tags         system
// @Produce      json
// @Success      200 {object} OverviewResponse
// @Router       /api/v1/overview [get]
func (h *OverviewHandler) Get(c *gin.Context) {
	response.JSON(c, http.StatusOK, OverviewResponse{
		Service:     h.cfg.App.Name,
		Environment: h.cfg.App.Env,
		Message:     "Spring Slumber API is running.",
		Timestamp:   time.Now().UTC(),
	})
}
