package middleware

import (
	"elderly-fitness/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRoleBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := config.AppConfig
	config.AppConfig = &config.Config{JWT: config.JWTConfig{Secret: "local-test-signing-key"}}
	defer func() { config.AppConfig = previous }()
	cases := []struct {
		name                string
		claims              jwt.MapClaims
		method              jwt.SigningMethod
		user, admin, upload int
	}{
		{"user", jwt.MapClaims{"user_id": 1}, jwt.SigningMethodHS256, 200, 403, 200},
		{"admin", jwt.MapClaims{"admin_id": 1, "type": "admin"}, jwt.SigningMethodHS256, 403, 200, 200},
		{"admin without role", jwt.MapClaims{"admin_id": 1}, jwt.SigningMethodHS256, 403, 403, 403},
		{"mixed identities", jwt.MapClaims{"user_id": 1, "admin_id": 1, "type": "admin"}, jwt.SigningMethodHS256, 403, 403, 403},
		{"invalid ID", jwt.MapClaims{"user_id": -1}, jwt.SigningMethodHS256, 401, 403, 401},
		{"fractional ID", jwt.MapClaims{"user_id": 1.5}, jwt.SigningMethodHS256, 401, 403, 401},
		{"wrong algorithm", jwt.MapClaims{"user_id": 1}, jwt.SigningMethodHS512, 401, 401, 401},
		{"expired", jwt.MapClaims{"user_id": 1, "exp": time.Now().Add(-time.Hour).Unix()}, jwt.SigningMethodHS256, 401, 401, 401},
		{"no expiry", jwt.MapClaims{"user_id": 1, "exp": nil}, jwt.SigningMethodHS256, 401, 401, 401},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := tc.claims["exp"]; !ok {
				tc.claims["exp"] = time.Now().Add(time.Hour).Unix()
			}
			signed, err := jwt.NewWithClaims(tc.method, tc.claims).SignedString([]byte(config.AppConfig.JWT.Secret))
			if err != nil {
				t.Fatal(err)
			}
			for _, target := range []struct {
				name       string
				middleware gin.HandlerFunc
				want       int
			}{{"user", AuthMiddleware(), tc.user}, {"admin", AdminAuthMiddleware(), tc.admin}, {"upload", UploadAuthMiddleware(), tc.upload}} {
				r := gin.New()
				r.GET("/", target.middleware, func(c *gin.Context) { c.Status(200) })
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Authorization", "Bearer "+signed)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != target.want {
					t.Fatalf("%s: got %d want %d", target.name, w.Code, target.want)
				}
			}
		})
	}
	r := gin.New()
	r.GET("/", AdminAuthMiddleware(), func(c *gin.Context) { c.Status(200) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 401 {
		t.Fatalf("missing authorization: %d", w.Code)
	}
}
