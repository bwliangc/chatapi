package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type luckySecondHandlerRepo struct {
	service.LuckySecondRepository
	admin    bool
	viewerID int64
}

func (r *luckySecondHandlerRepo) List(_ context.Context, _, _ int, viewerID int64) ([]service.LuckySecondCampaign, int64, error) {
	r.viewerID = viewerID
	amount := "5.00000000"
	return []service.LuckySecondCampaign{{ID: 1, MyAwardedAmount: &amount}}, 1, nil
}

func TestLuckySecondPersonalTotalUsesAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := &luckySecondHandlerRepo{}
	h := NewLuckySecondHandler(&service.LuckySecondService{Repo: r})
	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 17})
		h.list(c, false)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/?user_id=99", nil))
	require.Equal(t, 200, rec.Code)
	require.EqualValues(t, 17, r.viewerID)
	require.Contains(t, rec.Body.String(), `"my_awarded_amount":"5.00000000"`)
}

func (r *luckySecondHandlerRepo) Slots(_ context.Context, _ int64, admin bool, _, _ int) ([]service.LuckySecondSlot, int64, error) {
	r.admin = admin
	other, me := int64(2), int64(1)
	request := "private-request-id"
	return []service.LuckySecondSlot{{ID: 1, UserID: &other, Name: "another@example.com", RequestID: &request, State: "awarded", SecondAt: time.Now()}, {ID: 2, UserID: &me, Name: "me@example.com", RequestID: &request, State: "awarded"}}, 2, nil
}
func TestLuckySecondPublicAwardsRedactIdentityAndRequestIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := &luckySecondHandlerRepo{}
	h := NewLuckySecondHandler(&service.LuckySecondService{Repo: r})
	router := gin.New()
	router.GET("/:id", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
		h.slots(c, false)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/1", nil))
	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	require.False(t, r.admin)
	require.NotContains(t, body, "another@example.com")
	require.NotContains(t, body, "user_id")
	require.NotContains(t, body, "private-request-id")
	require.Contains(t, body, "me@example.com")
	require.Contains(t, body, `"is_me":true`)
}
