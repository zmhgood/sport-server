package service

import (
	"errors"
	"testing"
	"time"

	"elderly-fitness/internal/repository"
)

func TestDashboardRangeUsesShanghaiAndIncludesToday(t *testing.T) {
	now := time.Date(2032, 2, 29, 16, 0, 0, 0, time.UTC) // March 1 in Shanghai, leap year.
	for _, tc := range []struct {
		days  int
		start string
	}{{7, "2032-02-24"}, {30, "2032-02-01"}} {
		start, end, err := dashboardRange(tc.days, now)
		if err != nil || start.Format("2006-01-02") != tc.start || end.Format("2006-01-02") != "2032-03-01" || start.Location().String() != "Asia/Shanghai" {
			t.Fatalf("invalid business window: %v %v %v", start, end, err)
		}
	}
}
func TestDashboardRejectsUnsupportedRangeBeforeDatabaseAccess(t *testing.T) {
	s := NewUserService(nil, nil)
	for _, days := range []int{-1, 0, 1, 6, 8, 29, 31, 365, 1000000} {
		if _, err := s.GetDashboardStats(days); !errors.Is(err, ErrDashboardRange) {
			t.Fatalf("accepted range %d", days)
		}
	}
}

func TestDashboardCompletedWeeksAreMondayToSundayAndExcludeCurrentWeek(t *testing.T) {
	loc := clockZoneForTest(t)
	for _, tc := range []struct {
		days       int
		end        string
		firstStart string
		lastEnd    string
		count      int
	}{
		{7, "2032-03-01", "2032-02-23", "2032-02-29", 1}, // Monday; previous complete week.
		{30, "2032-03-01", "2032-02-02", "2032-02-29", 4},
		{7, "2032-03-07", "2032-02-23", "2032-02-29", 1}, // Sunday remains incomplete until the next week.
	} {
		end, err := time.ParseInLocation("2006-01-02", tc.end, loc)
		if err != nil {
			t.Fatal(err)
		}
		weeks := dashboardCompletedWeeks(tc.days, end)
		if len(weeks) != tc.count || weeks[0].start.Format("2006-01-02") != tc.firstStart || weeks[len(weeks)-1].end.Format("2006-01-02") != tc.lastEnd {
			t.Fatalf("days=%d end=%s got=%+v", tc.days, tc.end, weeks)
		}
		for _, week := range weeks {
			if week.start.Weekday() != time.Monday || week.end.Weekday() != time.Sunday || week.end.Sub(week.start) != 6*24*time.Hour {
				t.Fatalf("week is not Monday-Sunday: %+v", week)
			}
		}
	}
}

func TestBuildDashboardEngagementCountsDistinctUserDaysAndReturnsNullRates(t *testing.T) {
	loc := clockZoneForTest(t)
	parse := func(value string) time.Time {
		parsed, err := time.ParseInLocation("2006-01-02", value, loc)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	week := dashboardWeek{start: parse("2032-02-23"), end: parse("2032-02-29")}
	metrics := repository.DashboardEngagementMetrics{
		ActiveUserDays: []repository.DashboardUserDay{
			{UserID: 1, Date: "2032-02-23"}, {UserID: 1, Date: "2032-02-23"},
			{UserID: 1, Date: "2032-02-24"}, {UserID: 1, Date: "2032-02-25"},
			{UserID: 2, Date: "2032-02-23"}, {UserID: 2, Date: "2032-03-01"},
		},
		FamilyActiveCount: 2,
		FamilyDaily:       []repository.DashboardFamilyDaily{{Date: "2032-03-01", FamilyCount: 2}},
	}
	result := buildDashboardEngagement(metrics, []dashboardWeek{week}, parse("2032-03-01"), parse("2032-03-02"), parse("2032-02-23"), parse("2032-02-25"))
	if got := result.WeeklyExerciseDays[0]; got.ActiveUsers != 2 || got.OneToTwoDays != 1 || got.ThreeToFourDays != 1 || got.FiveToSevenDays != 0 {
		t.Fatalf("unexpected exercise-day distribution: %+v", got)
	}
	if got := result.WeeklyRetention[0]; got.CohortUsers != 0 || got.ReturnedUsers != 0 || got.Rate != nil || got.StartDate != "2032-02-16" || got.ReturnStartDate != "2032-02-23" {
		t.Fatalf("unexpected zero-cohort retention: %+v", got)
	}
	if result.Activation.Rate != nil || result.Families.Daily[0].FamilyCount != 2 || result.Families.Daily[1].FamilyCount != 0 {
		t.Fatalf("unexpected zero-rate or zero-fill result: %+v", result)
	}
}

func clockZoneForTest(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
