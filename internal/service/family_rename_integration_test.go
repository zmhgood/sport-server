package service

import (
	"elderly-fitness/internal/model"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestFamilyRenamePreservesRelatedData(t *testing.T) {
	f := newFixture(t)
	f.complete(t, 0, 3)
	other, err := f.familySvc.CreateFamily("Other family", f.users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		var family model.Family
		if err := f.db.First(&family, f.family.ID).Error; err != nil {
			t.Fatal(err)
		}
		var members []model.FamilyMember
		var goals []model.DailyGoal
		var goalMembers []model.GoalMember
		var exercises []model.GoalExercise
		var completions []model.GoalCompletion
		for _, q := range []struct {
			value interface{}
			where string
			args  []interface{}
		}{
			{&members, "family_id=?", []interface{}{f.family.ID}},
			{&goals, "family_id=?", []interface{}{f.family.ID}},
			{&goalMembers, "goal_id IN (?,?)", []interface{}{f.goal.ID, f.other.ID}},
			{&exercises, "goal_id IN (?,?)", []interface{}{f.goal.ID, f.other.ID}},
			{&completions, "goal_member_id IN (SELECT id FROM goal_members WHERE goal_id IN (?,?))", []interface{}{f.goal.ID, f.other.ID}},
		} {
			if err := f.db.Unscoped().Where(q.where, q.args...).Order("id").Find(q.value).Error; err != nil {
				t.Fatal(err)
			}
		}
		value := []interface{}{family.ID, family.CreatorID, family.InviteCode, family.CreatedAt, family.DeletedAt, members, goals, goalMembers, exercises, completions}
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	before := snapshot()
	for _, name := range []string{" 老张家 ", "家" + strings.Repeat("庭", 63), strings.Repeat("😀", 64), "x', creator_id=999 --", other.Name, other.Name} {
		got, err := f.familySvc.RenameFamily(f.family.ID, f.users[0].ID, name)
		if err != nil || got != strings.TrimSpace(name) {
			t.Fatalf("rename: %q %v", got, err)
		}
		persisted, err := f.familySvc.GetFamily(f.family.ID)
		if err != nil || persisted.Name != got {
			t.Fatal("name did not persist")
		}
		if snapshot() != before {
			t.Fatal("rename altered related data")
		}
		untouched, err := f.familySvc.GetFamily(other.ID)
		if err != nil || untouched.Name != other.Name {
			t.Fatal("another family changed")
		}
	}
	families, err := f.familySvc.GetFamilyByUserID(f.users[1].ID)
	if err != nil || len(families) != 1 || families[0].Name != other.Name {
		t.Fatal("member re-entry sees stale family name")
	}
}

func TestFamilyRenameRequiresCurrentAdmin(t *testing.T) {
	f := newFixture(t)
	expect := func(user uint, want error) {
		t.Helper()
		_, err := f.familySvc.RenameFamily(f.family.ID, user, "Forbidden change")
		if !errors.Is(err, want) {
			t.Fatalf("got %v want %v", err, want)
		}
	}
	expect(f.users[1].ID, ErrFamilyRenameForbidden)
	expect(f.users[2].ID, ErrFamilyRenameForbidden)
	expect(0, ErrFamilyRenameForbidden)
	if err := f.familySvc.TransferAdmin(f.family.ID, f.users[0].ID, f.users[1].ID); err != nil {
		t.Fatal(err)
	}
	expect(f.users[0].ID, ErrFamilyRenameForbidden)
	if _, err := f.familySvc.RenameFamily(f.family.ID, f.users[1].ID, "New admin family"); err != nil {
		t.Fatal(err)
	}
	if err := f.familySvc.RemoveMember(f.family.ID, f.users[1].ID, f.users[0].ID); err != nil {
		t.Fatal(err)
	}
	expect(f.users[0].ID, ErrFamilyRenameForbidden)
	if _, err := f.familySvc.JoinFamily(f.family.InviteCode, f.users[0].ID); err != nil {
		t.Fatal(err)
	}
	expect(f.users[0].ID, ErrFamilyRenameForbidden)
	if err := f.db.Delete(&model.Family{}, f.family.ID).Error; err != nil {
		t.Fatal(err)
	}
	expect(f.users[1].ID, ErrFamilyRenameNotFound)
	if _, err := f.familySvc.RenameFamily(4294967295, f.users[1].ID, "Missing"); !errors.Is(err, ErrFamilyRenameNotFound) {
		t.Fatal("missing family accepted")
	}
}

func TestFamilyRenameRejectsInvalidNamesWithoutWrites(t *testing.T) {
	f := newFixture(t)
	for _, name := range []string{"", "  ", strings.Repeat("家", 65), strings.Repeat("😀", 65), "家\n庭", "家\x00庭", "家\u0085庭"} {
		if _, err := f.familySvc.RenameFamily(f.family.ID, f.users[0].ID, name); !errors.Is(err, ErrFamilyNameInvalid) {
			t.Fatalf("invalid name accepted: %q %v", name, err)
		}
	}
	got, err := f.familySvc.GetFamily(f.family.ID)
	if err != nil || got.Name != f.family.Name {
		t.Fatal("invalid input changed family")
	}
}
