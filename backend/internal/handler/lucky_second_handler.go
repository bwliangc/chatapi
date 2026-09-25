package handler

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type LuckySecondHandler struct{ service *service.LuckySecondService }

func NewLuckySecondHandler(s *service.LuckySecondService) *LuckySecondHandler {
	return &LuckySecondHandler{service: s}
}

func (h *LuckySecondHandler) GatewayMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.service == nil || c.Request.Method != "POST" {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		// Jobs and persistent sessions have different lifecycles and are not entrants.
		eligible := strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/chat/completions") || strings.HasSuffix(path, "/responses") || strings.HasSuffix(path, "/embeddings") || strings.HasSuffix(path, "/images/generations") || strings.HasSuffix(path, "/images/edits") || strings.HasSuffix(path, "/alpha/search") || strings.HasSuffix(path, "/web_search") || strings.HasSuffix(path, "/x_search") || strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent")
		if !eligible {
			c.Next()
			return
		}
		ctx := h.service.Begin(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)
		defer service.ReleaseLuckySecond(ctx)
		c.Next()
	}
}
func (h *LuckySecondHandler) ListAdmin(c *gin.Context) { h.list(c, true) }
func (h *LuckySecondHandler) ListPublic(c *gin.Context) {
	if !h.service.Enabled(c.Request.Context()) {
		response.NotFound(c, "幸运秒活动未开启")
		return
	}
	h.list(c, false)
}
func (h *LuckySecondHandler) list(c *gin.Context, admin bool) {
	var viewerID int64
	if !admin {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			response.Unauthorized(c, "User not authenticated")
			return
		}
		viewerID = subject.UserID
	}
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	items, total, err := h.service.Repo.List(c.Request.Context(), page, size, viewerID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, total, page, size)
}
func (h *LuckySecondHandler) Create(c *gin.Context) {
	var in service.LuckySecondCreate
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "活动配置格式无效")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	slots, err := service.GenerateLuckySecondSchedule(in, time.Now())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	id, err := h.service.Repo.Create(c.Request.Context(), in, slots)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, gin.H{"id": id})
}
func (h *LuckySecondHandler) Cancel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "活动 ID 无效")
		return
	}
	err = h.service.Repo.Cancel(c.Request.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "活动不存在")
		return
	}
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"id": id})
}
func (h *LuckySecondHandler) SlotsAdmin(c *gin.Context) { h.slots(c, true) }
func (h *LuckySecondHandler) SlotsPublic(c *gin.Context) {
	if !h.service.Enabled(c.Request.Context()) {
		response.NotFound(c, "幸运秒活动未开启")
		return
	}
	h.slots(c, false)
}
func (h *LuckySecondHandler) slots(c *gin.Context, admin bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "活动 ID 无效")
		return
	}
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	items, total, err := h.service.Repo.Slots(c.Request.Context(), id, admin, page, size)
	if response.ErrorFrom(c, err) {
		return
	}
	if !admin {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			response.Unauthorized(c, "User not authenticated")
			return
		}
		for i := range items {
			s := &items[i]
			s.IsMe = s.UserID != nil && *s.UserID == subject.UserID
			if !s.IsMe {
				s.Name = maskLeaderboardEmail(s.Name)
			}
			s.UserID = nil
			s.RequestID = nil
		}
	}
	response.Paginated(c, items, total, page, size)
}
