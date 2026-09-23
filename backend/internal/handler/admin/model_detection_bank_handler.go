package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Management stays available while detection is disabled, so administrators can
// inspect/prepare the bank before enabling paid account probes.
func (h *AccountHandler) GetModelDetectionBank(c *gin.Context) {
	result, err := h.accountTestService.ModelDetectionBankStatus(c.Request.Context())
	modelDetectionBankResponse(c, result, err)
}
func (h *AccountHandler) CheckModelDetectionBank(c *gin.Context) {
	result, err := h.accountTestService.CheckModelDetectionBank(c.Request.Context())
	modelDetectionBankResponse(c, result, err)
}
func (h *AccountHandler) UpdateModelDetectionBank(c *gin.Context) {
	var request struct {
		Commit   string `json:"commit"`
		Revision string `json:"revision"`
	}
	if !decodeModelDetectionBankRequest(c, &request) {
		return
	}
	result, err := h.accountTestService.UpdateModelDetectionBank(c.Request.Context(), request.Commit, request.Revision)
	modelDetectionBankResponse(c, result, err)
}
func (h *AccountHandler) RollbackModelDetectionBank(c *gin.Context) {
	var request struct {
		Revision string `json:"revision"`
	}
	if !decodeModelDetectionBankRequest(c, &request) {
		return
	}
	result, err := h.accountTestService.RollbackModelDetectionBank(c.Request.Context(), request.Revision)
	modelDetectionBankResponse(c, result, err)
}
func decodeModelDetectionBankRequest(c *gin.Context, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		response.BadRequest(c, "更新参数无效")
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		response.BadRequest(c, "更新参数无效")
		return false
	}
	return true
}
func modelDetectionBankResponse(c *gin.Context, result any, err error) {
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, service.ErrModelDetectionBankConflict) {
			code = http.StatusConflict
		}
		response.Error(c, code, err.Error())
		return
	}
	response.Success(c, result)
}
