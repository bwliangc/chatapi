package admin

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestModelDetectionDisabledEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AccountHandler{}
	for _, handler := range []gin.HandlerFunc{h.ModelDetectionInfo, h.DetectAccountModel} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"gpt-6-sol"}`))
		handler(c)
		if w.Code != 404 || strings.Contains(w.Header().Get("Content-Type"), "event-stream") {
			t.Fatalf("disabled endpoint accessible: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestModelDetectionBankMutationJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"revision":"abc"}`, `{"unknown":true}`, `{"revision":"abc"} {}`, `{"revision":"` + strings.Repeat("a", 2048) + `"}`, `{`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
		var request struct {
			Revision string `json:"revision"`
		}
		ok := decodeModelDetectionBankRequest(c, &request)
		if body == `{"revision":"abc"}` {
			if !ok || request.Revision != "abc" {
				t.Fatal("valid revision rejected")
			}
		} else if ok || w.Code != 400 {
			t.Fatal("malformed mutation accepted")
		}
	}
}
