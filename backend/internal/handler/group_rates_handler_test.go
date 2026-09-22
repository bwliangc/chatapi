package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupRatesHandlerRequiresLogin(t *testing.T) {
	h := &GroupRatesHandler{}
	router := gin.New()
	router.GET("/groups/board", h.Get)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/groups/board", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}
