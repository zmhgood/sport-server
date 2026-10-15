package handler

import (
	"bytes"
	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"elderly-fitness/internal/service"
	"elderly-fitness/pkg/clock"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Exercise JSON binding, permissions and persistence together, using synthetic
// users in an explicitly isolated database. Authentication itself has separate tests.
func TestPlanHTTPContractAndFailedTodayLoad(t *testing.T) {
	path := os.Getenv("SPORT_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("isolated local MySQL required")
	}
	if err := config.LoadConfig(path); err != nil {
		t.Fatal("test config unavailable")
	}
	cfg := config.AppConfig.Database
	if (cfg.Host != "localhost" && cfg.Host != "127.0.0.1") || !strings.HasPrefix(cfg.DBName, "sport_qa_") {
		t.Fatal("isolated local MySQL required")
	}
	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("local connection failed")
	}
	conn, _ := db.DB()
	defer conn.Close()
	ur := repository.NewUserRepository(db)
	users := make([]model.User, 3)
	for i := range users {
		users[i] = model.User{OpenID: fmt.Sprintf("qa-plan-http-%d-%d", time.Now().UnixNano(), i)}
		if err := ur.Create(&users[i]); err != nil {
			t.Fatal(err)
		}
	}
	fr := repository.NewFamilyRepository(db)
	fs := service.NewFamilyService(fr)
	family, err := fs.CreateFamily("Synthetic HTTP family", users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.JoinFamily(family.InviteCode, users[1].ID); err != nil {
		t.Fatal(err)
	}
	group := model.MuscleGroup{Name: "Synthetic group"}
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	ex := model.Exercise{Name: "Synthetic action", MuscleGroupID: group.ID}
	if err := db.Create(&ex).Error; err != nil {
		t.Fatal(err)
	}
	h := NewGoalHandler(service.NewGoalService(repository.NewGoalRepository(db), fr, repository.NewExerciseRepository(db)))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	actingUser := users[0].ID
	r.Use(func(c *gin.Context) { c.Set("userID", actingUser) })
	r.POST("/goals", h.CreateGoal)
	r.GET("/goals/families", h.GetUserFamiliesWithGoals)
	r.GET("/goals/:id", h.GetGoal)
	r.PUT("/goals/:id", h.UpdateGoal)
	r.POST("/goals/:id/complete", h.CompleteExercise)
	r.POST("/goals/:id/members", h.AddGoalMember)
	r.DELETE("/goals/:id/members/me", h.LeaveGoal)
	request := func(method, path string, input interface{}, status int) model.DailyGoal {
		t.Helper()
		body, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		r.ServeHTTP(response, req)
		if response.Code != status {
			t.Fatalf("%s %s: got status %d, want %d", method, path, response.Code, status)
		}
		var envelope struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if (status == 200) != (envelope.Code == 0) {
			t.Fatal("HTTP status and application code disagree")
		}
		var goal model.DailyGoal
		if len(envelope.Data) > 0 && envelope.Data[0] == '{' {
			if err := json.Unmarshal(envelope.Data, &goal); err != nil {
				t.Fatal(err)
			}
		}
		return goal
	}
	exercises := []map[string]interface{}{{"exercise_id": ex.ID, "sets": 3, "reps": 10}}
	createURL := fmt.Sprintf("/goals?family_id=%d", family.ID)
	personal := request("POST", createURL, map[string]interface{}{
		"name": "Synthetic personal", "plan_type": "personal", "executor_id": users[1].ID, "exercises": exercises,
	}, 200)
	if personal.PlanType != "personal" || personal.ExecutorID != users[1].ID || !personal.CanManage || personal.IsParticipating || len(personal.Exercises) != 1 {
		t.Fatal("admin-created personal plan has incorrect API fields")
	}
	personalURL := fmt.Sprintf("/goals/%d", personal.ID)
	actingUser = users[1].ID
	view := request("GET", personalURL, nil, 200)
	if !view.IsParticipating || !view.CanManage || view.CurrentDate != clock.Now().Format("2006-01-02") {
		t.Fatal("executor did not immediately receive the plan")
	}
	request("POST", personalURL+"/complete", map[string]interface{}{
		"goal_exercise_id": view.Exercises[0].ID, "completed_sets": 1, "date": "2000-01-01",
	}, 400)
	request("POST", personalURL+"/complete", map[string]interface{}{
		"goal_exercise_id": view.Exercises[0].ID, "completed_sets": 1, "date": view.CurrentDate,
	}, 200)
	actingUser = users[0].ID
	exercises[0]["sets"] = 5
	updated := request("PUT", personalURL, map[string]interface{}{"name": "Changed", "exercises": exercises}, 200)
	if updated.PendingEffectiveDate != clock.Now().AddDate(0, 0, 1).Format("2006-01-02") || len(updated.PendingExercises) != 1 || updated.PendingExercises[0].Sets != 5 || updated.Exercises[0].Sets != 3 {
		t.Fatal("pending revision is not represented correctly over HTTP")
	}
	shared := request("POST", createURL, map[string]interface{}{
		"name": "Synthetic shared", "plan_type": "shared", "member_ids": []uint{}, "exercises": exercises,
	}, 200)
	if shared.IsParticipating || len(shared.Members) != 0 {
		t.Fatal("explicit empty member list incorrectly enrolled the creator")
	}
	sharedURL := fmt.Sprintf("/goals/%d", shared.ID)
	actingUser = users[1].ID
	request("POST", sharedURL+"/members", map[string]interface{}{"user_id": actingUser}, 200)
	joined := request("GET", sharedURL, nil, 200)
	if !joined.IsParticipating || joined.CanManage {
		t.Fatal("join granted plan administration")
	}
	request("DELETE", sharedURL+"/members/me", nil, 200)
	request("DELETE", sharedURL+"/members/me", nil, 200)
	if request("GET", sharedURL, nil, 200).IsParticipating {
		t.Fatal("leave endpoint did not remove participation")
	}
	actingUser = users[2].ID
	request("GET", personalURL, nil, 404)
	actingUser = users[1].ID
	conn.Close()
	request("GET", "/goals/families", nil, 500)
}
