package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	initDB() // 👈 補上這行，確保測試時資料庫變數 db 已成功初始化！
	return setupRouter()
}

// 測試 1：驗證 GET /api/v1/jobs 是否能成功回傳 200 OK
func TestGetJobs(t *testing.T) {
	router := setupTestEngine()

	req, _ := http.NewRequest("GET", "/api/v1/jobs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// 測試 2：驗證未帶 Token 時 POST 是否被 AuthMiddleware 攔截 (401 Unauthorized)
func TestCreateJob_Unauthorized(t *testing.T) {
	router := setupTestEngine()

	body := strings.NewReader(`{"company":"Test Co","title":"QA","location":"Taiwan"}`)
	req, _ := http.NewRequest("POST", "/api/v1/jobs", body)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// 測試 3：驗證帶正確 Token 時 POST 是否回應 201 Created
func TestCreateJob_Success(t *testing.T) {
	router := setupTestEngine()

	body := strings.NewReader(`{"company":"Google","title":"Android Architect","location":"Taiwan"}`)
	req, _ := http.NewRequest("POST", "/api/v1/jobs", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", "secret123")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}