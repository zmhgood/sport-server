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
	Engagement       DashboardEngagement               `json:"engagement"`
}

type DashboardEngagement struct {
	Activation         DashboardActivation           `json:"activation"`
	WeeklyExerciseDays []DashboardWeeklyExerciseDays `json:"weeklyExerciseDays"`
	WeeklyRetention    []DashboardWeeklyRetention    `json:"weeklyRetention"`
	Families           DashboardFamilies             `json:"families"`
}

type DashboardActivation struct {
	StartDate      string   `json:"startDate"`
	EndDate        string   `json:"endDate"`
	EligibleUsers  int64    `json:"eligibleUsers"`
	CompletedUsers int64    `json:"completedUsers"`
	Rate           *float64 `json:"rate"`
}

type DashboardWeeklyExerciseDays struct {
	StartDate       string `json:"startDate"`
	EndDate         string `json:"endDate"`
	ActiveUsers     int64  `json:"activeUsers"`
	OneToTwoDays    int64  `json:"oneToTwoDays"`
	ThreeToFourDays int64  `json:"threeToFourDays"`
	FiveToSevenDays int64  `json:"fiveToSevenDays"`
}

type DashboardWeeklyRetention struct {
	StartDate       string   `json:"startDate"`
	EndDate         string   `json:"endDate"`
	ReturnStartDate string   `json:"returnStartDate"`
	ReturnEndDate   string   `json:"returnEndDate"`
	CohortUsers     int64    `json:"cohortUsers"`
	ReturnedUsers   int64    `json:"returnedUsers"`
	Rate            *float64 `json:"rate"`
}

type DashboardFamilies struct {
	ActiveCount int64                  `json:"activeCount"`
	Daily       []DashboardFamilyDaily `json:"daily"`
}

type DashboardFamilyDaily struct {
	Date        string `json:"date"`
	FamilyCount int64  `json:"familyCount"`
}

type dashboardWeek struct {
	start time.Time
	end   time.Time
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
	weeks := dashboardCompletedWeeks(days, end)
	activityStart := start
	if len(weeks) > 0 && weeks[0].start.AddDate(0, 0, -7).Before(activityStart) {
		activityStart = weeks[0].start.AddDate(0, 0, -7)
	}
	registrationStart := start.AddDate(0, 0, -7)
	registrationEnd := end.AddDate(0, 0, -7)
	engagementRange := repository.DashboardEngagementRange{
		ActivityStartDate:     activityStart.Format("2006-01-02"),
		ActivityEndDate:       end.Format("2006-01-02"),
		RegistrationStartDate: registrationStart.Format("2006-01-02"),
		RegistrationAfterDate: registrationEnd.AddDate(0, 0, 1).Format("2006-01-02"),
		ActivationProgressEnd: registrationEnd.AddDate(0, 0, 6).Format("2006-01-02"),
		FamilyStartDate:       start.Format("2006-01-02"),
		FamilyEndDate:         end.Format("2006-01-02"),
	}
	metrics, err := s.userRepo.GetDashboardMetrics(start.Format("2006-01-02"), end.Format("2006-01-02"), engagementRange)
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
	result.Engagement = buildDashboardEngagement(metrics.Engagement, weeks, start, end, registrationStart, registrationEnd)
	return result, nil
}

func dashboardCompletedWeeks(days int, end time.Time) []dashboardWeek {
	count := 4
	if days == 7 {
		count = 1
	}
	currentMonday := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	weekdayOffset := (int(currentMonday.Weekday()) + 6) % 7
	currentMonday = currentMonday.AddDate(0, 0, -weekdayOffset)
	latestStart := currentMonday.AddDate(0, 0, -7)
	weeks := make([]dashboardWeek, count)
	for i := range weeks {
		weekStart := latestStart.AddDate(0, 0, -7*(count-1-i))
		weeks[i] = dashboardWeek{start: weekStart, end: weekStart.AddDate(0, 0, 6)}
	}
	return weeks
}

