package handler

import (
	"bytes"
	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"elderly-fitness/internal/service"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProfileHTTPPersistenceAndFamilyFailure(t *testing.T) {
	path := os.Getenv("SPORT_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("isolated local MySQL required")
	}
	if err := config.LoadConfig(path); err != nil {
		t.Fatal("test config unavailable")
	}
	cfg := config.AppConfig.Database
	if (cfg.Host != "localhost" && cfg.Host != "127.0.0.1") || !strings.HasPrefix(cfg.DBName, "sport_qa_") {
		t.Fatal("isolated local MySQL required")
	}
	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("local connection failed")
	}
	conn, _ := db.DB()
	defer conn.Close()
	user := model.User{OpenID: fmt.Sprintf("qa-http-%d", time.Now().UnixNano())}
	ur := repository.NewUserRepository(db)
	if err := ur.Create(&user); err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(service.NewAuthService(ur, nil), nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", user.ID) })
	r.PUT("/user/info", h.UpdateUserInfo)
	r.GET("/user/info", h.GetUserInfo)
	req := httptest.NewRequest("PUT", "/user/info", bytes.NewBufferString(`{"nick_name":"Synthetic HTTP","gender":2,"height":171,"weight":66.5,"age":65,"health_status":"Synthetic"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)
	if response.Code != 200 {
		t.Fatal("profile write failed")
	}
	response = httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest("GET", "/user/info", nil))
	var body struct {
		Code int
		Data model.User
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 || body.Data.Height != 171 || body.Data.Weight != 66.5 || body.Data.Gender != 2 || body.Data.NickName != "Synthetic HTTP" {
		t.Fatal("HTTP contract did not persist profile")
	}

	uh := NewUserHandler(service.NewUserService(ur, repository.NewExerciseRepository(db)))
	r.GET("/user/statistics/by-family", uh.GetFamilyStatistics)
	response = httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest("GET", "/user/statistics/by-family", nil))
	var familyStats struct {
		Code int
		Data []repository.FamilyStatistics
	}
	if err := json.Unmarshal(response.Body.Bytes(), &familyStats); err != nil || response.Code != 200 || familyStats.Code != 0 || familyStats.Data == nil || len(familyStats.Data) != 0 {
		t.Fatal("empty family statistics must be a successful JSON array, not null")
	}
	fh := NewFamilyHandler(service.NewFamilyService(repository.NewFamilyRepository(db)))
	r.GET("/families", fh.GetMyFamilies)
	conn.Close()
	response = httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest("GET", "/families", nil))
	if response.Code != 500 {
		t.Fatal("database failure became successful empty family")
	}
}
