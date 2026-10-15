package service

import (
	"database/sql"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"elderly-fitness/pkg/clock"
	"fmt"
	"testing"
	"time"
)

func createPlan(t *testing.T, f *fixture, creatorID uint, planType string, executorID uint, memberIDs []uint, exercises []GoalExerciseInput) *model.DailyGoal {
	t.Helper()
	goal, err := f.goalSvc.CreateGoal(f.family.ID, creatorID, CreateGoalInput{
		Name:       "Plan integration",
		PlanType:   planType,
		ExecutorID: executorID,
		MemberIDs:  memberIDs,
		Exercises:  exercises,
	})
	if err != nil {
		t.Fatalf("CreateGoal(%s): %v", planType, err)
	}
	return goal
}

func goalMemberCount(t *testing.T, f *fixture, goalID uint) int64 {
	t.Helper()
	return count(t, f.db, &model.GoalMember{}, "goal_id = ?", goalID)
}

func hasGoalID(goals []GoalWithProgress, goalID uint) bool {
	for _, goal := range goals {
		if goal.ID == goalID {
			return true
		}
	}
	return false
}

func weekStatForDate(t *testing.T, result *WeekStatsResult, date string) DailyStat {
	t.Helper()
	for _, stat := range result.Stats {
		if stat.Date == date {
			return stat
		}
	}
	t.Fatalf("week stats did not include date %s", date)
	return DailyStat{}
}

func TestPlanCreationParticipationAndPermissions(t *testing.T) {
	f := newFixture(t)
	exercises := []GoalExerciseInput{{ExerciseID: f.goal.Exercises[0].ExerciseID, Sets: 3, Reps: 10}}
	adminID, memberID, outsiderID := f.users[0].ID, f.users[1].ID, f.users[2].ID

	personal := createPlan(t, f, adminID, "personal", memberID, nil, exercises)
	if personal.ExecutorID != memberID || personal.PlanType != "personal" {
		t.Fatalf("personal plan metadata: type=%q executor=%d", personal.PlanType, personal.ExecutorID)
	}
	if n := goalMemberCount(t, f, personal.ID); n != 1 {
		t.Fatalf("personal plan should have only the executor participating, got %d members", n)
	}
	if n := count(t, f.db, &model.GoalMember{}, "goal_id = ? AND user_id = ?", personal.ID, memberID); n != 1 {
		t.Fatalf("executor membership count = %d", n)
	}
	if n := count(t, f.db, &model.GoalMember{}, "goal_id = ? AND user_id = ?", personal.ID, adminID); n != 0 {
		t.Fatalf("creator was implicitly added to personal plan: %d rows", n)
	}
	if err := f.goalSvc.AddGoalMember(personal.ID, memberID, memberID); err == nil {
		t.Fatal("personal plan accepted a voluntary join")
	}
	if _, err := f.goalSvc.CreateGoal(f.family.ID, memberID, CreateGoalInput{
		Name: "Personal for another user", PlanType: "personal", ExecutorID: adminID, Exercises: exercises,
	}); err == nil {
		t.Fatal("ordinary member created a personal plan for another executor")
	}
	if _, err := f.goalSvc.CreateGoal(f.family.ID, memberID, CreateGoalInput{
		Name: "Personal outside family", PlanType: "personal", ExecutorID: outsiderID, Exercises: exercises,
	}); err == nil {
		t.Fatal("personal plan accepted an executor outside the family")
	}

	shared := createPlan(t, f, adminID, "shared", 0, nil, exercises)
	if n := goalMemberCount(t, f, shared.ID); n != 1 {
		t.Fatalf("empty shared membership should default to creator only, got %d", n)
	}
	unjoinedShared := createPlan(t, f, adminID, "shared", 0, []uint{}, exercises)
	if n := goalMemberCount(t, f, unjoinedShared.ID); n != 0 {
		t.Fatalf("explicit empty shared membership should enroll nobody, got %d members", n)
	}
	if n := count(t, f.db, &model.GoalMember{}, "goal_id = ? AND user_id = ?", shared.ID, memberID); n != 0 {
		t.Fatal("shared plan forcibly enrolled another family member")
	}
	if err := f.goalSvc.AddGoalMember(shared.ID, adminID, memberID); err == nil {
		t.Fatal("creator enrolled another user without their consent")
	}
	if err := f.goalSvc.AddGoalMember(shared.ID, memberID, memberID); err != nil {
		t.Fatalf("member could not join shared plan: %v", err)
	}
	if err := f.goalSvc.AddGoalMember(shared.ID, memberID, memberID); err != nil {
		t.Fatalf("repeated shared-plan join should be idempotent: %v", err)
	}
	if n := count(t, f.db, &model.GoalMember{}, "goal_id = ? AND user_id = ?", shared.ID, memberID); n != 1 {
		t.Fatalf("repeated join created %d membership rows", n)
	}
	if _, err := f.goalSvc.UpdateGoal(shared.ID, memberID, UpdateGoalInput{Name: "Unauthorized edit"}); err == nil {
		t.Fatal("non-creator shared-plan participant updated the plan")
	}
	if err := f.goalSvc.DeleteGoal(shared.ID, memberID); err == nil {
		t.Fatal("non-creator shared-plan participant archived the plan")
	}
	stillActive, err := f.goalSvc.GetGoal(shared.ID)
	if err != nil || !stillActive.IsActive || stillActive.Name != "Plan integration" {
		t.Fatalf("unauthorized shared-plan mutation changed the plan: goal=%+v err=%v", stillActive, err)
	}
	if err := f.goalSvc.LeaveGoal(shared.ID, memberID); err != nil {
		t.Fatalf("member could not leave shared plan: %v", err)
	}
	if err := f.goalSvc.LeaveGoal(shared.ID, memberID); err != nil {
		t.Fatalf("repeated leave should be idempotent: %v", err)
	}
	if err := f.goalSvc.AddGoalMember(personal.ID, outsiderID, outsiderID); err == nil {
		t.Fatal("outsider enrolled in a personal plan")
	}

	adminView, err := f.goalSvc.GetGoalForUser(personal.ID, adminID)
	if err != nil {
		t.Fatal(err)
	}
	if adminView.IsParticipating {
		t.Fatal("administrator is shown as participating in the executor's personal plan")
	}
	if !adminView.CanManage {
		t.Fatal("current administrator cannot manage personal plan")
	}
	memberFamilies, err := f.goalSvc.GetUserFamiliesWithGoals(memberID)
	if err != nil {
		t.Fatal(err)
	}
	if len(memberFamilies) == 0 || hasGoalID(memberFamilies[0].Goals, shared.ID) {
		t.Fatal("family goal list included a shared plan after the member left")
	}
}

