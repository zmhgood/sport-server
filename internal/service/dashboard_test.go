package service

import (
	"errors"
	"testing"
	"time"
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
