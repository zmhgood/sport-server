package middleware

import (
	"elderly-fitness/config"
	"elderly-fitness/internal/handler"
	"elderly-fitness/internal/repository"
	"elderly-fitness/internal/service"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDashboardHTTPPermissionsRangesAndFailure(t *testing.T) {
	path := os.Getenv("SPORT_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("isolated local MySQL required")
	}
	previous := config.AppConfig
	defer func() { config.AppConfig = previous }()
	if err := config.LoadConfig(path); err != nil {
		t.Fatal("test configuration unavailable")
	}
	cfg := config.AppConfig.Database
	if cfg.Host != "127.0.0.1" || !strings.HasPrefix(cfg.DBName, "sport_qa_") {
		t.Fatal("local test database required")
	}
	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("local database unavailable")
	}
	conn, _ := db.DB()
	defer conn.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	h := handler.NewAdminHandler(nil, service.NewUserService(repository.NewUserRepository(tx), repository.NewExerciseRepository(tx)))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/stats", AdminAuthMiddleware(), h.GetDashboardStats)
	token := func(claims jwt.MapClaims) string {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
		value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.AppConfig.JWT.Secret))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	admin := token(jwt.MapClaims{"admin_id": 1, "type": "admin"})
	user := token(jwt.MapClaims{"user_id": 1})
	call := func(query, auth string, status int) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/stats"+query, nil)
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("query=%s status=%d expected=%d", query, w.Code, status)
		}
		return w
	}
	call("", "", 401)
	call("", user, 403)
	call("", "invalid", 401)
	for _, value := range []string{"0", "-1", "8", "29", "31", "7.0", "7 OR 1=1", "30;DROP TABLE users", "999999999999999999999999999"} {
		call("?days="+url.QueryEscape(value), admin, 400)
	}
	for _, tc := range []struct {
		query string
		days  int
	}{{"", 7}, {"?days=7", 7}, {"?days=30", 30}} {
		w := call(tc.query, admin, 200)
		var body struct {
			Code int                    `json:"code"`
			Data service.DashboardStats `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != 0 || body.Data.Days != tc.days || len(body.Data.DailyActivity) != tc.days || body.Data.TimeZone != "Asia/Shanghai" || body.Data.HotExercises == nil || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("dashboard response contract failed")
		}
	}
	tx.Rollback()
	w := call("?days=7", admin, 500)
	if !strings.Contains(w.Body.String(), "获取统计失败") || strings.Contains(w.Body.String(), "SELECT") || strings.Contains(w.Body.String(), "sql:") {
		t.Fatal("database failure must remain a safe error, not empty success")
	}
}
