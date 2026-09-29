package admin

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestModelDetectionDisabledEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AccountHandler{}
	for _, handler := range []gin.HandlerFunc{h.ModelDetectionInfo, h.DetectAccountModel, h.GetModelDetectionSchedule, h.SaveModelDetectionSchedule} {
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

type detectionHistoryHandlerRepo struct {
	service.AccountRepository
	service.ModelDetectionHistoryRepository
	id         int64
	page, size int
	exists     bool
}

func (r *detectionHistoryHandlerRepo) ExistsByID(context.Context, int64) (bool, error) {
	return r.exists, nil
}
func (r *detectionHistoryHandlerRepo) ListModelDetectionHistory(_ context.Context, id int64, page, size int) ([]service.ModelDetectionHistoryRecord, int64, error) {
	r.id = id
	r.page = page
	r.size = size
	return []service.ModelDetectionHistoryRecord{{ID: 12, Source: "scheduled", ModelDetectionSnapshot: service.ModelDetectionSnapshot{Status: "consistent", Model: "gpt-6-sol"}}}, 1, nil
}
func TestModelDetectionHistoryHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &detectionHistoryHandlerRepo{exists: true}
	h := &AccountHandler{accountTestService: service.NewAccountTestService(repo, nil, nil, nil, nil, nil, nil, nil)}
	router := gin.New()
	router.GET("/accounts/:id/history", h.GetModelDetectionHistory)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/accounts/42/history?page=2&page_size=1000", nil))
	require.Equal(t, 200, w.Code)
	require.EqualValues(t, 42, repo.id)
	require.Equal(t, 2, repo.page)
	require.Equal(t, 100, repo.size)
	var response struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			}
			Total int64 `json:"total"`
		}
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data.Items, 1)
	require.Equal(t, "12", response.Data.Items[0].ID)
	for _, id := range []string{"0", "-1", "abc"} {
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/accounts/"+id+"/history", nil))
		require.Equal(t, 400, w.Code)
	}
	repo.exists = false
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/accounts/42/history", nil))
	require.Equal(t, 404, w.Code)
}
