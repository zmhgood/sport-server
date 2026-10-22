package service

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/pkg/clock"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"testing"
	"time"
)

// These wire types intentionally describe the public JSON contract instead of
// depending on the production engagement Go structs.
type engagementWire struct {
	Activation struct {
		StartDate      string   `json:"startDate"`
		EndDate        string   `json:"endDate"`
		EligibleUsers  int64    `json:"eligibleUsers"`
		CompletedUsers int64    `json:"completedUsers"`
		Rate           *float64 `json:"rate"`
	} `json:"activation"`
	WeeklyExerciseDays []struct {
		StartDate    string `json:"startDate"`
		EndDate      string `json:"endDate"`
		ActiveUsers  int64  `json:"activeUsers"`
		OneToTwoDays int64  `json:"oneToTwoDays"`
		ThreeToFour  int64  `json:"threeToFourDays"`
		FiveToSeven  int64  `json:"fiveToSevenDays"`
	} `json:"weeklyExerciseDays"`
	WeeklyRetention []struct {
		StartDate     string   `json:"startDate"`
		EndDate       string   `json:"endDate"`
		ReturnStart   string   `json:"returnStartDate"`
		ReturnEnd     string   `json:"returnEndDate"`
		CohortUsers   int64    `json:"cohortUsers"`
		ReturnedUsers int64    `json:"returnedUsers"`
		Rate          *float64 `json:"rate"`
	} `json:"weeklyRetention"`
	Families struct {
		ActiveCount int64 `json:"activeCount"`
		Daily       []struct {
			Date        string `json:"date"`
			FamilyCount int64  `json:"familyCount"`
		} `json:"daily"`
	} `json:"families"`
}

type engagementEnvelopeWire struct {
	Engagement engagementWire `json:"engagement"`
}

type engagementDaily struct {
	Date        string `json:"date"`
	FamilyCount int64  `json:"familyCount"`
}
type engagementWeek struct {
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	ActiveUsers  int64  `json:"activeUsers"`
	OneToTwoDays int64  `json:"oneToTwoDays"`
	ThreeToFour  int64  `json:"threeToFourDays"`
	FiveToSeven  int64  `json:"fiveToSevenDays"`
}
type engagementRetention struct {
	StartDate     string   `json:"startDate"`
	EndDate       string   `json:"endDate"`
	ReturnStart   string   `json:"returnStartDate"`
	ReturnEnd     string   `json:"returnEndDate"`
	CohortUsers   int64    `json:"cohortUsers"`
	ReturnedUsers int64    `json:"returnedUsers"`
	Rate          *float64 `json:"rate"`
}
type engagementExpected struct {
	Activation struct {
		StartDate      string   `json:"startDate"`
		EndDate        string   `json:"endDate"`
		EligibleUsers  int64    `json:"eligibleUsers"`
		CompletedUsers int64    `json:"completedUsers"`
		Rate           *float64 `json:"rate"`
	} `json:"activation"`
	WeeklyExerciseDays []engagementWeek      `json:"weeklyExerciseDays"`
	WeeklyRetention    []engagementRetention `json:"weeklyRetention"`
	Families           struct {
		ActiveCount int64             `json:"activeCount"`
		Daily       []engagementDaily `json:"daily"`
	} `json:"families"`
}

type engagementEnvelopeExpected struct {
	Engagement engagementExpected `json:"engagement"`
}

type engagementUserRow struct {
	ID   uint
	Date string
}
type engagementRawRow struct {
	ID         uint
	UserID     uint
	GoalID     uint
	ExerciseID uint
	Date       string
	FamilyID   uint
	FamilyOK   int
	Sets       int
	Target     int
}

func engagementDate(value string) time.Time {
	d, err := time.ParseInLocation("2006-01-02", value, clock.Zone)
	if err != nil {
		panic(err)
	}
	return d
}

func engagementAdd(date string, days int) string {
	return engagementDate(date).AddDate(0, 0, days).Format("2006-01-02")
}

func engagementRatio(n, d int64) *float64 {
	if d == 0 {
		return nil
	}
	r := float64(n) / float64(d)
	return &r
}

