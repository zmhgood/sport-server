package handler

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestProfileRejectsInvalidNumericJSONBeforeDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"weight":66.5}`, `{"height":170.5}`, `{"age":65.5}`, `{"weight":"1 OR 1=1"}`, `{"age":"65"}`, `{"height":1e100}`, `{"weight":1e1000}`} {
		t.Run(body, func(t *testing.T) {
			h := NewAuthHandler(nil, nil)
			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set("userID", uint(1)) })
			r.PUT("/user/info", h.UpdateUserInfo)
			req := httptest.NewRequest("PUT", "/user/info", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 400 {
				t.Fatalf("invalid numeric input was not rejected: %d", w.Code)
			}
		})
	}
}
