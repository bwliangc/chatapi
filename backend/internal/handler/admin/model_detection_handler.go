package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AccountHandler) ModelDetectionInfo(c *gin.Context) {
	if !h.accountTestService.ModelDetectionEnabled(c.Request.Context()) {
		response.NotFound(c, "模型检测功能未启用")
		return
	}
	bank, err := h.accountTestService.CurrentModelDetectionBank(c.Request.Context())
	if err != nil {
		response.Error(c, 503, "指纹库不可用")
		return
	}
	models := make([]string, 0, len(bank.Models))
	for _, model := range bank.Models {
		models = append(models, model.ID)
	}
	response.Success(c, gin.H{"models": models, "bank_sha256": bank.SHA256, "bank_built_at": bank.BuiltAt})
}

func (h *AccountHandler) DetectAccountModel(c *gin.Context) {
	if !h.accountTestService.ModelDetectionEnabled(c.Request.Context()) {
		response.NotFound(c, "模型检测功能未启用")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效账号 ID")
		return
	}
	var input struct {
		Model string `json:"model"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.BadRequest(c, "检测参数无效")
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || strings.TrimSpace(input.Model) == "" || len(input.Model) > 160 {
		response.BadRequest(c, "请选择有效模型")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()
	events := make(chan any, 8)
	go func() {
		defer close(events)
		send := func(v any) {
			select {
			case events <- v:
			case <-ctx.Done():
			}
		}
		result, err := h.accountTestService.DetectModel(ctx, id, input.Model, h.concurrencyService, func(p service.ModelDetectionProgress) { send(p) })
		if err != nil {
			send(gin.H{"type": "error", "message": err.Error()})
			return
		}
		send(gin.H{"type": "result", "data": result})
	}()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(c.Writer, ": keepalive\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			raw, err := json.Marshal(event)
			if err != nil {
				return
			}
			if _, err = io.WriteString(c.Writer, "data: "+string(raw)+"\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}
