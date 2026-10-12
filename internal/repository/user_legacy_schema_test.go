package repository

import (
	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"os"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestWeChatCreateWithLegacyUsername(t *testing.T) {
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
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	// Temporary table and one connection keep all fixture writes isolated from real users.
	if err := db.Exec("CREATE TEMPORARY TABLE sport_user_legacy_fixture LIKE users").Error; err != nil {
		t.Fatal(err)
	}
	defer db.Exec("DROP TEMPORARY TABLE sport_user_legacy_fixture")
	var legacyColumns int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?", "users", "username").Scan(&legacyColumns).Error; err != nil {
		t.Fatal(err)
	}
	if legacyColumns == 0 {
		if err := db.Exec("ALTER TABLE sport_user_legacy_fixture ADD COLUMN username varchar(50) NOT NULL").Error; err != nil {
			t.Fatal(err)
		}
	} else {
		if err := db.Exec("ALTER TABLE sport_user_legacy_fixture MODIFY COLUMN username varchar(50) NOT NULL").Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewUserRepository(db.Table("sport_user_legacy_fixture").Session(&gorm.Session{}))
	failed := &model.User{OpenID: "fixture-before-migration"}
	err = repo.Create(failed)
	mysqlErr, ok := err.(*mysqldriver.MySQLError)
	if !ok || mysqlErr.Number != 1364 {
		t.Fatal("expected missing legacy username error 1364")
	}
	// A legacy value must survive the migration unchanged.
	if err := db.Exec("INSERT INTO sport_user_legacy_fixture (openid, username) VALUES (?, ?)", "fixture-legacy", "legacy-user").Error; err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../migrations/20261012_allow_legacy_username_null.sql")
	if err != nil {
		t.Fatal(err)
	}
	fixtureMigration := strings.Replace(string(migration), "ALTER TABLE users", "ALTER TABLE sport_user_legacy_fixture", 1)
	if err := db.Exec(fixtureMigration).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"fixture-first", "fixture-second"} {
		user := &model.User{OpenID: id}
		if err := repo.Create(user); err != nil {
			t.Fatal(err)
		}
		user.NickName = "fixture"
		if err := repo.Update(user); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Table("sport_user_legacy_fixture").Where("openid IN ? AND username IS NULL AND phone IS NULL", []string{"fixture-first", "fixture-second"}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("new accounts must have NULL username and phone")
	}
	var name string
	if err := db.Table("sport_user_legacy_fixture").Select("username").Where("openid = ?", "fixture-legacy").Scan(&name).Error; err != nil || name != "legacy-user" {
		t.Fatal("legacy username changed")
	}
	duplicate := &model.User{OpenID: "fixture-first"}
	if err := repo.Create(duplicate); err == nil {
		t.Fatal("openid uniqueness must remain enforced")
	}
}
