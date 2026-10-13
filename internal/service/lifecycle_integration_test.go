package service

import (
	"elderly-fitness/config"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type fixture struct {
	db        *gorm.DB
	users     []model.User
	family    *model.Family
	goal      *model.DailyGoal
	other     *model.DailyGoal
	familySvc *FamilyService
	goalSvc   *GoalService
	userSvc   *UserService
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	path := os.Getenv("SPORT_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("isolated local MySQL config required")
	}
	if err := config.LoadConfig(path); err != nil {
		t.Fatal("test configuration unavailable")
	}
	cfg := config.AppConfig.Database
	if (cfg.Host != "127.0.0.1" && cfg.Host != "localhost") || !strings.HasPrefix(cfg.DBName, "sport_qa_") {
		t.Fatal("isolated local test database required")
	}
	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("local test connection failed")
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(20)
	t.Cleanup(func() { sqlDB.Close() })
	f := &fixture{db: db}
	fr := repository.NewFamilyRepository(db)
	gr := repository.NewGoalRepository(db)
	ur := repository.NewUserRepository(db)
	er := repository.NewExerciseRepository(db)
	f.familySvc = NewFamilyService(fr)
	f.goalSvc = NewGoalService(gr, fr, er)
	f.userSvc = NewUserService(ur, er)
	for i := 0; i < 3; i++ {
		u := model.User{OpenID: fmt.Sprintf("qa-%d-%d", time.Now().UnixNano(), i), NickName: fmt.Sprintf("Synthetic%d", i)}
		if err := ur.Create(&u); err != nil {
			t.Fatal(err)
		}
		f.users = append(f.users, u)
	}
	f.family, err = f.familySvc.CreateFamily("Synthetic family", f.users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.familySvc.JoinFamily(f.family.InviteCode, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	group := model.MuscleGroup{Name: "下肢"}
	if err = db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	var exercises []GoalExerciseInput
	for i, sets := range []int{3, 2} {
		ex := model.Exercise{Name: fmt.Sprintf("Synthetic%d", i), MuscleGroupID: group.ID, Sets: sets, Reps: "10"}
		if err = db.Create(&ex).Error; err != nil {
			t.Fatal(err)
		}
		exercises = append(exercises, GoalExerciseInput{ExerciseID: ex.ID, Sets: sets, Reps: 10})
	}
	f.goal, err = f.goalSvc.CreateGoal(f.family.ID, f.users[0].ID, CreateGoalInput{Name: "Synthetic goal", MemberIDs: []uint{f.users[0].ID, f.users[1].ID}, Exercises: exercises})
	if err != nil {
		t.Fatal(err)
	}
	f.other, err = f.goalSvc.CreateGoal(f.family.ID, f.users[0].ID, CreateGoalInput{Name: "Other goal", Exercises: exercises[:1]})
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func count(t *testing.T, db *gorm.DB, value interface{}, where string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	if err := db.Model(value).Where(where, args...).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
func (f *fixture) complete(t *testing.T, index, sets int) {
	t.Helper()
	if err := f.goalSvc.CompleteExercise(f.goal.ID, f.users[0].ID, CompleteExerciseInput{GoalExerciseID: f.goal.Exercises[index].ID, CompletedSets: sets}); err != nil {
		t.Fatal(err)
	}
}

func TestMembershipRejoinAndRole(t *testing.T) {
	f := newFixture(t)
	u := f.users[1].ID
	fr := repository.NewFamilyRepository(f.db)
	before, _ := fr.GetMemberByFamilyAndUser(f.family.ID, u)
	if err := f.familySvc.RemoveMember(f.family.ID, f.users[0].ID, u); err != nil {
		t.Fatal(err)
	}
	if _, err := f.familySvc.JoinFamily(f.family.InviteCode, u); err != nil {
		t.Fatalf("removed member must rejoin: %v", err)
	}
	after, _ := fr.GetMemberByFamilyAndUser(f.family.ID, u)
	if after.ID != before.ID || after.Role != "member" {
		t.Fatal("rejoin changed identity or restored admin")
	}
	if n := count(t, f.db.Unscoped(), &model.FamilyMember{}, "family_id=? AND user_id=?", f.family.ID, u); n != 1 {
		t.Fatalf("membership count %d", n)
	}
	if err := f.familySvc.LeaveFamily(f.family.ID, u); err != nil {
		t.Fatal(err)
	}
	if _, err := f.familySvc.JoinFamily(f.family.InviteCode, u); err != nil {
		t.Fatal(err)
	}
	if err := f.familySvc.RemoveMember(f.family.ID, f.users[0].ID, f.users[0].ID); err == nil {
		t.Fatal("administrator must not remove self")
	}
	if err := f.familySvc.TransferAdmin(f.family.ID, f.users[0].ID, f.users[2].ID); err == nil {
		t.Fatal("transfer to outsider must fail")
	}
	if err := f.familySvc.TransferAdmin(f.family.ID, f.users[0].ID, u); err != nil {
		t.Fatal(err)
	}
	if n := count(t, f.db, &model.FamilyMember{}, "family_id=? AND role=?", f.family.ID, "admin"); n != 1 {
		t.Fatalf("administrators %d", n)
	}
}
func TestRemovedMemberCannotSubmitAndHistoricalDataRemain(t *testing.T) {
	f := newFixture(t)
	f.complete(t, 0, 3)
	if err := f.familySvc.RemoveMember(f.family.ID, f.users[0].ID, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.goalSvc.CompleteExercise(f.goal.ID, f.users[1].ID, CompleteExerciseInput{GoalExerciseID: f.goal.Exercises[0].ID, CompletedSets: 3}); err == nil {
		t.Fatal("removed member submitted")
	}
	goals, err := f.goalSvc.GetUserGoals(f.users[1].ID)
	if err != nil || len(goals) != 0 {
		t.Fatal("removed member still sees active family targets")
	}
	if err := f.familySvc.LeaveFamily(f.family.ID, f.users[0].ID); err != nil {
		t.Fatal(err)
	}
	stats, err := repository.NewUserRepository(f.db).GetUserTotalStats(f.users[0].ID)
	if err != nil || stats.TotalSets != 3 || stats.TotalExercises != 1 {
		t.Fatalf("personal history lost: %+v %v", stats, err)
	}
	if n := count(t, f.db, &model.GoalCompletion{}, "goal_exercise_id=?", f.goal.Exercises[0].ID); n != 1 {
		t.Fatal("history deleted")
	}
}
func TestProgressOwnershipBoundsAndNoPartialWrites(t *testing.T) {
	f := newFixture(t)
	for _, in := range []CompleteExerciseInput{{f.other.Exercises[0].ID, 1}, {99999999, 1}, {f.goal.Exercises[0].ID, -1}, {f.goal.Exercises[0].ID, 4}} {
		if err := f.goalSvc.CompleteExercise(f.goal.ID, f.users[0].ID, in); err == nil {
			t.Errorf("invalid progress accepted %+v", in)
		}
	}
	gm, _ := repository.NewGoalRepository(f.db).GetGoalMember(f.goal.ID, f.users[0].ID)
	if n := count(t, f.db, &model.GoalCompletion{}, "goal_member_id=?", gm.ID); n != 0 {
		t.Fatalf("invalid request left %d rows", n)
	}
	if err := f.goalSvc.DeleteGoal(f.goal.ID, f.users[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.goalSvc.CompleteExercise(f.goal.ID, f.users[0].ID, CompleteExerciseInput{f.goal.Exercises[0].ID, 3}); err == nil {
		t.Fatal("deleted target accepted completion")
	}
}
func TestProgressIdempotencyConcurrentAndStatReconciliation(t *testing.T) {
	f := newFixture(t)
	f.complete(t, 0, 1)
	f.complete(t, 1, 1)
	stats, err := repository.NewUserRepository(f.db).GetUserTotalStats(f.users[0].ID)
	if err != nil || stats.TotalSets != 2 || stats.TotalExercises != 0 || stats.TotalMinutes != 1 || stats.TotalDays != 1 {
		t.Fatalf("partial stats %+v %v", stats, err)
	}
	week, err := f.userSvc.GetWeekStats(f.users[0].ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	minutes := 0
	for _, d := range week.Stats {
		minutes += d.Minutes
	}
	if minutes != 1 {
		t.Fatalf("week minutes %d", minutes)
	}
	history, _, err := f.userSvc.GetExerciseHistoryPaginated(f.users[0].ID, 1, 10)
	if err != nil || len(history) != 1 || history[0].Progress != 0 {
		t.Fatalf("partial history %+v %v", history, err)
	}
	for i := 0; i < 3; i++ {
		f.complete(t, 0, 3)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- f.goalSvc.CompleteExercise(f.goal.ID, f.users[0].ID, CompleteExerciseInput{f.goal.Exercises[1].ID, 2})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	gm, _ := repository.NewGoalRepository(f.db).GetGoalMember(f.goal.ID, f.users[0].ID)
	if n := count(t, f.db, &model.GoalCompletion{}, "goal_member_id=?", gm.ID); n != 2 {
		t.Fatalf("completion rows %d", n)
	}
	stats, err = repository.NewUserRepository(f.db).GetUserTotalStats(f.users[0].ID)
	if err != nil || stats.TotalSets != 5 || stats.TotalExercises != 2 || stats.TotalMinutes != 2 {
		t.Fatalf("full stats %+v %v", stats, err)
	}
	var independent struct {
		Sets      int
		Completed int
		Seconds   int
	}
	if err := f.db.Raw("SELECT SUM(gc.completed_sets) sets,SUM(CASE WHEN gc.status='done' THEN 1 ELSE 0 END) completed,SUM(gc.completed_sets*30) seconds FROM goal_completions gc JOIN goal_members gm ON gm.id=gc.goal_member_id WHERE gm.user_id=? AND gc.deleted_at IS NULL", f.users[0].ID).Scan(&independent).Error; err != nil {
		t.Fatal(err)
	}
	if independent.Sets != 5 || independent.Completed != 2 || independent.Seconds != 150 {
		t.Fatal("independent SQL reconciliation mismatch")
	}
	var daily model.UserDailyStats
	if err = f.db.Where("user_id=?", f.users[0].ID).First(&daily).Error; err != nil {
		t.Fatal(err)
	}
	if daily.ExerciseCount != 2 || daily.TotalDuration != 150 {
		t.Fatalf("daily aggregates %+v", daily)
	}
	if n := count(t, f.db, &model.UserExerciseRecord{}, "user_id=?", f.users[0].ID); n != 2 {
		t.Fatalf("duplicate ledger %d", n)
	}
	f.complete(t, 0, 0)
	stats, _ = repository.NewUserRepository(f.db).GetUserTotalStats(f.users[0].ID)
	if stats.TotalSets != 2 || stats.TotalExercises != 1 {
		t.Fatalf("zero reduction %+v", stats)
	}
}

func TestFirstProgressRaceAndAtomicRollback(t *testing.T) {
	f := newFixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- f.goalSvc.CompleteExercise(f.goal.ID, f.users[0].ID, CompleteExerciseInput{f.goal.Exercises[0].ID, 3})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	gm, _ := repository.NewGoalRepository(f.db).GetGoalMember(f.goal.ID, f.users[0].ID)
	if count(t, f.db, &model.GoalCompletion{}, "goal_member_id=?", gm.ID) != 1 || count(t, f.db, &model.UserExerciseRecord{}, "user_id=?", f.users[0].ID) != 1 {
		t.Fatal("first submit race duplicates")
	}
	name := fmt.Sprintf("qa-failure-%d", time.Now().UnixNano())
	f.db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "user_exercise_records" {
			tx.AddError(fmt.Errorf("synthetic ledger failure"))
		}
	})
	defer f.db.Callback().Create().Remove(name)
	err := f.goalSvc.CompleteExercise(f.goal.ID, f.users[0].ID, CompleteExerciseInput{f.goal.Exercises[1].ID, 2})
	if err == nil {
		t.Fatal("injected failure falsely succeeded")
	}
	if count(t, f.db, &model.GoalCompletion{}, "goal_exercise_id=?", f.goal.Exercises[1].ID) != 0 {
		t.Fatal("partial progress remained after ledger failure")
	}
}
func TestGoalCreationAndTransferRollback(t *testing.T) {
	f := newFixture(t)
	beforeFamilies := count(t, f.db, &model.Family{}, "creator_id=?", f.users[0].ID)
	failureName := fmt.Sprintf("qa-family-fail-%d", time.Now().UnixNano())
	f.db.Callback().Create().Before("gorm:create").Register(failureName, func(tx *gorm.DB) {
		if tx.Statement.Table == "family_members" {
			tx.AddError(fmt.Errorf("synthetic admin member failure"))
		}
	})
	_, familyErr := f.familySvc.CreateFamily("Should rollback", f.users[0].ID)
	f.db.Callback().Create().Remove(failureName)
	if familyErr == nil || count(t, f.db, &model.Family{}, "creator_id=?", f.users[0].ID) != beforeFamilies {
		t.Fatal("failed family left orphan family")
	}

	before := count(t, f.db, &model.DailyGoal{}, "family_id=?", f.family.ID)
	name := fmt.Sprintf("qa-create-fail-%d", time.Now().UnixNano())
	f.db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "goal_exercises" {
			tx.AddError(fmt.Errorf("synthetic action failure"))
		}
	})
	_, err := f.goalSvc.CreateGoal(f.family.ID, f.users[0].ID, CreateGoalInput{Name: "Should rollback", Exercises: []GoalExerciseInput{{f.goal.Exercises[0].ExerciseID, 3, 10}}})
	f.db.Callback().Create().Remove(name)
	if err == nil || count(t, f.db, &model.DailyGoal{}, "family_id=?", f.family.ID) != before {
		t.Fatal("failed create left target")
	}
	name = fmt.Sprintf("qa-role-fail-%d", time.Now().UnixNano())
	f.db.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "family_members" {
			if values, ok := tx.Statement.Dest.(map[string]interface{}); ok && values["role"] == "member" {
				tx.AddError(fmt.Errorf("synthetic role failure"))
			}
		}
	})
	err = f.familySvc.TransferAdmin(f.family.ID, f.users[0].ID, f.users[1].ID)
	f.db.Callback().Update().Remove(name)
	if err == nil {
		t.Fatal("injected role failure succeeded")
	}
	fm, _ := repository.NewFamilyRepository(f.db).GetMemberByFamilyAndUser(f.family.ID, f.users[0].ID)
	if fm.Role != "admin" || count(t, f.db, &model.FamilyMember{}, "family_id=? AND role=?", f.family.ID, "admin") != 1 {
		t.Fatal("transfer half committed")
	}
}
func TestCrossFamilyStatsArchivedHistoryAndDistinctDays(t *testing.T) {
	f := newFixture(t)
	f.complete(t, 0, 3)
	second, err := f.familySvc.CreateFamily("Other synthetic family", f.users[2].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.familySvc.JoinFamily(second.InviteCode, f.users[0].ID); err != nil {
		t.Fatal(err)
	}
	other, err := f.goalSvc.CreateGoal(second.ID, f.users[0].ID, CreateGoalInput{Name: "Other family target", Exercises: []GoalExerciseInput{{f.goal.Exercises[0].ExerciseID, 1, 10}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.goalSvc.CompleteExercise(other.ID, f.users[0].ID, CompleteExerciseInput{other.Exercises[0].ID, 1}); err != nil {
		t.Fatal(err)
	}
	stats, _ := repository.NewUserRepository(f.db).GetUserTotalStats(f.users[0].ID)
	if stats.TotalDays != 1 || stats.TotalExercises != 2 || stats.TotalSets != 4 {
		t.Fatalf("cross-family aggregate %+v", stats)
	}
	if err = f.familySvc.RemoveMember(f.family.ID, f.users[0].ID, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	if err = f.familySvc.LeaveFamily(f.family.ID, f.users[0].ID); err != nil {
		t.Fatal(err)
	}
	rows, err := f.userSvc.GetFamilyStatistics(f.users[0].ID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("family history %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.FamilyID == f.family.ID && (row.IsCurrentMember || row.TotalSets != 3) {
			t.Fatal("archived family not retained and marked")
		}
	}
	history, _, err := f.userSvc.GetExerciseHistoryPaginated(f.users[0].ID, -1, 10)
	if err != nil || len(history) != 2 {
		t.Fatal("history or safe pagination failed")
	}
}

func TestProfilePersistenceAndCommentIntegrity(t *testing.T) {
	f := newFixture(t)
	auth := NewAuthService(repository.NewUserRepository(f.db), nil)
	nickname := "Synthetic profile"
	gender, age, height, weight := 2, 65, 172, 66.5
	health := "Synthetic note"
	if err := auth.UpdateUserInfo(f.users[0].ID, &UserInfoUpdate{NickName: &nickname, Gender: &gender, Age: &age, Height: &height, Weight: &weight, HealthStatus: &health}); err != nil {
		t.Fatal(err)
	}
	var actual model.User
	if err := f.db.First(&actual, f.users[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if actual.Gender != 2 || actual.Height != 172 || actual.Weight != 66.5 || actual.NickName != nickname {
		t.Fatal("profile not persisted")
	}
	health = ""
	age = 0
	gender = 0
	if err := auth.UpdateUserInfo(f.users[0].ID, &UserInfoUpdate{Age: &age, Gender: &gender, HealthStatus: &health}); err != nil {
		t.Fatal(err)
	}
	f.db.First(&actual, f.users[0].ID)
	if actual.HealthStatus != "" || actual.Age != 0 || actual.Gender != 0 || actual.Weight != 66.5 {
		t.Fatal("clear or omitted field semantics invalid")
	}
	cr := repository.NewCommentRepository(f.db)
	cs := NewCommentService(cr)
	comment, err := cs.CreateComment(f.users[0].ID, &CreateCommentRequest{ExerciseID: f.goal.Exercises[0].ExerciseID, Content: strings.Repeat("测", 500)})
	if err != nil {
		t.Fatal(err)
	}
	f.userSvc.SetCommentRepo(cr)
	if err := f.userSvc.UpdateCommentStatus(comment.ID, 0); err != nil {
		t.Fatal(err)
	}
	changed, err := cr.GetByID(comment.ID)
	if err != nil || changed.Content != comment.Content || changed.UserID != f.users[0].ID || changed.ExerciseID != comment.ExerciseID {
		t.Fatal("moderation destroyed unrelated fields")
	}
	if err := f.userSvc.UpdateCommentStatus(comment.ID, 1); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- cs.LikeComment(comment.ID, f.users[1].ID) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	changed, _ = cr.GetByID(comment.ID)
	if changed.LikeCount != 1 || count(t, f.db, &model.CommentLike{}, "comment_id=?", comment.ID) != 1 {
		t.Fatal("concurrent likes drifted")
	}
	if err := cs.UnlikeComment(comment.ID, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := cs.LikeComment(comment.ID, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CreateComment(f.users[0].ID, &CreateCommentRequest{ExerciseID: f.goal.Exercises[1].ExerciseID, ParentID: &comment.ID, Content: "Synthetic cross-action reply"}); err == nil {
		t.Fatal("cross-action reply accepted")
	}
}
func TestLegacyDuplicateProgressReadOnceAndRetained(t *testing.T) {
	f := newFixture(t)
	gm, _ := repository.NewGoalRepository(f.db).GetGoalMember(f.goal.ID, f.users[0].ID)
	for _, sets := range []int{1, 3} {
		if err := f.db.Create(&model.GoalCompletion{GoalMemberID: gm.ID, GoalExerciseID: f.goal.Exercises[0].ID, Date: time.Now(), CompletedSets: sets}).Error; err != nil {
			t.Fatal(err)
		}
	}
	f.complete(t, 0, 2)
	stats, _ := repository.NewUserRepository(f.db).GetUserTotalStats(f.users[0].ID)
	if stats.TotalSets != 2 || stats.TotalExercises != 0 {
		t.Fatalf("duplicate read counts %+v", stats)
	}
	if count(t, f.db, &model.GoalCompletion{}, "goal_member_id=?", gm.ID) != 2 {
		t.Fatal("legacy rows deleted")
	}
}

func TestHistoryOverOneThousandAndWeekBoundary(t *testing.T) {
	f := newFixture(t)
	gm, _ := repository.NewGoalRepository(f.db).GetGoalMember(f.goal.ID, f.users[0].ID)
	today := time.Now()
	records := make([]model.GoalCompletion, 1005)
	for i := range records {
		records[i] = model.GoalCompletion{GoalMemberID: gm.ID, GoalExerciseID: f.goal.Exercises[0].ID, Date: today.AddDate(0, 0, -i), CompletedSets: 3, Status: "done"}
	}
	if err := f.db.CreateInBatches(records, 100).Error; err != nil {
		t.Fatal(err)
	}
	_, total, err := f.userSvc.GetExerciseHistoryPaginated(f.users[0].ID, 101, 10)
	if err != nil || total != 1005 {
		t.Fatalf("truncated history %d %v", total, err)
	}
	stats, _ := repository.NewUserRepository(f.db).GetUserTotalStats(f.users[0].ID)
	if stats.TotalDays != 1005 || stats.TotalExercises != 1005 {
		t.Fatal("history aggregate mismatch")
	}
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := today.AddDate(0, 0, 1-weekday).Format("2006-01-02")
	week, err := repository.NewUserRepository(f.db).GetWeekCompletions(f.users[0].ID, monday)
	if err != nil || len(week) != weekday {
		t.Fatalf("week midnight boundary %d want %d", len(week), weekday)
	}
}

func TestCreateRequestRetryIsIdempotent(t *testing.T) {
	f := newFixture(t)
	key := fmt.Sprintf("qa-request-%d", time.Now().UnixNano())
	input := CreateGoalInput{Name: "Synthetic retry", RequestID: key, Exercises: []GoalExerciseInput{{f.goal.Exercises[0].ExerciseID, 3, 10}}}
	before := count(t, f.db, &model.DailyGoal{}, "family_id=?", f.family.ID)
	ids := make(chan uint, 10)
	errs := make(chan error, 10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			goal, err := f.goalSvc.CreateGoal(f.family.ID, f.users[0].ID, input)
			if err != nil {
				errs <- err
				return
			}
			ids <- goal.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var id uint
	for next := range ids {
		if id != 0 && id != next {
			t.Fatal("retry created a new target")
		}
		id = next
	}
	if count(t, f.db, &model.DailyGoal{}, "family_id=?", f.family.ID) != before+1 {
		t.Fatal("create count drift")
	}
	family1, err := f.familySvc.CreateFamily("Synthetic family retry", f.users[0].ID, key)
	if err != nil {
		t.Fatal(err)
	}
	family2, err := f.familySvc.CreateFamily("Synthetic family retry", f.users[0].ID, key)
	if err != nil || family1.ID != family2.ID {
		t.Fatal("family retry not idempotent")
	}
}

func TestFamilyHistoryMarksDepartedMemberAndRejoin(t *testing.T) {
	f := newFixture(t)
	if err := f.goalSvc.CompleteExercise(f.goal.ID, f.users[1].ID, CompleteExerciseInput{GoalExerciseID: f.goal.Exercises[0].ID, CompletedSets: 3}); err != nil {
		t.Fatal(err)
	}
	if err := f.familySvc.RemoveMember(f.family.ID, f.users[0].ID, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	p, err := f.goalSvc.GetGoalProgress(f.goal.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	check := func(p *GoalProgress, current bool) {
		t.Helper()
		found := false
		for _, m := range p.MemberStats {
			if m.UserID == f.users[1].ID {
				found = true
				if m.IsCurrentMember != current || m.CompletedSets != 3 || m.CompletedCount != 1 {
					t.Fatalf("member history: %+v", m)
				}
			}
		}
		if !found {
			t.Fatal("departed history disappeared")
		}
	}
	check(p, false)
	if _, err = f.familySvc.JoinFamily(f.family.InviteCode, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	p, err = f.goalSvc.GetGoalProgress(f.goal.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	check(p, true)
	if len(p.MemberStats) != 2 {
		t.Fatal("rejoin duplicated target participant")
	}
}