func buildDashboardEngagement(metrics repository.DashboardEngagementMetrics, weeks []dashboardWeek, start, end, registrationStart, registrationEnd time.Time) DashboardEngagement {
	activeDates := make(map[string]map[uint]struct{}, len(metrics.ActiveUserDays))
	for _, row := range metrics.ActiveUserDays {
		if activeDates[row.Date] == nil {
			activeDates[row.Date] = make(map[uint]struct{})
		}
		activeDates[row.Date][row.UserID] = struct{}{}
	}
	weekly := make([]DashboardWeeklyExerciseDays, 0, len(weeks))
	retention := make([]DashboardWeeklyRetention, 0, len(weeks))
	for _, week := range weeks {
		currentUsers := dashboardActiveUsers(activeDates, week)
		item := DashboardWeeklyExerciseDays{StartDate: week.start.Format("2006-01-02"), EndDate: week.end.Format("2006-01-02")}
		for userID := range currentUsers {
			days := dashboardUserDayCount(activeDates, userID, week)
			switch {
			case days <= 2:
				item.OneToTwoDays++
			case days <= 4:
				item.ThreeToFourDays++
			default:
				item.FiveToSevenDays++
			}
		}
		item.ActiveUsers = item.OneToTwoDays + item.ThreeToFourDays + item.FiveToSevenDays
		weekly = append(weekly, item)
		baseline := dashboardActiveUsers(activeDates, dashboardWeek{start: week.start.AddDate(0, 0, -7), end: week.start.AddDate(0, 0, -1)})
		returned := int64(0)
		for userID := range baseline {
			if _, ok := currentUsers[userID]; ok {
				returned++
			}
		}
		retention = append(retention, DashboardWeeklyRetention{
			StartDate: week.start.AddDate(0, 0, -7).Format("2006-01-02"), EndDate: week.start.AddDate(0, 0, -1).Format("2006-01-02"),
			ReturnStartDate: week.start.Format("2006-01-02"), ReturnEndDate: week.end.Format("2006-01-02"),
			CohortUsers: int64(len(baseline)), ReturnedUsers: returned, Rate: dashboardRate(returned, int64(len(baseline))),
		})
	}
	dailyFamilies := make(map[string]int64, len(metrics.FamilyDaily))
	for _, row := range metrics.FamilyDaily {
		dailyFamilies[row.Date] = row.FamilyCount
	}
	families := DashboardFamilies{ActiveCount: metrics.FamilyActiveCount, Daily: make([]DashboardFamilyDaily, 0)}
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		key := date.Format("2006-01-02")
		families.Daily = append(families.Daily, DashboardFamilyDaily{Date: key, FamilyCount: dailyFamilies[key]})
	}
	return DashboardEngagement{
		Activation:         DashboardActivation{StartDate: registrationStart.Format("2006-01-02"), EndDate: registrationEnd.Format("2006-01-02"), EligibleUsers: metrics.ActivationEligibleUsers, CompletedUsers: metrics.ActivationCompletedUsers, Rate: dashboardRate(metrics.ActivationCompletedUsers, metrics.ActivationEligibleUsers)},
		WeeklyExerciseDays: weekly, WeeklyRetention: retention, Families: families,
	}
}

func dashboardActiveUsers(activeDates map[string]map[uint]struct{}, week dashboardWeek) map[uint]struct{} {
	users := make(map[uint]struct{})
	for date := week.start; !date.After(week.end); date = date.AddDate(0, 0, 1) {
		for userID := range activeDates[date.Format("2006-01-02")] {
			users[userID] = struct{}{}
		}
	}
	return users
}

func dashboardUserDayCount(activeDates map[string]map[uint]struct{}, userID uint, week dashboardWeek) int {
	count := 0
	for date := week.start; !date.After(week.end); date = date.AddDate(0, 0, 1) {
		if _, ok := activeDates[date.Format("2006-01-02")][userID]; ok {
			count++
		}
	}
	return count
}

func dashboardRate(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	rate := float64(numerator) / float64(denominator)
	return &rate
}