func TestPlanManagementFollowsCurrentFamilyAdmin(t *testing.T) {
	f := newFixture(t)
	exercises := []GoalExerciseInput{{ExerciseID: f.goal.Exercises[0].ExerciseID, Sets: 3, Reps: 10}}
	oldAdmin, executor, newAdmin := f.users[0].ID, f.users[1].ID, f.users[2].ID
	if _, err := f.familySvc.JoinFamily(f.family.InviteCode, newAdmin); err != nil {
		t.Fatal(err)
	}
	personal := createPlan(t, f, oldAdmin, "personal", executor, nil, exercises)
	shared := createPlan(t, f, oldAdmin, "shared", 0, nil, exercises)
	if err := f.familySvc.TransferAdmin(f.family.ID, oldAdmin, newAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := f.goalSvc.UpdateGoal(personal.ID, oldAdmin, UpdateGoalInput{Name: "Former admin"}); err == nil {
		t.Fatal("former administrator still manages a personal plan they do not execute")
	}
	if _, err := f.goalSvc.UpdateGoal(personal.ID, newAdmin, UpdateGoalInput{Name: "Current admin"}); err != nil {
		t.Fatalf("current administrator cannot manage personal plan: %v", err)
	}
	if _, err := f.goalSvc.UpdateGoal(shared.ID, newAdmin, UpdateGoalInput{Name: "Admin managed shared"}); err != nil {
		t.Fatalf("new administrator cannot manage shared plan: %v", err)
	}
	formerAdminView, err := f.goalSvc.GetGoalForUser(personal.ID, oldAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if formerAdminView.CanManage {
		t.Fatalf("former admin who does not execute the personal plan should lose management: view=%+v", formerAdminView)
	}
}

func TestPlanRevisionImmediateWithoutProgressAndRejectsStaleAction(t *testing.T) {
	f := newFixture(t)
	goal := createPlan(t, f, f.users[0].ID, "shared", 0, nil, []GoalExerciseInput{{
		ExerciseID: f.goal.Exercises[0].ExerciseID, Sets: 3, Reps: 10,
	}})
	before, err := f.goalSvc.GetGoalForUser(goal.ID, f.users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	oldExercise := goal.Exercises[0]
	newExerciseID := f.goal.Exercises[1].ExerciseID
	_, err = f.goalSvc.UpdateGoal(goal.ID, f.users[0].ID, UpdateGoalInput{Exercises: []GoalExerciseInput{{
		ExerciseID: newExerciseID, Sets: 2, Reps: 12,
	}}})
	if err != nil {
		t.Fatalf("update without progress: %v", err)
	}
	updated, err := f.goalSvc.GetGoalForUser(goal.ID, f.users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RevisionID == before.RevisionID {
		t.Fatalf("exercise update did not create a revision: before=%d after=%d", before.RevisionID, updated.RevisionID)
	}
	if updated.PendingEffectiveDate != "" {
		t.Fatalf("no-progress edit should apply today, pending=%q", updated.PendingEffectiveDate)
	}
	progress, err := f.goalSvc.GetGoalProgress(goal.ID, clock.Now().Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.MemberStats) != 1 || len(progress.MemberStats[0].ExerciseStats) != 1 || progress.MemberStats[0].ExerciseStats[0].ExerciseID != newExerciseID {
		t.Fatalf("today's progress did not use updated revision: %+v", progress.MemberStats)
	}
	if err := f.goalSvc.CompleteExercise(goal.ID, f.users[0].ID, CompleteExerciseInput{
		GoalExerciseID: oldExercise.ID, CompletedSets: 1,
	}); err == nil {
		t.Fatal("stale exercise from replaced revision accepted completion")
	}
}

func TestPlanRevisionWaitsForTomorrowAndPreservesHistoryAndStats(t *testing.T) {
	f := newFixture(t)
	goal := createPlan(t, f, f.users[0].ID, "shared", 0, nil, []GoalExerciseInput{{
		ExerciseID: f.goal.Exercises[0].ExerciseID, Sets: 3, Reps: 10,
	}})
	oldExercise := goal.Exercises[0]
	today := clock.Now().Format("2006-01-02")
	if err := f.goalSvc.CompleteExercise(goal.ID, f.users[0].ID, CompleteExerciseInput{
		GoalExerciseID: oldExercise.ID, CompletedSets: 3, Date: today,
	}); err != nil {
		t.Fatal(err)
	}
	statsBefore, err := repository.NewUserRepository(f.db).GetUserTotalStats(f.users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	newExerciseID := f.goal.Exercises[1].ExerciseID
	_, err = f.goalSvc.UpdateGoal(goal.ID, f.users[0].ID, UpdateGoalInput{Exercises: []GoalExerciseInput{{
		ExerciseID: newExerciseID, Sets: 2, Reps: 12,
	}}})
	if err != nil {
		t.Fatalf("update after progress: %v", err)
	}
	_, err = f.goalSvc.UpdateGoal(goal.ID, f.users[0].ID, UpdateGoalInput{Exercises: []GoalExerciseInput{{
		ExerciseID: newExerciseID, Sets: 4, Reps: 12,
	}}})
	if err != nil {
		t.Fatalf("repeat edit of pending revision: %v", err)
	}
	updated, err := f.goalSvc.GetGoalForUser(goal.ID, f.users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	tomorrow := clock.Now().AddDate(0, 0, 1).Format("2006-01-02")
	if updated.PendingEffectiveDate != tomorrow {
		t.Fatalf("edit should be pending tomorrow: got %q want %q", updated.PendingEffectiveDate, tomorrow)
	}
	todayProgress, err := f.goalSvc.GetGoalProgress(goal.ID, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(todayProgress.Exercises) != 1 || todayProgress.Exercises[0].ID != oldExercise.ID || todayProgress.Exercises[0].RevisionID != 0 || todayProgress.Exercises[0].Sets != 3 {
		t.Fatalf("today's historical plan snapshot lost the original exercise requirement: %+v", todayProgress.Exercises)
	}
	if len(todayProgress.MemberStats) != 1 || len(todayProgress.MemberStats[0].ExerciseStats) != 1 || todayProgress.MemberStats[0].ExerciseStats[0].GoalExerciseID != oldExercise.ID {
		t.Fatalf("today's historical progress changed after pending edit: %+v", todayProgress.MemberStats)
	}
	if todayProgress.MemberStats[0].ExerciseStats[0].TargetSets != todayProgress.Exercises[0].Sets || todayProgress.MemberStats[0].ExerciseStats[0].CompletedSets != 3 {
		t.Fatalf("today's completion no longer matches the historical requirement snapshot: exercise=%+v progress=%+v", todayProgress.Exercises[0], todayProgress.MemberStats[0].ExerciseStats[0])
	}
	tomorrowProgress, err := f.goalSvc.GetGoalProgress(goal.ID, tomorrow)
	if err != nil {
		t.Fatal(err)
	}
	if len(tomorrowProgress.MemberStats) != 1 || len(tomorrowProgress.MemberStats[0].ExerciseStats) != 1 || tomorrowProgress.MemberStats[0].ExerciseStats[0].ExerciseID != newExerciseID || tomorrowProgress.MemberStats[0].ExerciseStats[0].TargetSets != 4 {
		t.Fatalf("tomorrow's progress did not use pending revision: %+v", tomorrowProgress.MemberStats)
	}
	statsAfter, err := repository.NewUserRepository(f.db).GetUserTotalStats(f.users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if *statsAfter != *statsBefore {
		t.Fatalf("plan revision changed completed-work statistics: before=%+v after=%+v", statsBefore, statsAfter)
	}
	if statsAfter.TotalExercises != 1 || statsAfter.TotalSets != 3 || statsAfter.TotalDays != 1 {
		t.Fatalf("one completed 3-set exercise should reconcile to 1 exercise, 3 sets, 1 day: %+v", statsAfter)
	}
	if n := count(t, f.db, &model.PlanRevision{}, "goal_id = ?", goal.ID); n != 1 {
		t.Fatalf("initial actions are revision 0; repeated pending edits should create one future revision, found %d", n)
	}
	var historicalExercise model.GoalExercise
	if err := f.db.Unscoped().First(&historicalExercise, oldExercise.ID).Error; err != nil {
		t.Fatalf("baseline exercise row was not preserved: %v", err)
	}
	if historicalExercise.RevisionID != 0 {
		t.Fatalf("legacy baseline exercise revision should remain zero, got %d", historicalExercise.RevisionID)
	}
	var historicalCompletion model.GoalCompletion
	if err := f.db.Where("goal_exercise_id = ? AND date = ?", oldExercise.ID, today).First(&historicalCompletion).Error; err != nil {
		t.Fatalf("completion no longer references baseline exercise: %v", err)
	}
	if historicalCompletion.CompletedSets != 3 {
		t.Fatalf("historical completion changed after pending edits: %+v", historicalCompletion)
	}
	if err := f.goalSvc.CompleteExercise(goal.ID, f.users[0].ID, CompleteExerciseInput{
		GoalExerciseID: oldExercise.ID, CompletedSets: 3, Date: tomorrow,
	}); err == nil {
		t.Fatal("old action accepted after its revision expired")
	}
}

func TestPlanProgressRejectsExplicitNonTodayCompletionDate(t *testing.T) {
	f := newFixture(t)
	goal := createPlan(t, f, f.users[0].ID, "shared", 0, nil, []GoalExerciseInput{{
		ExerciseID: f.goal.Exercises[0].ExerciseID, Sets: 3, Reps: 10,
	}})
	yesterday := clock.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if err := f.goalSvc.CompleteExercise(goal.ID, f.users[0].ID, CompleteExerciseInput{
		GoalExerciseID: goal.Exercises[0].ID, CompletedSets: 1, Date: yesterday,
	}); err == nil {
		t.Fatal("completion for a non-today date was accepted")
	}
	if n := count(t, f.db, &model.GoalCompletion{}, "goal_exercise_id = ? AND date = ?", goal.Exercises[0].ID, yesterday); n != 0 {
		t.Fatalf("rejected date left %d completion rows", n)
	}
}

func TestWeekStatsHistoricalCountAndCurrentParticipatingTargets(t *testing.T) {
	f := newFixture(t)
	userID := f.users[1].ID
	now := clock.Now()
	today := now.Format("2006-01-02")
	pastDate := now.AddDate(0, 0, -1)
	past := pastDate.Format("2006-01-02")
	pastWeekOffset := 0
	if now.Weekday() == time.Monday {
		pastWeekOffset = -1
	}

	goalMember, err := repository.NewGoalRepository(f.db).GetGoalMember(f.goal.ID, userID)
	if err != nil {
		t.Fatal(err)
	}
	legacyCompletion := model.GoalCompletion{
		GoalMemberID:   goalMember.ID,
		GoalExerciseID: f.goal.Exercises[0].ID,
		Date:           time.Date(pastDate.Year(), pastDate.Month(), pastDate.Day(), 0, 0, 0, 0, clock.Zone),
		CompletedSets:  2,
		Status:         "pending",
	}
	if err := f.db.Create(&legacyCompletion).Error; err != nil {
		t.Fatal(err)
	}

	pastBefore, err := f.userSvc.GetWeekStats(userID, pastWeekOffset)
	if err != nil {
		t.Fatal(err)
	}
	pastBeforeStat := weekStatForDate(t, pastBefore, past)
	if pastBeforeStat.TargetKnown || pastBeforeStat.TotalSets != 0 || pastBeforeStat.Count != 2 {
		t.Fatalf("historical chart should retain completed sets without today's target: %+v", pastBeforeStat)
	}

	if err := f.goalSvc.CompleteExercise(f.goal.ID, userID, CompleteExerciseInput{
		GoalExerciseID: f.goal.Exercises[0].ID,
		CompletedSets:  1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.goalSvc.UpdateGoal(f.goal.ID, f.users[0].ID, UpdateGoalInput{Exercises: []GoalExerciseInput{{
		ExerciseID: f.goal.Exercises[0].ExerciseID,
		Sets:       4,
		Reps:       12,
	}}}); err != nil {
		t.Fatalf("schedule tomorrow's plan revision: %v", err)
	}
	todayStats, err := f.userSvc.GetWeekStats(userID, 0)
	if err != nil {
		t.Fatal(err)
	}
	todayStat := weekStatForDate(t, todayStats, today)
	if !todayStat.TargetKnown || todayStat.TotalSets != 5 || todayStat.Count != 1 {
		t.Fatalf("today should use current participating targets and actual completed sets: %+v", todayStat)
	}

	assertPastUnchanged := func(stage string) {
		t.Helper()
		pastResult, err := f.userSvc.GetWeekStats(userID, pastWeekOffset)
		if err != nil {
			t.Fatal(err)
		}
		pastStat := weekStatForDate(t, pastResult, past)
		if pastStat.TargetKnown || pastStat.TotalSets != 0 || pastStat.Count != pastBeforeStat.Count {
			t.Fatalf("%s changed historical actual-count chart: before=%+v after=%+v", stage, pastBeforeStat, pastStat)
		}
	}
	assertPastUnchanged("tomorrow-effective edit")

	if _, err := f.goalSvc.UpdateGoal(f.goal.ID, f.users[0].ID, UpdateGoalInput{Exercises: []GoalExerciseInput{{
		ExerciseID: f.goal.Exercises[0].ExerciseID,
		Sets:       6,
		Reps:       12,
	}}}); err != nil {
		t.Fatalf("repeat edit of tomorrow's pending revision: %v", err)
	}
	assertPastUnchanged("repeated pending edit")
	todayStats, err = f.userSvc.GetWeekStats(userID, 0)
	if err != nil {
		t.Fatal(err)
	}
	todayStat = weekStatForDate(t, todayStats, today)
	if !todayStat.TargetKnown || todayStat.TotalSets != 5 || todayStat.Count != 1 {
		t.Fatalf("pending repeated edit changed today's current target or actual count: %+v", todayStat)
	}
}

func TestRemovedPlanMemberKeepsHistoryAndRejoinsOnce(t *testing.T) {
	f := newFixture(t)
	goalID := f.goal.ID
	adminID, memberID := f.users[0].ID, f.users[1].ID
	today := clock.Now().Format("2006-01-02")
	goalExerciseID := f.goal.Exercises[0].ID
	if err := f.goalSvc.CompleteExercise(goalID, memberID, CompleteExerciseInput{
		GoalExerciseID: goalExerciseID,
		CompletedSets:  3,
		Date:           today,
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.familySvc.RemoveMember(f.family.ID, adminID, memberID); err != nil {
		t.Fatal(err)
	}

	currentGoal, err := f.goalSvc.GetGoal(goalID)
	if err != nil {
		t.Fatal(err)
	}
	if len(currentGoal.Members) != 1 || currentGoal.Members[0].UserID != adminID {
		t.Fatalf("removed family member remained in current goal members: %+v", currentGoal.Members)
	}
	progress, err := f.goalSvc.GetGoalProgress(goalID, today)
	if err != nil {
		t.Fatal(err)
	}
	findMember := func(progress *GoalProgress) (MemberProgress, int) {
		t.Helper()
		var match MemberProgress
		matches := 0
		for _, member := range progress.MemberStats {
			if member.UserID == memberID {
				match = member
				matches++
			}
		}
		return match, matches
	}
	departed, matches := findMember(progress)
	if matches != 1 || departed.IsCurrentMember || departed.CompletedSets != 3 {
		t.Fatalf("removed member history should remain once and be marked non-current: matches=%d member=%+v", matches, departed)
	}

	if _, err := f.familySvc.JoinFamily(f.family.InviteCode, memberID); err != nil {
		t.Fatalf("member could not rejoin family: %v", err)
	}
	currentGoal, err = f.goalSvc.GetGoal(goalID)
	if err != nil {
		t.Fatal(err)
	}
	if len(currentGoal.Members) != 2 {
		t.Fatalf("rejoined member did not return to current goal members exactly once: %+v", currentGoal.Members)
	}
	progress, err = f.goalSvc.GetGoalProgress(goalID, today)
	if err != nil {
		t.Fatal(err)
	}
	rejoined, matches := findMember(progress)
	if matches != 1 || !rejoined.IsCurrentMember || rejoined.CompletedSets != 3 {
		t.Fatalf("rejoin duplicated or lost historical completion: matches=%d member=%+v", matches, rejoined)
	}
	if n := count(t, f.db, &model.GoalCompletion{}, "goal_exercise_id = ? AND date = ?", goalExerciseID, today); n != 1 {
		t.Fatalf("rejoining duplicated the member's historical completion: got %d rows", n)
	}
}

func TestLegacyPlanColumnsReceiveCompatibleDefaults(t *testing.T) {
	f := newFixture(t)
	suffix := time.Now().UnixNano()
	oldExercisesTable := fmt.Sprintf("legacy_goal_exercises_%d", suffix)
	oldGoalsTable := fmt.Sprintf("legacy_daily_goals_%d", suffix)
	t.Cleanup(func() {
		_ = f.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", oldExercisesTable)).Error
		_ = f.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", oldGoalsTable)).Error
	})

	if err := f.db.Exec(fmt.Sprintf("CREATE TABLE `%s` (`id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, `goal_id` BIGINT UNSIGNED NOT NULL)", oldExercisesTable)).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(fmt.Sprintf("INSERT INTO `%s` (`goal_id`) VALUES (?)", oldExercisesTable), f.goal.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Table(oldExercisesTable).Migrator().AddColumn(&model.GoalExercise{}, "RevisionID"); err != nil {
		t.Fatalf("add revision_id to legacy exercise table: %v", err)
	}
	var exerciseRow struct {
		RevisionID sql.NullInt64 `gorm:"column:revision_id"`
	}
	if err := f.db.Table(oldExercisesTable).Select("revision_id").First(&exerciseRow).Error; err != nil {
		t.Fatal(err)
	}
	if !exerciseRow.RevisionID.Valid || exerciseRow.RevisionID.Int64 != 0 {
		t.Fatalf("legacy exercise revision_id should default to non-null zero: %+v", exerciseRow.RevisionID)
	}

	if err := f.db.Exec(fmt.Sprintf("CREATE TABLE `%s` (`id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY)", oldGoalsTable)).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(fmt.Sprintf("INSERT INTO `%s` (`id`) VALUES (?)", oldGoalsTable), 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Table(oldGoalsTable).Migrator().AddColumn(&model.DailyGoal{}, "PlanType"); err != nil {
		t.Fatalf("add plan_type to legacy goal table: %v", err)
	}
	if err := f.db.Table(oldGoalsTable).Migrator().AddColumn(&model.DailyGoal{}, "ExecutorID"); err != nil {
		t.Fatalf("add executor_id to legacy goal table: %v", err)
	}
	var goalRow struct {
		PlanType   sql.NullString `gorm:"column:plan_type"`
		ExecutorID sql.NullInt64  `gorm:"column:executor_id"`
	}
	if err := f.db.Table(oldGoalsTable).Select("plan_type, executor_id").First(&goalRow).Error; err != nil {
		t.Fatal(err)
	}
	if !goalRow.PlanType.Valid || goalRow.PlanType.String != "shared" || !goalRow.ExecutorID.Valid || goalRow.ExecutorID.Int64 != 0 {
		t.Fatalf("legacy goal defaults should be shared/zero: plan_type=%+v executor_id=%+v", goalRow.PlanType, goalRow.ExecutorID)
	}
}
