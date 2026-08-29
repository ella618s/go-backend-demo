package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const CWA_API_KEY = "CWA-D5F5E26E-A7DE-48FE-832E-83B2945E2D43"

type JobOpportunity struct {
	gorm.Model
	Company  string `json:"company"`
	Title    string `json:"title"`
	Location string `json:"location"`
}

var db *gorm.DB

// ---------------------------------------------------
// WebSocket 全局管理 (Hub)
// ---------------------------------------------------
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允許跨網域連線
	},
}

type ClientHub struct {
	clients   map[*websocket.Conn]bool
	broadcast chan []byte
	mutex     sync.Mutex
}

var hub = ClientHub{
	clients:   make(map[*websocket.Conn]bool),
	broadcast: make(chan []byte),
}

func startWebSocketHub() {
	for {
		msg := <-hub.broadcast
		hub.mutex.Lock()
		for client := range hub.clients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				client.Close()
				delete(hub.clients, client)
			}
		}
		hub.mutex.Unlock()
	}
}

// ---------------------------------------------------
// Database 初始化 (含 PostGIS)
// ---------------------------------------------------
func initDB() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// 優先取用 docker-compose 設定的環境變數組合
		dbHost := os.Getenv("DB_HOST")
		dbUser := os.Getenv("DB_USER")
		dbPassword := os.Getenv("DB_PASSWORD")
		dbName := os.Getenv("DB_NAME")
		dbPort := os.Getenv("DB_PORT")
		if dbHost != "" {
			dbURL = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
				dbHost, dbUser, dbPassword, dbName, dbPort)
		}
	}

	if dbURL == "" {
		log.Println("⚠️ 未偵測到資料庫連線字串，跳過 DB 初始化")
		return
	}

	var err error
	db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Printf("⚠️ PostgreSQL 連線失敗: %v", err)
		return
	}

	// 👈 階段二重點：自動啟用 PostGIS 空間幾何擴充
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS postgis;").Error; err != nil {
		log.Printf("⚠️ 啟用 PostGIS 擴充失敗: %v", err)
	} else {
		log.Println("✅ PostGIS 空間資料庫擴充啟用成功！")
	}

	db.AutoMigrate(&JobOpportunity{})
}

func fetchRealSeaConditions() ([]gin.H, error) {
	url := fmt.Sprintf("https://opendata.cwa.gov.tw/api/v1/rest/datastore/C-B0024-001?Authorization=%s", CWA_API_KEY)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ CWA API HTTP 請求失敗: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		log.Printf("❌ Dynamic JSON 解析失敗: %v", err)
		return nil, err
	}

	var results []gin.H

	records, ok := rawData["records"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("records field not found or invalid")
	}

	locations, ok := records["location"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("location list not found or invalid")
	}

	for i := 0; i < len(locations); i++ {
		locMap, ok := locations[i].(map[string]interface{})
		if !ok {
			continue
		}

		stationName := "未知測站"
		stationID := "N/A"
		if station, ok := locMap["station"].(map[string]interface{}); ok {
			if name, ok := station["StationName"].(string); ok && name != "" {
				stationName = name
			}
			if id, ok := station["StationID"].(string); ok && id != "" {
				stationID = id
			}
		}

		windSpeed := "1.2"
		if obsTimesMap, ok := locMap["stationObsTimes"].(map[string]interface{}); ok {
			if obsTimeList, ok := obsTimesMap["stationObsTime"].([]interface{}); ok && len(obsTimeList) > 0 {
				if firstObs, ok := obsTimeList[0].(map[string]interface{}); ok {
					if elements, ok := firstObs["weatherElements"].(map[string]interface{}); ok {
						if w, ok := elements["WindSpeed"].(string); ok && w != "" && w != "-99" {
							windSpeed = w
						} else if wNum, ok := elements["WindSpeed"].(float64); ok {
							windSpeed = fmt.Sprintf("%.1f", wNum)
						}
					}
				}
			}
		}

		results = append(results, gin.H{
			"location_name":  stationName,
			"wave_height_m":  "1.2",
			"wind_speed_kts": windSpeed,
			"tide_info":      "測站編號: " + stationID,
			"updated_at":     time.Now().Format("2006-01-02 15:04"),
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no valid location parsed")
	}

	return results, nil
}

func setupRouter() *gin.Engine {
	r := gin.Default()
	// 允許所有來源（包含 Wasm 網頁 localhost）跨域存取 API
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 1. HTTP 輪詢備用 API
	r.GET("/api/v1/sea-conditions", func(c *gin.Context) {
		seaData, err := fetchRealSeaConditions()
		if err != nil {
			log.Printf("⚠️ 觸發降級備援模式，原因: %v", err)
			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"data": []gin.H{
					{
						"location_name":  "基隆八斗子 (備援)",
						"wave_height_m":  "1.2",
						"wind_speed_kts": "14",
						"tide_info":      "乾潮 14:20 / 滿潮 20:45",
						"updated_at":     time.Now().Format("2006-01-02 15:04"),
					},
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   seaData,
		})
	})

	// 2. 階段二重點：WebSocket 實時海況推播 API
	r.GET("/ws/sea-conditions", func(c *gin.Context) {
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("WebSocket Upgrade Error:", err)
			return
		}
		hub.mutex.Lock()
		hub.clients[ws] = true
		hub.mutex.Unlock()

		// 建立連線時，先推送第一筆當前海況
		if seaData, err := fetchRealSeaConditions(); err == nil {
			msg, _ := json.Marshal(gin.H{"type": "REALTIME_UPDATE", "data": seaData})
			ws.WriteMessage(websocket.TextMessage, msg)
		}
	})

	// 3. 階段二重點：PostGIS 空間查詢地理圍欄 (Geofencing) API
	r.GET("/api/v1/nearby-hazards", func(c *gin.Context) {
		lat := c.Query("lat")
		lng := c.Query("lng")
		radius := c.DefaultQuery("radius", "5000") // 預設 5000 公尺

		if lat == "" || lng == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing lat or lng parameter"})
			return
		}

		// 利用 PostGIS ST_DWithin 幾何計算指令進行空間查詢 (範例回傳結構)
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"query":  gin.H{"lat": lat, "lng": lng, "radius_meters": radius},
			"hazards": []gin.H{
				{
					"id":          101,
					"name":        "基隆嶼外海風場強風警戒區",
					"hazard_type": "HIGH_WIND_ZONE",
					"distance_m":  1240.5,
				},
			},
		})
	})

	r.GET("/api/v1/community-spots", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": []gin.H{
				{
					"id":           1,
					"name":         "野柳沉船點",
					"latitude":     25.205,
					"longitude":    121.690,
					"fish_type":    "紅甘 / 軟絲",
					"depth_meters": 28.5,
					"created_by":   "Captain_Jack",
				},
			},
		})
	})

	return r
}

func main() {
	go startWebSocketHub() // 啟動背景 WebSocket 廣播引擎
	initDB()
	r := setupRouter()
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}