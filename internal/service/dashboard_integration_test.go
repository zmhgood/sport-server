package service

import (
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"elderly-fitness/pkg/clock"
	"fmt"
	"gorm.io/gorm"
	"reflect"
	"sort"
	"testing"
	"time"
)

var dashboardTestNow = time.Date(2032, 3, 1, 9, 0, 0, 0, clock.Zone)

func dashboardFixture(t *testing.T) (*fixture, *gorm.DB, *UserService) {
	t.Helper()
	f := newFixture(t)
	tx := f.db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	return f, tx, NewUserService(repository.NewUserRepository(tx), repository.NewExerciseRepository(tx))
}
func dashboardMember(t *testing.T, db *gorm.DB, goalID, userID uint) model.GoalMember {
	t.Helper()
	var m model.GoalMember
	if err := db.Unscoped().Where("goal_id=? AND user_id=?", goalID, userID).First(&m).Error; err != nil {
		t.Fatal(err)
	}
	return m
}
func dashboardProgress(t *testing.T, db *gorm.DB, memberID, exerciseID uint, date string, sets int) model.GoalCompletion {
	t.Helper()
	day, err := time.ParseInLocation("2006-01-02", date, clock.Zone)
	if err != nil {
		t.Fatal(err)
	}
	row := model.GoalCompletion{GoalMemberID: memberID, GoalExerciseID: exerciseID, Date: day, CompletedSets: sets, Status: "pending"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

// Independent reconciliation: fetch raw rows without SQL aggregates and fold them
// in Go by the business key. It does not call the production query/helper.
func reconcileDashboard(t *testing.T, db *gorm.DB, result *DashboardStats) {
	t.Helper()
	var raw []struct {
		ID         uint
		UserID     uint
		GoalID     uint
		ExerciseID uint
		Date       string
		Name       string
		Sets       int
		Target     int
	}
	err := db.Raw(`SELECT c.id,m.user_id,m.goal_id,ge.exercise_id,DATE_FORMAT(c.date,'%Y-%m-%d') date,e.name,c.completed_sets sets,ge.sets target
 FROM goal_completions c JOIN goal_members m ON m.id=c.goal_member_id JOIN daily_goals g ON g.id=m.goal_id
 JOIN goal_exercises ge ON ge.id=c.goal_exercise_id JOIN exercises e ON e.id=ge.exercise_id
 WHERE c.deleted_at IS NULL AND ge.goal_id=m.goal_id AND c.date>=? AND c.date<=?`, result.StartDate, result.EndDate).Scan(&raw).Error
	if err != nil {
		t.Fatal(err)
	}
	latest := map[string]int{}
	for i, row := range raw {
		key := fmt.Sprintf("%d/%s/%d/%d", row.UserID, row.Date, row.GoalID, row.ExerciseID)
		j, ok := latest[key]
		if !ok || row.ID > raw[j].ID {
			latest[key] = i
		}
	}
	active := map[string]map[uint]bool{}
	hot := map[uint]repository.DashboardHotExercise{}
	completed := map[string]int64{}
	for _, i := range latest {
		row := raw[i]
		if row.Target <= 0 || row.Sets <= 0 || row.Sets > row.Target {
			continue
		}
		if active[row.Date] == nil {
			active[row.Date] = map[uint]bool{}
		}
		active[row.Date][row.UserID] = true
		if row.Sets == row.Target {
			item := hot[row.ExerciseID]
			item.ExerciseID = row.ExerciseID
			item.Name = row.Name
			item.Count++
			hot[row.ExerciseID] = item
			completed[row.Date]++
		}
	}
	expected := []repository.DashboardHotExercise{}
	for _, item := range hot {
		expected = append(expected, item)
	}
	sort.Slice(expected, func(i, j int) bool {
		if expected[i].Count != expected[j].Count {
			return expected[i].Count > expected[j].Count
		}
		return expected[i].ExerciseID < expected[j].ExerciseID
	})
	if len(expected) > 10 {
		expected = expected[:10]
	}
	if !reflect.DeepEqual(expected, result.HotExercises) {
		t.Fatalf("ranking reconciliation mismatch: expected=%+v actual=%+v", expected, result.HotExercises)
	}
	if len(result.DailyActivity) != result.Days {
		t.Fatal("missing zero-filled days")
	}
	for _, day := range result.DailyActivity {
		if day.UserCount != int64(len(active[day.Date])) {
			t.Fatalf("daily reconciliation mismatch: %s", day.Date)
		}
	}
	if result.TodayRecordCount != completed[result.EndDate] {
		t.Fatalf("today-card reconciliation mismatch: expected=%d actual=%d", completed[result.EndDate], result.TodayRecordCount)
	}
	t.Logf("reconciled days=%d raw_rows=%d logical_keys=%d ranked_actions=%d today_completed=%d", result.Days, len(raw), len(latest), len(expected), result.TodayRecordCount)
}

func TestDashboardEmptyWindowsAreZeroFilled(t *testing.T) {
	_, tx, s := dashboardFixture(t)
	for _, days := range []int{7, 30} {
		r, err := s.dashboardStats(days, dashboardTestNow)
		if err != nil {
			t.Fatal(err)
		}
		reconcileDashboard(t, tx, r)
		if len(r.HotExercises) != 0 || r.TodayRecordCount != 0 {
			t.Fatal("empty database window must not invent data")
		}
		for _, d := range r.DailyActivity {
			if d.UserCount != 0 {
				t.Fatal("empty day must be zero")
			}
		}
	}
}

func TestDashboardPartialCompleteRepeatsAndCrossFamilyReconcile(t *testing.T) {
	f, tx, s := dashboardFixture(t)
	a := dashboardMember(t, tx, f.goal.ID, f.users[0].ID)
	b := dashboardMember(t, tx, f.goal.ID, f.users[1].ID)
	x, y := f.goal.Exercises[0], f.goal.Exercises[1]
	var last model.GoalCompletion
	for _, sets := range []int{1, 3, 3} {
		last = dashboardProgress(t, tx, a.ID, x.ID, "2032-03-01", sets)
	}
	dashboardProgress(t, tx, a.ID, y.ID, "2032-03-01", 1)
	dashboardProgress(t, tx, b.ID, x.ID, "2032-03-01", 1)
	dashboardProgress(t, tx, b.ID, y.ID, "2032-03-01", 2)
	dashboardProgress(t, tx, b.ID, y.ID, "2032-03-01", 0) // Latest zero withdraws participation; old full row must not reappear.
	other := dashboardMember(t, tx, f.other.ID, f.users[0].ID)
	dashboardProgress(t, tx, other.ID, f.other.Exercises[0].ID, "2032-03-01", 3)
	family := model.Family{Name: "Synthetic second family", CreatorID: f.users[0].ID}
	if err := tx.Create(&family).Error; err != nil {
		t.Fatal(err)
	}
	goal := model.DailyGoal{FamilyID: family.ID, Name: "Synthetic personal plan", CreatedBy: f.users[0].ID, PlanType: "personal", ExecutorID: f.users[0].ID}
	if err := tx.Create(&goal).Error; err != nil {
		t.Fatal(err)
	}
	m := model.GoalMember{GoalID: goal.ID, UserID: f.users[0].ID}
	ge := model.GoalExercise{GoalID: goal.ID, ExerciseID: x.ExerciseID, Sets: 3, Reps: 10}
	if err := tx.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&ge).Error; err != nil {
		t.Fatal(err)
	}
	dashboardProgress(t, tx, m.ID, ge.ID, "2032-03-01", 3)
	// Mirrors and stale derived totals must not be added to the canonical progress.
	mirror := model.UserExerciseRecord{CompletionID: &last.ID, UserID: f.users[0].ID, ExerciseID: x.ExerciseID, Sets: 3, CompletedAt: dashboardTestNow}
	if err := tx.Create(&mirror).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.UserExerciseRecord{UserID: f.users[2].ID, ExerciseID: x.ExerciseID, Sets: 3, CompletedAt: dashboardTestNow}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.UserDailyStats{UserID: f.users[0].ID, Date: dashboardTestNow, ExerciseCount: 999}).Error; err != nil {
		t.Fatal(err)
	}
	r, err := s.dashboardStats(7, dashboardTestNow)
	if err != nil {
		t.Fatal(err)
	}
	reconcileDashboard(t, tx, r)
	if r.DailyActivity[6].UserCount != 2 || r.TodayRecordCount != 3 || len(r.HotExercises) != 1 || r.HotExercises[0].Count != 3 {
		t.Fatalf("wrong business totals: %+v", r)
	}
}

