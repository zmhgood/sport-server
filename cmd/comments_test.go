package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyCommentRoutesAreGone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerDisabledCommentRoutes(r.Group("/api"))
	// No database is initialized. Even valid-looking input cannot reach persistence.
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/comments?exercise_id=1", ""},
		{"POST", "/api/comments", `{"exercise_id":1,"content":"test"}`},
		{"POST", "/api/comments", `{"exercise_id":1,"parent_id":1,"content":"reply"}`},
		{"POST", "/api/comments", "invalid json"},
		{"DELETE", "/api/comments/1", ""},
		{"POST", "/api/comments/1/like", ""},
		{"GET", "/api/user/comments", ""},
	} {
		for _, auth := range []string{"", "Bearer retired-client-session"} {
			t.Run(tc.method+tc.path+tc.body+auth, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", auth)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != http.StatusGone {
					t.Fatalf("status=%d", w.Code)
				}
				var body struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Code != http.StatusGone || body.Message != "评论功能已停用" {
					t.Fatalf("unexpected response: %+v", body)
				}
			})
		}
	}
	// The shutdown must not consume unrelated login, upload, or admin routes.
	for _, path := range []string{"/api/auth/login", "/api/upload/image", "/api/admin/comments"} {
		r.POST(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("unrelated route blocked: %s", path)
		}
	}
}
