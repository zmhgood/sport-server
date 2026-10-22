package service

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/pkg/clock"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestDashboardActivationNaturalDaysAndMatureCohortEdges(t *testing.T) {
	f, tx, s := dashboardFixture(t)
	// 2032-03-01 00:00 Shanghai. The last eligible registration date is Feb 23.
	now := time.Date(2032, 2, 29, 16, 0, 0, 0, time.UTC)
	for i, tc := range []struct {
		registered string
		offset     int
	}{
		{"2032-02-17", 0}, {"2032-02-23", 6},
		{"2032-02-23", 7}, {"2032-02-17", -1},
		{"2032-02-16", 0}, {"2032-02-24", 0},
	} {
		// Late registration remains in its Shanghai calendar date, not UTC date.
		reg := engagementDate(tc.registered).Add(23*time.Hour + 59*time.Minute)
		u := model.User{OpenID: fmt.Sprintf("edge-%d-%d", time.Now().UnixNano(), i), CreatedAt: reg, UpdatedAt: reg}
		if err := tx.Omit("Phone").Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		m := model.GoalMember{GoalID: f.goal.ID, UserID: u.ID}
		if err := tx.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
		dashboardProgress(t, tx, m.ID, f.goal.Exercises[0].ID, engagementAdd(tc.registered, tc.offset), f.goal.Exercises[0].Sets)
	}
	result, err := s.dashboardStats(7, now)
	if err != nil {
		t.Fatal(err)
	}
	got := result.Engagement.Activation
	if got.StartDate != "2032-02-17" || got.EndDate != "2032-02-23" || got.EligibleUsers != 4 || got.CompletedUsers != 2 || got.Rate == nil || *got.Rate != .5 {
		t.Fatalf("natural-day activation mismatch: %+v", got)
	}
	data, _ := json.Marshal(result)
	reconcileEngagement(t, tx, 7, data, now)
	// At the instant before Shanghai Monday, the complete weekly period is older.
	previous, err := s.dashboardStats(7, now.Add(-time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	if previous.EndDate != "2032-02-29" || previous.Engagement.WeeklyExerciseDays[0].EndDate != "2032-02-22" {
		t.Fatal("Shanghai midnight week rollover failed")
	}
	if result.EndDate != now.In(clock.Zone).Format("2006-01-02") {
		t.Fatal("timezone mismatch")
	}
}