func TestDashboardBoundsAndArchivedHistory(t *testing.T) {
	f, tx, s := dashboardFixture(t)
	m := dashboardMember(t, tx, f.goal.ID, f.users[0].ID)
	ge := f.goal.Exercises[0]
	for _, date := range []string{"2032-01-31", "2032-02-01", "2032-02-23", "2032-02-24", "2032-02-29", "2032-03-01", "2032-03-02"} {
		dashboardProgress(t, tx, m.ID, ge.ID, date, 3)
	}
	for _, value := range []interface{}{&m, &ge, &f.goal, &f.family} {
		if err := tx.Delete(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Delete(&model.Exercise{}, ge.ExerciseID).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		days  int
		count int64
	}{{7, 3}, {30, 5}} {
		r, err := s.dashboardStats(tc.days, dashboardTestNow)
		if err != nil {
			t.Fatal(err)
		}
		reconcileDashboard(t, tx, r)
		if len(r.HotExercises) != 1 || r.HotExercises[0].Count != tc.count || r.TodayRecordCount != 1 {
			t.Fatalf("history or date boundary changed: %+v", r)
		}
	}
}

func TestDashboardLatestLogicalActionAndInvalidProgress(t *testing.T) {
	f, tx, s := dashboardFixture(t)
	m := dashboardMember(t, tx, f.goal.ID, f.users[0].ID)
	ge := f.goal.Exercises[0]
	dashboardProgress(t, tx, m.ID, ge.ID, "2032-03-01", 3)
	duplicate := model.GoalExercise{GoalID: f.goal.ID, ExerciseID: ge.ExerciseID, Sets: 3, Reps: 10}
	if err := tx.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	dashboardProgress(t, tx, m.ID, duplicate.ID, "2032-03-01", 1)
	r, err := s.dashboardStats(7, dashboardTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if r.TodayRecordCount != 0 || r.DailyActivity[6].UserCount != 1 {
		t.Fatal("same catalog action was double-counted")
	}
	reconcileDashboard(t, tx, r)
	for _, sets := range []int{0, -1, 4} {
		dashboardProgress(t, tx, m.ID, duplicate.ID, "2032-03-01", sets)
		r, err = s.dashboardStats(7, dashboardTestNow)
		if err != nil {
			t.Fatal(err)
		}
		if r.DailyActivity[6].UserCount != 0 || r.TodayRecordCount != 0 {
			t.Fatalf("invalid latest progress resurrected an old completion: %d", sets)
		}
		reconcileDashboard(t, tx, r)
	}
	// Cross-plan exercise references and zero targets are not valid participation.
	dashboardProgress(t, tx, m.ID, f.other.Exercises[0].ID, "2032-03-01", 3)
	zero := model.GoalExercise{GoalID: f.goal.ID, ExerciseID: f.goal.Exercises[1].ExerciseID, Sets: 0}
	if err := tx.Create(&zero).Error; err != nil {
		t.Fatal(err)
	}
	dashboardProgress(t, tx, m.ID, zero.ID, "2032-03-01", 1)
	r, err = s.dashboardStats(7, dashboardTestNow)
	if err != nil {
		t.Fatal(err)
	}
	reconcileDashboard(t, tx, r)
	if r.DailyActivity[6].UserCount != 0 {
		t.Fatal("invalid relation counted")
	}
}

func TestDashboardTopTenStableTiesAndSoftDeletedProgress(t *testing.T) {
	f, tx, s := dashboardFixture(t)
	m := dashboardMember(t, tx, f.goal.ID, f.users[0].ID)
	ids := []uint{}
	for i := 0; i < 12; i++ {
		ex := model.Exercise{Name: fmt.Sprintf("Synthetic rank '%d OR 1=1 --", i), MuscleGroupID: f.goal.Exercises[0].Exercise.MuscleGroupID, Sets: 1, Reps: "10"}
		if err := tx.Create(&ex).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, ex.ID)
		ge := model.GoalExercise{GoalID: f.goal.ID, ExerciseID: ex.ID, Sets: 1, Reps: 10}
		if err := tx.Create(&ge).Error; err != nil {
			t.Fatal(err)
		}
		dashboardProgress(t, tx, m.ID, ge.ID, "2032-03-01", 1)
		if i == 0 {
			dashboardProgress(t, tx, m.ID, ge.ID, "2032-02-29", 1)
		}
		if i == 11 {
			row := dashboardProgress(t, tx, m.ID, ge.ID, "2032-02-29", 1)
			if err := tx.Delete(&row).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	r, err := s.dashboardStats(30, dashboardTestNow)
	if err != nil {
		t.Fatal(err)
	}
	reconcileDashboard(t, tx, r)
	if len(r.HotExercises) != 10 || r.HotExercises[0].Count != 2 || r.TodayRecordCount != 12 {
		t.Fatal("top10 truncation changed totals")
	}
	for i, item := range r.HotExercises {
		if item.ExerciseID != ids[i] {
			t.Fatal("tied ranking is unstable")
		}
	}
}

func TestDashboardRealSubmissionUpdatesReconcileWithPersonalStats(t *testing.T) {
	f := newFixture(t)
	s := NewUserService(repository.NewUserRepository(f.db), repository.NewExerciseRepository(f.db))
	before, err := s.GetDashboardStats(7)
	if err != nil {
		t.Fatal(err)
	}
	for _, sets := range []int{1, 3, 3} {
		f.complete(t, 0, sets)
	}
	after, err := s.GetDashboardStats(7)
	if err != nil {
		t.Fatal(err)
	}
	if after.TodayRecordCount != before.TodayRecordCount+1 || after.DailyActivity[6].UserCount != before.DailyActivity[6].UserCount+1 {
		t.Fatal("repeated real submissions changed logical totals")
	}
	personal, err := f.userSvc.GetUserStatistics(f.users[0].ID)
	if err != nil || personal.TotalExercises != 1 || personal.TotalDays != 1 || personal.TotalSets != 3 {
		t.Fatalf("personal reconciliation: %+v %v", personal, err)
	}
	reconcileDashboard(t, f.db, after)
}

func TestDashboardPlanRevisionKeepsHistoricalTargets(t *testing.T) {
	f, tx, s := dashboardFixture(t)
	member := dashboardMember(t, tx, f.goal.ID, f.users[0].ID)
	old := f.goal.Exercises[0]
	dashboardProgress(t, tx, member.ID, old.ID, "2032-02-29", 3)
	effective, _ := time.ParseInLocation("2006-01-02", "2032-03-01", clock.Zone)
	revision := model.PlanRevision{GoalID: f.goal.ID, EffectiveDate: effective}
	if err := tx.Create(&revision).Error; err != nil {
		t.Fatal(err)
	}
	replacement := model.GoalExercise{GoalID: f.goal.ID, RevisionID: revision.ID, ExerciseID: old.ExerciseID, Sets: 2, Reps: 10}
	if err := tx.Create(&replacement).Error; err != nil {
		t.Fatal(err)
	}
	dashboardProgress(t, tx, member.ID, replacement.ID, "2032-03-01", 2)
	if err := tx.Delete(&old).Error; err != nil {
		t.Fatal(err)
	}
	result, err := s.dashboardStats(7, dashboardTestNow)
	if err != nil {
		t.Fatal(err)
	}
	reconcileDashboard(t, tx, result)
	if result.TodayRecordCount != 1 || len(result.HotExercises) != 1 || result.HotExercises[0].Count != 2 || result.DailyActivity[5].UserCount != 1 || result.DailyActivity[6].UserCount != 1 {
		t.Fatal("new action requirements changed historical completion counts")
	}
}