func engagementWeekEnds(end string, count int) [][2]string {
	// The frozen test clock is Monday 2032-03-01. Walk back to the most recent
	// Sunday before that date, then retain only whole Monday-Sunday weeks.
	sunday := engagementDate(end)
	for sunday.Weekday() != time.Sunday || sunday.Format("2006-01-02") == end {
		sunday = sunday.AddDate(0, 0, -1)
	}
	weeks := make([][2]string, count)
	for i := count - 1; i >= 0; i-- {
		weekEnd := sunday.AddDate(0, 0, -7*i)
		weekStart := weekEnd.AddDate(0, 0, -6)
		weeks[count-1-i] = [2]string{weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02")}
	}
	return weeks
}

// reconcileEngagement reads source rows directly and folds canonical records
// in Go. It deliberately does not call dashboard query or aggregation helpers.
func reconcileEngagement(t *testing.T, db *gorm.DB, days int, resultJSON []byte, now time.Time) {
	t.Helper()
	var envelope engagementEnvelopeWire
	if err := json.Unmarshal(resultJSON, &envelope); err != nil {
		t.Fatal(err)
	}
	wire := envelope.Engagement
	var base struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	if err := json.Unmarshal(resultJSON, &base); err != nil {
		t.Fatal(err)
	}
	start, end := base.StartDate, base.EndDate
	if start == "" || end == "" {
		t.Fatalf("dashboard response is missing its window: %+v", base)
	}
	activationStart, activationEnd := engagementAdd(start, -7), engagementAdd(end, -7)
	weekCount := 1
	if days == 30 {
		weekCount = 4
	}
	weeks := engagementWeekEnds(end, weekCount)
	from := activationStart
	if firstBaseline := engagementAdd(weeks[0][0], -7); firstBaseline < from {
		from = firstBaseline
	}
	var users []engagementUserRow
	if err := db.Raw("SELECT id, DATE_FORMAT(created_at,'%Y-%m-%d') AS date FROM users").Scan(&users).Error; err != nil {
		t.Fatal(err)
	}
	userDates := map[uint]string{}
	for _, u := range users {
		userDates[u.ID] = u.Date
	}
	var raw []engagementRawRow
	query := `SELECT c.id,m.user_id,m.goal_id,ge.exercise_id,DATE_FORMAT(c.date,'%Y-%m-%d') AS date,
 g.family_id,EXISTS(SELECT 1 FROM families f WHERE f.id=g.family_id) AS family_ok,
 c.completed_sets AS sets,ge.sets AS target
 FROM goal_completions c JOIN goal_members m ON m.id=c.goal_member_id
 JOIN daily_goals g ON g.id=m.goal_id JOIN goal_exercises ge ON ge.id=c.goal_exercise_id
 JOIN exercises e ON e.id=ge.exercise_id
 WHERE c.deleted_at IS NULL AND ge.goal_id=m.goal_id AND c.date>=? AND c.date<=?`
	if err := db.Raw(query, from, end).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	latest := map[string]engagementRawRow{}
	for _, row := range raw {
		key := fmt.Sprintf("%d/%d/%d/%s", row.UserID, row.GoalID, row.ExerciseID, row.Date)
		if old, ok := latest[key]; !ok || row.ID > old.ID {
			latest[key] = row
		}
	}
	valid := make([]engagementRawRow, 0, len(latest))
	for _, row := range latest {
		if row.Target > 0 && row.Sets > 0 && row.Sets <= row.Target {
			valid = append(valid, row)
		}
	}

	var expected engagementExpected
	expected.Activation.StartDate, expected.Activation.EndDate = activationStart, activationEnd
	for _, u := range users {
		if u.Date >= activationStart && u.Date <= activationEnd && engagementAdd(u.Date, 6) <= now.In(clock.Zone).Format("2006-01-02") {
			expected.Activation.EligibleUsers++
		}
	}
	activated := map[uint]bool{}
	for _, row := range valid {
		if row.Sets == row.Target {
			registered := userDates[row.UserID]
			if registered >= activationStart && registered <= activationEnd && row.Date >= registered && row.Date <= engagementAdd(registered, 6) && engagementAdd(registered, 6) <= now.In(clock.Zone).Format("2006-01-02") {
				activated[row.UserID] = true
			}
		}
	}
	expected.Activation.CompletedUsers = int64(len(activated))
	expected.Activation.Rate = engagementRatio(expected.Activation.CompletedUsers, expected.Activation.EligibleUsers)

	activityByWeek := make([]map[uint]map[string]bool, len(weeks))
	for i := range activityByWeek {
		activityByWeek[i] = map[uint]map[string]bool{}
	}
	dailyFamilies := map[string]map[uint]bool{}
	activeFamilies := map[uint]bool{}
	for _, row := range valid {
		if row.Date >= start && row.Date <= end && row.FamilyOK != 0 && row.FamilyID != 0 {
			if dailyFamilies[row.Date] == nil {
				dailyFamilies[row.Date] = map[uint]bool{}
			}
			dailyFamilies[row.Date][row.FamilyID] = true
			activeFamilies[row.FamilyID] = true
		}
		for i, week := range weeks {
			if row.Date >= week[0] && row.Date <= week[1] {
				if activityByWeek[i][row.UserID] == nil {
					activityByWeek[i][row.UserID] = map[string]bool{}
				}
				activityByWeek[i][row.UserID][row.Date] = true
			}
		}
	}
	expected.Families.ActiveCount = int64(len(activeFamilies))
	expected.Families.Daily = make([]engagementDaily, 0, days)
	for i := 0; i < days; i++ {
		date := engagementAdd(start, i)
		expected.Families.Daily = append(expected.Families.Daily, engagementDaily{Date: date, FamilyCount: int64(len(dailyFamilies[date]))})
	}

	for i, week := range weeks {
		var row engagementWeek
		row.StartDate, row.EndDate = week[0], week[1]
		for _, distinctDays := range activityByWeek[i] {
			switch n := len(distinctDays); {
			case n >= 1 && n <= 2:
				row.OneToTwoDays++
			case n <= 4:
				row.ThreeToFour++
			case n <= 7:
				row.FiveToSeven++
			}
		}
		row.ActiveUsers = row.OneToTwoDays + row.ThreeToFour + row.FiveToSeven
		expected.WeeklyExerciseDays = append(expected.WeeklyExerciseDays, row)
		baselineUsers := map[uint]bool{}
		baselineStart, baselineEnd := engagementAdd(week[0], -7), engagementAdd(week[1], -7)
		for _, source := range valid {
			if source.Date >= baselineStart && source.Date <= baselineEnd {
				baselineUsers[source.UserID] = true
			}
		}
		returned := int64(0)
		for user := range baselineUsers {
			if len(activityByWeek[i][user]) > 0 {
				returned++
			}
		}
		cohort := int64(len(baselineUsers))
		expected.WeeklyRetention = append(expected.WeeklyRetention, engagementRetention{
			StartDate: baselineStart, EndDate: baselineEnd, ReturnStart: week[0], ReturnEnd: week[1],
			CohortUsers: cohort, ReturnedUsers: returned, Rate: engagementRatio(returned, cohort),
		})
	}
	var actual engagementEnvelopeExpected
	if err := json.Unmarshal(resultJSON, &actual); err != nil {
		t.Fatal(err)
	}
	var expectedEnvelope engagementEnvelopeExpected
	expectedEnvelope.Engagement = expected
	expectedJSON, _ := json.Marshal(expectedEnvelope)
	actualJSON, _ := json.Marshal(actual)
	if string(expectedJSON) != string(actualJSON) {
		t.Fatalf("engagement raw-row reconciliation failed\nexpected: %s\nactual:   %s", expectedJSON, actualJSON)
	}
	if len(wire.WeeklyExerciseDays) != weekCount || len(wire.WeeklyRetention) != weekCount {
		t.Fatalf("wrong completed-week count: exercise=%d retention=%d want=%d", len(wire.WeeklyExerciseDays), len(wire.WeeklyRetention), weekCount)
	}
}

func TestDashboardEngagementRawReconciliationSevenAndThirtyDays(t *testing.T) {
	f, tx, s := dashboardFixture(t)
	// Keep fixture-created users outside all synthetic registration cohorts.
	if err := tx.Model(&model.User{}).Where("openid LIKE ?", "qa-%").Update("created_at", "1900-01-01 00:00:00").Error; err != nil {
		t.Fatal(err)
	}
	newUser := func(name, registered string, softDeleted bool) model.User {
		t.Helper()
		created := engagementDate(registered)
		u := model.User{OpenID: fmt.Sprintf("engage-%s-%d", name, time.Now().UnixNano()), Phone: fmt.Sprintf("9%019d", time.Now().UnixNano()), NickName: name, CreatedAt: created, UpdatedAt: created}
		if err := tx.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		if softDeleted {
			if err := tx.Delete(&u).Error; err != nil {
				t.Fatal(err)
			}
		}
		return u
	}
	// Both 7-day and 30-day activation cohorts have first-day, last-day,
	// incomplete, empty, outside-boundary and soft-deleted-account cases.
	u7First := newUser("7-first", "2032-02-17", false)
	u7Last := newUser("7-last", "2032-02-22", true)
	u7Partial := newUser("7-partial", "2032-02-17", false)
	newUser("7-empty", "2032-02-22", false)
	u7Before := newUser("7-before", "2032-02-16", false)
	u7After := newUser("7-after", "2032-02-24", false)
	u30First := newUser("30-first", "2032-01-25", false)
	u30Last := newUser("30-last", "2032-01-31", false)
	u30Partial := newUser("30-partial", "2032-01-25", false)
	newUser("30-empty", "2032-01-31", false)
	u30Before := newUser("30-before", "2032-01-24", false)
	u30After := newUser("30-after", "2032-02-01", false)
	uFuture := newUser("future", "2032-03-02", false)

	member := func(goalID, userID uint) model.GoalMember {
		m := model.GoalMember{GoalID: goalID, UserID: userID}
		if err := tx.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
		return m
	}
	progress := func(m model.GoalMember, ge model.GoalExercise, date string, sets int) model.GoalCompletion {
		t.Helper()
		row := model.GoalCompletion{GoalMemberID: m.ID, GoalExerciseID: ge.ID, Date: engagementDate(date), CompletedSets: sets, Status: "done"}
		if err := tx.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		return row
	}
	ge := f.goal.Exercises[0]
	var firstCanonical model.GoalCompletion
	for _, tc := range []struct {
		u    model.User
		date string
	}{{u7First, "2032-02-17"}, {u7Last, "2032-02-28"}, {u30First, "2032-01-25"}, {u30Last, "2032-02-06"}} {
		row := progress(member(f.goal.ID, tc.u.ID), ge, tc.date, ge.Sets)
		if tc.u.ID == u7First.ID {
			firstCanonical = row
		}
	}
	progress(member(f.goal.ID, u7Partial.ID), ge, "2032-02-17", 1)
	progress(member(f.goal.ID, u30Partial.ID), ge, "2032-01-25", 1)
	// Before-registration and after-cohort activity do not activate either cohort.
	progress(member(f.goal.ID, u7Before.ID), ge, "2032-02-17", ge.Sets)
	progress(member(f.goal.ID, u7After.ID), ge, "2032-02-24", ge.Sets)
	progress(member(f.goal.ID, u30Before.ID), ge, "2032-01-25", ge.Sets)
	progress(member(f.goal.ID, u30After.ID), ge, "2032-02-01", ge.Sets)
	progress(member(f.goal.ID, uFuture.ID), ge, "2032-03-02", ge.Sets)
	if err := tx.Create(&model.UserExerciseRecord{CompletionID: &firstCanonical.ID, UserID: u7First.ID, ExerciseID: ge.ExerciseID, Sets: ge.Sets, CompletedAt: engagementDate("2032-02-17")}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.UserExerciseRecord{UserID: f.users[2].ID, ExerciseID: ge.ExerciseID, Sets: ge.Sets, CompletedAt: dashboardTestNow}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.UserDailyStats{UserID: u7First.ID, Date: engagementDate("2032-02-16"), ExerciseCount: 999}).Error; err != nil {
		t.Fatal(err)
	}

	// Populate completed Monday-Sunday weeks with users spanning every bin.
	weekDates := []string{"2032-02-02", "2032-02-09", "2032-02-16", "2032-02-23"}
	activityCounts := []int{1, 2, 3, 5, 7}
	for _, n := range activityCounts {
		u := newUser(fmt.Sprintf("weekly-%d", n), "2032-01-01", false)
		m := member(f.goal.ID, u.ID)
		progress(m, ge, "2032-01-30", 1) // first retention baseline
		for wi, weekStart := range weekDates {
			activeDays := 1
			if wi == len(weekDates)-1 {
				activeDays = n
			}
			for j := 0; j < activeDays; j++ {
				progress(m, ge, engagementAdd(weekStart, j), 1)
			}
		}
	}
	// Same user is one active user but contributes two distinct families.
	secondFamily := model.Family{Name: "Synthetic second family", CreatorID: f.users[0].ID}
	if err := tx.Create(&secondFamily).Error; err != nil {
		t.Fatal(err)
	}
	secondGoal := model.DailyGoal{FamilyID: secondFamily.ID, Name: "Synthetic second-family goal", CreatedBy: f.users[0].ID, PlanType: "personal", ExecutorID: f.users[0].ID}
	if err := tx.Create(&secondGoal).Error; err != nil {
		t.Fatal(err)
	}
	secondMember := member(secondGoal.ID, f.users[0].ID)
	secondGE := model.GoalExercise{GoalID: secondGoal.ID, ExerciseID: ge.ExerciseID, Sets: ge.Sets, Reps: ge.Reps, RevisionID: 4}
	if err := tx.Create(&secondGE).Error; err != nil {
		t.Fatal(err)
	}
	progress(secondMember, secondGE, "2032-03-01", secondGE.Sets)
	// Duplicate revisions are keyed to the catalog action; the newest partial
	// revision suppresses an older full record. A newer soft-deleted row does not.
	versioned := model.GoalExercise{GoalID: f.goal.ID, ExerciseID: ge.ExerciseID, Sets: 4, Reps: ge.Reps, RevisionID: 9}
	if err := tx.Create(&versioned).Error; err != nil {
		t.Fatal(err)
	}
	versionMember := member(f.goal.ID, f.users[2].ID)
	progress(versionMember, ge, "2032-02-28", ge.Sets)
	latestPartial := progress(versionMember, versioned, "2032-02-28", 2)
	deleted := progress(versionMember, versioned, "2032-02-28", 0)
	if err := tx.Delete(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	_ = latestPartial
	// A later non-deleted invalid row withdraws the logical action entirely.
	progress(versionMember, ge, "2032-02-27", ge.Sets)
	progress(versionMember, versioned, "2032-02-27", 0)
	progress(versionMember, ge, "2032-02-25", ge.Sets)
	progress(versionMember, versioned, "2032-02-25", -1)
	progress(versionMember, ge, "2032-02-24", ge.Sets)
	progress(versionMember, versioned, "2032-02-24", versioned.Sets+1)
	// Soft-deleted membership, plan, exercise version and family keep historical
	// source rows available to the raw oracle and the dashboard.
	archivedMember := dashboardMember(t, tx, f.goal.ID, f.users[0].ID)
	progress(archivedMember, ge, "2032-02-26", 1)
	if err := tx.Delete(&archivedMember).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Delete(&versioned).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Delete(&secondGoal).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Delete(&secondFamily).Error; err != nil {
		t.Fatal(err)
	}
	// Transactions prevent global fixture residue across date cohorts and ranges.
	for _, days := range []int{7, 30} {
		result, err := s.dashboardStats(days, dashboardTestNow)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}

		reconcileEngagement(t, tx, days, encoded, dashboardTestNow)
		var envelope engagementEnvelopeWire
		if err := json.Unmarshal(encoded, &envelope); err != nil {
			t.Fatal(err)
		}
		report := envelope.Engagement
		if days == 7 && (report.Activation.EligibleUsers != 4 || report.Activation.CompletedUsers != 2) {
			t.Fatalf("7-day activation boundary/denominator mismatch: %+v", report.Activation)
		}
		if days == 30 && (report.Activation.EligibleUsers != 10 || report.Activation.CompletedUsers != 6) {
			t.Fatalf("30-day activation boundary/denominator mismatch: %+v", report.Activation)
		}
		if days == 7 {
			week := report.WeeklyExerciseDays[0]
			retention := report.WeeklyRetention[0]
			if week.ActiveUsers != 9 || week.OneToTwoDays != 6 || week.ThreeToFour != 1 || week.FiveToSeven != 2 {
				t.Fatalf("7-day weekly bins mismatch: %+v", week)
			}
			if retention.CohortUsers != 8 || retention.ReturnedUsers != 5 {
				t.Fatalf("7-day retention mismatch: %+v", retention)
			}
		} else {
			wantActive := []int64{6, 5, 8, 9}
			wantCohort := []int64{6, 6, 5, 8}
			wantReturned := []int64{5, 5, 5, 5}
			for i := range wantActive {
				if report.WeeklyExerciseDays[i].ActiveUsers != wantActive[i] || report.WeeklyRetention[i].CohortUsers != wantCohort[i] || report.WeeklyRetention[i].ReturnedUsers != wantReturned[i] {
					t.Fatalf("30-day week %d mismatch: activity=%+v retention=%+v", i, report.WeeklyExerciseDays[i], report.WeeklyRetention[i])
				}
			}
		}
		if report.Families.ActiveCount != 2 || report.Families.Daily[len(report.Families.Daily)-1].FamilyCount != 1 {
			t.Fatalf("family distinct-count or last-day assertion mismatch: %+v", report.Families)
		}
		if days == 7 && len(report.Families.Daily) != 7 || days == 30 && len(report.Families.Daily) != 30 {
			t.Fatalf("family daily series must zero-fill the full %d-day window", days)
		}
	}
}

func TestDashboardEngagementEmptySampleUsesNullRates(t *testing.T) {
	_, tx, s := dashboardFixture(t)
	for _, days := range []int{7, 30} {
		result, err := s.dashboardStats(days, dashboardTestNow)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		reconcileEngagement(t, tx, days, encoded, dashboardTestNow)
		var envelope engagementEnvelopeWire
		if err := json.Unmarshal(encoded, &envelope); err != nil {
			t.Fatal(err)
		}
		report := envelope.Engagement
		if report.Activation.EligibleUsers != 0 || report.Activation.Rate != nil {
			t.Fatalf("empty activation cohort must have a null rate: %+v", report.Activation)
		}
		for i, row := range report.WeeklyRetention {
			if row.CohortUsers != 0 || row.Rate != nil {
				t.Fatalf("empty retention cohort %d must have a null rate: %+v", i, row)
			}
		}
	}
}

func TestDashboardEngagementCompletedWeeksCrossYearAndExcludeSundayInProgress(t *testing.T) {
	_, _, s := dashboardFixture(t)
	for _, tc := range []struct {
		now       time.Time
		lastStart string
		lastEnd   string
	}{
		{time.Date(2033, 1, 3, 12, 0, 0, 0, clock.Zone), "2032-12-27", "2033-01-02"},
		{time.Date(2033, 1, 2, 12, 0, 0, 0, clock.Zone), "2032-12-20", "2032-12-26"},
	} {
		result, err := s.dashboardStats(7, tc.now)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var envelope engagementEnvelopeWire
		if err := json.Unmarshal(encoded, &envelope); err != nil {
			t.Fatal(err)
		}
		report := envelope.Engagement
		if len(report.WeeklyExerciseDays) != 1 || report.WeeklyExerciseDays[0].StartDate != tc.lastStart || report.WeeklyExerciseDays[0].EndDate != tc.lastEnd {
			t.Fatalf("wrong complete Monday-Sunday week for %s: %+v", tc.now.Format("2006-01-02"), report.WeeklyExerciseDays)
		}
	}
}
