package repository

import (
	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"os"
	"strconv"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestListAllLoadsCreatorName(t *testing.T) {
	path := os.Getenv("SPORT_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("local MySQL config required")
	}
	previous := config.AppConfig
	defer func() { config.AppConfig = previous }()
	if err := config.LoadConfig(path); err != nil {
		t.Fatal("cannot load test configuration")
	}
	cfg := config.AppConfig.Database
	if cfg.Host != "127.0.0.1" && cfg.Host != "localhost" {
		t.Fatal("local database required")
	}
	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("cannot connect to test database")
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()

	id := strconv.FormatInt(time.Now().UnixNano(), 36)
	user := model.User{OpenID: "test-" + id, NickName: "测试创建者"}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	family := model.Family{Name: "test-" + id, CreatorID: user.ID, InviteCode: "t" + id}
	if err := tx.Create(&family).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.FamilyMember{FamilyID: family.ID, UserID: user.ID, Role: "admin"}).Error; err != nil {
		t.Fatal(err)
	}

	families, _, err := NewFamilyRepository(tx).ListAll(1, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range families {
		if item.ID == family.ID {
			if len(item.Members) != 1 || item.Members[0].User.NickName != "测试创建者" {
				t.Fatal("creator nickname was not loaded")
			}
			if item.Members[0].User.OpenID != "" {
				t.Fatal("family list exposed more user fields than needed")
			}
			return
		}
	}
	t.Fatal("created family missing from first page")
}
