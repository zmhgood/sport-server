// Prepare only a disposable local database; never migrate the source database.
package main

import (
	"crypto/rand"
	"database/sql"
	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"encoding/hex"
	"encoding/json"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"regexp"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 3 || (os.Args[1] != "create" && os.Args[1] != "drop") {
		return fmt.Errorf("usage: testdb create LOCAL_CONFIG PRIVATE_OUTPUT.yaml | testdb drop PRIVATE_OUTPUT.yaml")
	}
	if os.Args[1] == "create" && len(os.Args) != 4 {
		return fmt.Errorf("private output path required")
	}
	if err := config.LoadConfig(os.Args[2]); err != nil {
		return fmt.Errorf("cannot load private local configuration")
	}
	cfg := config.AppConfig.Database
	if cfg.Host != "localhost" && cfg.Host != "127.0.0.1" {
		return fmt.Errorf("only local MySQL allowed")
	}
	dsn, err := driver.ParseDSN(cfg.GetDSN())
	if err != nil {
		return fmt.Errorf("invalid local configuration")
	}
	dsn.DBName = ""
	conn, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		return fmt.Errorf("local connection unavailable")
	}
	defer conn.Close()
	if os.Args[1] == "drop" {
		if !regexp.MustCompile(`^sport_qa_[0-9]+$`).MatchString(cfg.DBName) {
			return fmt.Errorf("only generated sport_qa_ databases may be removed")
		}
		if _, err = conn.Exec("DROP DATABASE `" + cfg.DBName + "`"); err != nil {
			return fmt.Errorf("test cleanup failed")
		}
		fmt.Println("Disposable database removed; remove the private configuration file separately")
		return nil
	}
	output, err := os.OpenFile(os.Args[3], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("private output must not already exist")
	}
	defer output.Close()
	cfg.DBName = fmt.Sprintf("sport_qa_%d", time.Now().UnixNano())
	if _, err = conn.Exec("CREATE DATABASE `" + cfg.DBName + "` CHARACTER SET utf8mb4"); err != nil {
		os.Remove(os.Args[3])
		return fmt.Errorf("cannot create disposable database")
	}
	ready := false
	defer func() {
		if !ready {
			conn.Exec("DROP DATABASE `" + cfg.DBName + "`")
			os.Remove(os.Args[3])
		}
	}()
	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("test database unavailable")
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = db.AutoMigrate(&model.User{}, &model.MuscleGroup{}, &model.Exercise{}, &model.ExerciseStep{}, &model.ExerciseBenefit{}, &model.ExercisePrecaution{}, &model.UserExerciseRecord{}, &model.UserDailyStats{}, &model.Family{}, &model.FamilyMember{}, &model.DailyGoal{}, &model.GoalMember{}, &model.GoalExercise{}, &model.GoalCompletion{}, &model.PlanRevision{}, &model.Comment{}, &model.CommentLike{}, &model.Admin{}, &model.SystemConfig{}); err != nil {
		return fmt.Errorf("test schema creation failed")
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return fmt.Errorf("test secret generation failed")
	}
	// JSON is a YAML subset. Copy only local database credentials, never cloud secrets.
	payload := map[string]interface{}{"database": map[string]interface{}{"host": cfg.Host, "port": cfg.Port, "user": cfg.User, "password": cfg.Password, "dbname": cfg.DBName}, "jwt": map[string]interface{}{"secret": hex.EncodeToString(secret), "expire_time": "1h"}, "wechat": map[string]interface{}{"app_id": "synthetic"}}
	if err = json.NewEncoder(output).Encode(payload); err != nil {
		return fmt.Errorf("private configuration write failed")
	}
	ready = true
	fmt.Println("Disposable local database and private test configuration ready")
	return nil
}
