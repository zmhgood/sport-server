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

func TestFamilyRenameAuthenticatedHTTP(t *testing.T) {
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
	ur := repository.NewUserRepository(db)
	users := make([]model.User, 3)
	for i := range users {
		users[i] = model.User{OpenID: fmt.Sprintf("qa-rename-http-%d-%d", time.Now().UnixNano(), i)}
		if err := ur.Create(&users[i]); err != nil {
			t.Fatal(err)
		}
	}
	fs := service.NewFamilyService(repository.NewFamilyRepository(db))
	family, err := fs.CreateFamily("Synthetic HTTP family", users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.JoinFamily(family.InviteCode, users[1].ID); err != nil {
		t.Fatal(err)
	}
	// Synthetic local signing key; never use a production credential for testing.
	config.AppConfig.JWT.Secret = "local-rename-integration-test-key"
	token := func(uid uint) string {
		value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": uid, "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(config.AppConfig.JWT.Secret))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/family/:id/name", AuthMiddleware(), handler.NewFamilyHandler(fs).RenameFamily)
	request := func(id, auth, body string, want int) {
		t.Helper()
		req := httptest.NewRequest("PUT", "/family/"+id+"/name", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var response struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if w.Code != want || (want == 200) != (response.Code == 0) {
			t.Fatalf("HTTP=%d code=%d want=%d", w.Code, response.Code, want)
		}
		if strings.Contains(response.Message, "Error 1062") || strings.Contains(response.Message, "UPDATE ") {
			t.Fatal("database details exposed")
		}
	}
	id := fmt.Sprint(family.ID)
	admin := token(users[0].ID)
	request(id, "", `{"name":"New"}`, 401)
	request(id, "invalid", `{"name":"New"}`, 401)
	request(id, token(users[1].ID), `{"name":"New"}`, 403)
	request(id, token(users[2].ID), `{"name":"New"}`, 403)
	for _, badID := range []string{"0", "-1", "abc", "4294967296"} {
		request(badID, admin, `{"name":"New"}`, 400)
	}
	request("4294967295", admin, `{"name":"New"}`, 404)
	for _, body := range []string{`{}`, `{"name":null}`, `{"name":3}`, `{"name":""}`, `{"name":"   "}`, `{"name":"Bad\nname"}`, `{"name":"` + strings.Repeat("家", 65) + `"}`, `{`} {
		request(id, admin, body, 400)
	}
	current, err := fs.GetFamily(family.ID)
	if err != nil || current.Name != family.Name {
		t.Fatal("failed request changed family")
	}
	request(id, admin, `{"name":"  HTTP 新家  ","creator_id":999,"invite_code":"OVERRIDE","role":"admin","id":999}`, 200)
	current, err = fs.GetFamily(family.ID)
	if err != nil || current.Name != "HTTP 新家" || current.CreatorID != family.CreatorID || current.InviteCode != family.InviteCode {
		t.Fatal("rename persisted wrong fields")
	}
	request(id, admin, `{"name":"HTTP 新家"}`, 200)
	if err := fs.TransferAdmin(family.ID, users[0].ID, users[1].ID); err != nil {
		t.Fatal(err)
	}
	request(id, admin, `{"name":"Old admin"}`, 403)
	request(id, token(users[1].ID), `{"name":"Current admin"}`, 200)
}
