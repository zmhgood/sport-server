package middleware

import (
	"bytes"
	"elderly-fitness/config"
	"elderly-fitness/internal/handler"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"elderly-fitness/internal/service"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBannerHTTPAuthAndAtomicSave(t *testing.T) {
	path := os.Getenv("SPORT_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("isolated local MySQL required")
	}
	previous := config.AppConfig
	defer func() { config.AppConfig = previous }()
	if err := config.LoadConfig(path); err != nil {
		t.Fatal("test config unavailable")
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
	if err := tx.Unscoped().Where("`key`=?", model.HomeBannersConfigKey).Delete(&model.SystemConfig{}).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewSystemRepository(tx)
	bh := handler.NewBannerHandler(service.NewBannerService(repo, "https://assets.example.test"))
	sh := handler.NewSystemHandler(service.NewSystemService(repo))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/banners", bh.Public)
	admin := r.Group("/admin", AdminAuthMiddleware())
	admin.GET("/banners", bh.Admin)
	admin.PUT("/banners", bh.Save)
	admin.POST("/configs", sh.UpdateConfigs)
	token := func(claims jwt.MapClaims) string {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
		value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.AppConfig.JWT.Secret))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	adminToken := token(jwt.MapClaims{"admin_id": 1, "type": "admin"})
	userToken := token(jwt.MapClaims{"user_id": 1})
	request := func(method, path, auth, body string, want int) model.BannerConfig {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: got %d want %d", method, path, w.Code, want)
		}
		var response struct {
			Code    int                `json:"code"`
			Message string             `json:"message"`
			Data    model.BannerConfig `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if (want == 200) != (response.Code == 0) {
			t.Fatal("business status disagrees with HTTP")
		}
		if strings.Contains(response.Message, "SELECT ") || strings.Contains(response.Message, "Error 1062") {
			t.Fatal("database error leaked")
		}
		return response.Data
	}
	request("GET", "/admin/banners", "", "", 401)
	request("PUT", "/admin/banners", "", "{}", 401)
	request("GET", "/admin/banners", userToken, "", 403)
	request("PUT", "/admin/banners", userToken, "{}", 403)
	request("PUT", "/admin/banners", "invalid", "{}", 401)
	if state := request("GET", "/banners", "", "", 200); state.Configured || len(state.Items) != 0 {
		t.Fatal("unconfigured public payload")
	}
	good := `{"items":[{"id":2,"title":"家庭锻炼","description":"一起动起来","image_url":"https://assets.example.test/uploads/images/2.jpg","enabled":true},{"id":1,"title":"不公开","image_url":"https://assets.example.test/uploads/images/1.png","enabled":false}]}`
	request("PUT", "/admin/banners", adminToken, good, 200)
	if state := request("GET", "/admin/banners", adminToken, "", 200); len(state.Items) != 2 || state.Items[0].ID != 2 {
		t.Fatal("admin order lost")
	}
	if state := request("GET", "/banners", "", "", 200); len(state.Items) != 1 || state.Items[0].Title != "家庭锻炼" {
		t.Fatal("public disabled filtering failed")
	}
	for _, body := range []string{`{}`, `{"items":null}`, `{"items":{}}`, `{"items":[{"id":1.5}]}`, `{"items":[]`, strings.Repeat(" ", 33<<10), strings.Replace(good, "家庭锻炼", strings.Repeat("家", 21), 1), strings.Replace(good, "assets.example.test", "evil.test", 1)} {
		request("PUT", "/admin/banners", adminToken, body, 400)
	}
	if state := request("GET", "/admin/banners", adminToken, "", 200); len(state.Items) != 2 || state.Items[0].Title != "家庭锻炼" {
		t.Fatal("failed batch overwrote config")
	}
	request("POST", "/admin/configs", adminToken, `{"configs":{"home_banners":"[]","banner_http_unrelated":"changed"}}`, 400)
	if _, err := repo.GetConfig("banner_http_unrelated"); err == nil {
		t.Fatal("reserved generic config request partially applied")
	}
	request("PUT", "/admin/banners", adminToken, `{"items":[]}`, 200)
	if state := request("GET", "/banners", "", "", 200); !state.Configured || len(state.Items) != 0 {
		t.Fatal("empty list semantics lost")
	}
	tx.Model(&model.SystemConfig{}).Where("`key`=?", model.HomeBannersConfigKey).Update("value", fmt.Sprint("corrupt"))
	request("GET", "/banners", "", "", 500)
}
