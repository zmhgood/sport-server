package repository

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
	"testing"
)

// Capture actual repository SQL and bound variables without opening a database.
func TestUserInputRemainsBoundSQLParameter(t *testing.T) {
	payloads := []string{"' OR 1=1 --", "x'; DROP TABLE users; --", "' UNION SELECT password FROM admins --", "x\\' OR SLEEP(5)#"}
	queries := []struct {
		name string
		run  func(*gorm.DB, string)
	}{
		{"admin login", func(db *gorm.DB, p string) { NewAdminRepository(db).FindByUsername(p) }},
		{"openid", func(db *gorm.DB, p string) { NewUserRepository(db).FindByOpenID(p) }},
		{"phone", func(db *gorm.DB, p string) { NewUserRepository(db).FindByPhone(p) }},
		{"invitation", func(db *gorm.DB, p string) { NewFamilyRepository(db).FindByInviteCode(p) }},
		{"family search", func(db *gorm.DB, p string) { NewFamilyRepository(db).ListAll(1, 10, p) }},
		{"user search", func(db *gorm.DB, p string) { NewUserRepository(db).ListUsers(1, 10, p, nil) }},
		{"admin user search", func(db *gorm.DB, p string) { NewAdminRepository(db).ListUsers(1, 10, p, nil) }},
		{"comment search", func(db *gorm.DB, p string) { NewAdminRepository(db).ListAllComments(1, 10, nil, p) }},
		{"exercise search", func(db *gorm.DB, p string) { NewExerciseRepository(db).GetExercisesList(1, 10, 0, "", p) }},
		{"exercise filter", func(db *gorm.DB, p string) { NewExerciseRepository(db).GetExercisesList(1, 10, 0, p, "") }},
		{"config key", func(db *gorm.DB, p string) { NewSystemRepository(db).GetConfig(p) }},
	}
	for _, q := range queries {
		for _, payload := range payloads {
			t.Run(q.name+"/"+payload, func(t *testing.T) {
				db, err := gorm.Open(mysql.New(mysql.Config{DSN: "dryrun:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
				if err != nil {
					t.Fatal("dry-run setup failed")
				}
				count, bound := 0, false
				db.Callback().Query().After("gorm:query").Register("qa:bound", func(tx *gorm.DB) {
					sql := tx.Statement.SQL.String()
					if sql == "" {
						return
					}
					count++
					if strings.Contains(sql, payload) {
						t.Fatal("user payload interpolated into SQL")
					}
					for _, v := range tx.Statement.Vars {
						if s, ok := v.(string); ok && strings.Contains(s, payload) {
							bound = true
						}
					}
				})
				q.run(db, payload)
				if count == 0 || !bound {
					t.Fatal("repository path did not bind the tested input")
				}
			})
		}
	}
}
