package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type GroupRatesHandler struct{ service *service.GroupRatesService }

func NewGroupRatesHandler(groups *service.APIKeyService, usage service.UsageLogRepository) *GroupRatesHandler {
	usageReader, _ := usage.(service.GroupRateUsageRepository)
	return &GroupRatesHandler{service: service.NewGroupRatesService(groups, usageReader)}
}

func (h *GroupRatesHandler) Get(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}
	board, err := h.service.Get(c.Request.Context(), subject.UserID)
	if err != nil {
		response.InternalError(c, "Unable to load group rates and usage")
		return
	}
	response.Success(c, board)
}
