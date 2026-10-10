package middleware

import (
	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"elderly-fitness/internal/service"
	"elderly-fitness/pkg/database"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestFamilyAccessIntegration(t *testing.T) {
	path := os.Getenv("SPORT_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("local MySQL config required")
	}
	oldConfig := config.AppConfig
	defer func() { config.AppConfig = oldConfig }()
	if err := config.LoadConfig(path); err != nil {
		t.Fatal("cannot load test configuration")
	}
	cfg := config.AppConfig.Database
	if cfg.Host != "127.0.0.1" && cfg.Host != "localhost" {
		t.Fatal("integration test requires local database")
	}
	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("cannot connect to local test database")
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	oldDB := database.DB
	database.DB = tx
	defer func() { database.DB = oldDB }()
	prefix := fmt.Sprintf("access-test-%d", time.Now().UnixNano())
	member := model.User{OpenID: prefix + "-member"}
	outsider := model.User{OpenID: prefix + "-outsider"}
	if err := repository.NewUserRepository(tx).Create(&member); err != nil {
		t.Fatal(err)
	}
	if err := repository.NewUserRepository(tx).Create(&outsider); err != nil {
		t.Fatal(err)
	}
	member.NickName = "test-member"
	outsider.NickName = "test-outsider"
	if err := repository.NewUserRepository(tx).Update(&member); err != nil {
		t.Fatal(err)
	}
	if err := repository.NewUserRepository(tx).Update(&outsider); err != nil {
		t.Fatal(err)
	}
	var phoneCount int64
	tx.Model(&model.User{}).Where("id IN ? AND phone IS NULL", []uint{member.ID, outsider.ID}).Count(&phoneCount)
	if phoneCount != 2 {
		t.Fatal("WeChat accounts must retain NULL phone values")
	}
	family := model.Family{Name: prefix, CreatorID: member.ID, InviteCode: fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)}
	if err := tx.Create(&family).Error; err != nil {
		t.Fatal(err)
	}
	fm := model.FamilyMember{FamilyID: family.ID, UserID: member.ID, Role: "admin", JoinedAt: time.Now()}
	if err := tx.Create(&fm).Error; err != nil {
		t.Fatal(err)
	}
	goal := model.DailyGoal{FamilyID: family.ID, Name: prefix, CreatedBy: member.ID, IsActive: true}
	if err := tx.Create(&goal).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name string
		user uint
		goal bool
		path string
		want int
	}{
		{"member family", member.ID, false, fmt.Sprintf("/family/%d", family.ID), 200},
		{"outsider family", outsider.ID, false, fmt.Sprintf("/family/%d", family.ID), 403},
		{"member goal", member.ID, true, fmt.Sprintf("/goals/%d", goal.ID), 200},
		{"outsider goal", outsider.ID, true, fmt.Sprintf("/goals/%d", goal.ID), 403},
		{"invalid ID", member.ID, false, "/family/0", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set("userID", tc.user) })
			route := "/family/:id"
			if tc.goal {
				route = "/goals/:id"
			}
			r.GET(route, FamilyAccessMiddleware(tc.goal), func(c *gin.Context) { c.Status(200) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
		})
	}
	goalService := service.NewGoalService(repository.NewGoalRepository(tx), repository.NewFamilyRepository(tx), repository.NewExerciseRepository(tx))
	_, err = goalService.CreateGoal(family.ID, member.ID, service.CreateGoalInput{Name: prefix, MemberIDs: []uint{outsider.ID}})
	if err == nil {
		t.Fatal("outsider accepted as goal member")
	}
	var count int64
	tx.Model(&model.DailyGoal{}).Where("family_id = ?", family.ID).Count(&count)
	if count != 1 {
		t.Fatal("invalid membership partially created a goal")
	}
	if err := tx.Delete(&fm).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", member.ID) })
	r.GET("/family/:id", FamilyAccessMiddleware(false), func(c *gin.Context) { c.Status(200) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/family/%d", family.ID), nil))
	if w.Code != 403 {
		t.Fatal("removed family member retained access")
	}
}
