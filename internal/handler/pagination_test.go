package handler

import (
	"elderly-fitness/internal/repository"
	"elderly-fitness/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"testing"
)

func TestListHandlersBoundPaginationBeforeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, route := range []string{"users", "comments", "exercises", "families"} {
		for _, query := range []string{"?page=-1&page_size=-1", "?page=9223372036854775807&page_size=100", "?page=1&page_size=999999", "?page=1&page_size=100"} {
			t.Run(route+query, func(t *testing.T) {
				db, err := gorm.Open(mysql.New(mysql.Config{DSN: "dryrun:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
				if err != nil {
					t.Fatal("dry-run setup failed")
				}
				bounded := false
				db.Callback().Query().After("gorm:query").Register("qa:limit", func(tx *gorm.DB) {
					if c, ok := tx.Statement.Clauses["LIMIT"]; ok {
						if limit, ok := c.Expression.(clause.Limit); ok {
							if limit.Limit == nil || *limit.Limit < 1 || *limit.Limit > 100 || limit.Offset < 0 {
								t.Fatal("query pagination is unbounded")
							}
							bounded = true
						}
					}
				})
				ur := repository.NewUserRepository(db)
				er := repository.NewExerciseRepository(db)
				us := service.NewUserService(ur, er)
				us.SetCommentRepo(repository.NewCommentRepository(db))
				ah := NewAdminHandler(nil, us)
				r := gin.New()
				switch route {
				case "users":
					r.GET("/", ah.GetUsers)
				case "comments":
					r.GET("/", ah.GetComments)
				case "exercises":
					r.GET("/", NewExerciseHandler(service.NewExerciseService(er, ur)).GetExercises)
				case "families":
					r.GET("/", NewFamilyHandler(service.NewFamilyService(repository.NewFamilyRepository(db))).ListFamiliesForAdmin)
				}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest("GET", "/"+query, nil))
				if !bounded {
					t.Fatal("handler never reached a bounded query")
				}
				if w.Code != 200 {
					t.Fatalf("unexpected status %d", w.Code)
				}
			})
		}
	}
}
