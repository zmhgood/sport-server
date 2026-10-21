package service

import (
	"elderly-fitness/internal/repository"
	"elderly-fitness/pkg/clock"
	"errors"
	"time"
)

var ErrDashboardRange = errors.New("统计范围仅支持最近7天或30天")

type DashboardStats struct {
	UserCount        int64                             `json:"userCount"`
	ExerciseCount    int64                             `json:"exerciseCount"`
	CommentCount     int64                             `json:"commentCount"`
	TodayRecordCount int64                             `json:"todayRecordCount"`
	Days             int                               `json:"days"`
	StartDate        string                            `json:"startDate"`
	EndDate          string                            `json:"endDate"`
	TimeZone         string                            `json:"timeZone"`
	DailyActivity    []repository.DashboardActivity    `json:"dailyActivity"`
	HotExercises     []repository.DashboardHotExercise `json:"hotExercises"`
}

func dashboardRange(days int, now time.Time) (time.Time, time.Time, error) {
	if days != 7 && days != 30 {
		return time.Time{}, time.Time{}, ErrDashboardRange
	}
	local := now.In(clock.Zone)
	end := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, clock.Zone)
	return end.AddDate(0, 0, 1-days), end, nil
}

func (s *UserService) GetDashboardStats(days int) (*DashboardStats, error) {
	return s.dashboardStats(days, clock.Now())
}

func (s *UserService) dashboardStats(days int, now time.Time) (*DashboardStats, error) {
	start, end, err := dashboardRange(days, now)
	if err != nil {
		return nil, err
	}
	metrics, err := s.userRepo.GetDashboardMetrics(start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	result := &DashboardStats{UserCount: metrics.UserCount, ExerciseCount: metrics.ExerciseCount, CommentCount: metrics.CommentCount,
		Days: days, StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02"), TimeZone: "Asia/Shanghai",
		DailyActivity: make([]repository.DashboardActivity, 0, days), HotExercises: metrics.HotExercises}
	byDate := make(map[string]repository.DashboardActivity, len(metrics.Activity))
	for _, day := range metrics.Activity {
		byDate[day.Date] = day
	}
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		day := byDate[date]
		day.Date = date
		result.DailyActivity = append(result.DailyActivity, day)
		if date == result.EndDate {
			result.TodayRecordCount = day.CompletedCount
		}
	}
	return result, nil
}
