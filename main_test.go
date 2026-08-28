package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return setupRouter()
}

// 測試 1：驗證海況 API 回應狀態
func TestSeaConditionsRoute(t *testing.T) {
	router := setupTestEngine()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/sea-conditions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// 測試 2：驗證 PostGIS 空間查詢地理圍欄 API (測試缺參與帶參)
func TestNearbyHazardsRoute(t *testing.T) {
	router := setupTestEngine()

	// 缺少 lat/lng 時應回傳 400 Bad Request
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/api/v1/nearby-hazards", nil)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusBadRequest, w1.Code)

	// 帶入座標與半徑時應回傳 200 OK
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/nearby-hazards?lat=25.205&lng=121.690&radius=5000", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

// 測試 3：驗證社群熱點 API
func TestCommunitySpotsRoute(t *testing.T) {
	router := setupTestEngine()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/community-spots", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}