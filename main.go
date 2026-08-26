package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres" 
	"gorm.io/gorm"
)

// 定義職缺資料模型 (嵌入 gorm.Model，自動產生 ID, CreatedAt, UpdatedAt, DeletedAt)
type JobOpportunity struct {
	gorm.Model
	Company  string `json:"company"`
	Title    string `json:"title"`
	Location string `json:"location"`
}

// 資料庫全域變數
var db *gorm.DB

// 初始化資料庫與預設資料
func initDB() {
	// 從環境變數讀取，若無則使用本機預設值
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "user"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "password"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "jobdb"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Taipei",
		host, user, password, dbname, port)

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to PostgreSQL database!")
	}

	// 自動遷移 Schema
	db.AutoMigrate(&JobOpportunity{})

	// 若資料庫內無資料，寫入預設測試資料
	var count int64
	db.Model(&JobOpportunity{}).Count(&count)
	if count == 0 {
		db.Create(&JobOpportunity{Company: "Mercari Japan", Title: "Senior Go/Mobile Engineer", Location: "Japan"})
		db.Create(&JobOpportunity{Company: "Tech Corp TW", Title: "Senior Android Engineer", Location: "Taiwan"})
		fmt.Println("🎉 PostgreSQL database seeded successfully!")
	}
}

// 自訂 API Token 檢查中間件 (類似 OkHttp Interceptor)
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-API-Token")

		// 檢查 Header 是否帶有正確的 Token
		if token != "secret123" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Unauthorized: Invalid or missing API Token",
			})
			c.Abort() // 攔截請求，不繼續往下執行
			return
		}

		c.Next() // 驗證通過，繼續執行後續 Handler
	}
}

// 模擬背景耗時任務 (用 Goroutine 執行)
func logAnalytics(action string) {
	go func() {
		time.Sleep(100 * time.Millisecond) // 模擬非同步寫入 Log
		fmt.Printf("⚡ [Goroutine Background Log] Action recorded: %s at %s\n", action, time.Now().Format("15:04:05"))
	}()
}

func main() {
	// 1. 啟動時初始化 DB (👉 新增這行)
	initDB()

	// 初始化 Gin 引擎
	r := gin.Default()

	// 1. GET 請求：從 DB 取得所有職缺
	r.GET("/api/v1/jobs", func(c *gin.Context) {
		logAnalytics("Fetch All Jobs")

		var jobList []JobOpportunity
		db.Find(&jobList) // 👈 從資料庫 SELECT 所有資料

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   jobList,
		})
	})

	// 2. GET 請求：透過 ID 從 DB 查詢單一職缺
	r.GET("/api/v1/jobs/:id", func(c *gin.Context) {
		id := c.Param("id")
		logAnalytics("Fetch Job ID: " + id)

		var job JobOpportunity
		// 👈 用 GORM 直接向資料庫查詢 ID
		if err := db.First(&job, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Job not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success", "data": job})
	})

	// 3. POST 請求：新增筆職缺寫入 DB (加上 AuthMiddleware 權限保護)
	r.POST("/api/v1/jobs", AuthMiddleware(), func(c *gin.Context) {
		var newJob JobOpportunity
		if err := c.ShouldBindJSON(&newJob); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		db.Create(&newJob) // 👈 寫入資料庫 INSERT

		logAnalytics("Create New Job: " + newJob.Company)
		c.JSON(http.StatusCreated, gin.H{"status": "success", "data": newJob})
	})

	r.Run(":8080")
}