package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 定義職缺資料模型
type JobOpportunity struct {
	ID       string `json:"id"`
	Company  string `json:"company"`
	Title    string `json:"title"`
	Location string `json:"location"` // 例如: "Taiwan" 或 "Japan"
}

// 模擬記憶體資料庫
var jobs = []JobOpportunity{
	{ID: "1", Company: "Mercari Japan", Title: "Senior Go/Mobile Engineer", Location: "Japan"},
	{ID: "2", Company: "Tech Corp TW", Title: "Senior Android Engineer", Location: "Taiwan"},
}

func main() {
	// 初始化 Gin 引擎 (自帶 Logger 與 Recovery 中間件)
	r := gin.Default()

	// 1. GET 請求：取得所有職缺清單
	r.GET("/api/v1/jobs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   jobs,
		})
	})

	// 2. GET 請求：透過 URL 參數 (Path variable) 查詢單一職缺
	r.GET("/api/v1/jobs/:id", func(c *gin.Context) {
		id := c.Param("id")
		for _, job := range jobs {
			if job.ID == id {
				c.JSON(http.StatusOK, gin.H{"status": "success", "data": job})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Job not found"})
	})

	// 3. POST 請求：新增筆職缺 (測試 JSON binding)
	r.POST("/api/v1/jobs", func(c *gin.Context) {
		var newJob JobOpportunity
		// 自動將 Request Body 的 JSON 綁定至 Struct
		if err := c.ShouldBindJSON(&newJob); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}
		jobs = append(jobs, newJob)
		c.JSON(http.StatusCreated, gin.H{"status": "success", "data": newJob})
	})

	// 啟動伺服器，預設監聽 8080 埠
	r.Run(":8080")
}