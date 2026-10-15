package model

import "time"

// Revisions keep the action requirements referenced by historical completions immutable.
// Revision 0 denotes the existing goal actions before versioned editing was introduced.
type PlanRevision struct {
	ID            uint      `gorm:"primaryKey"`
	GoalID        uint      `gorm:"uniqueIndex:idx_plan_revision_day"`
	EffectiveDate time.Time `gorm:"type:date;uniqueIndex:idx_plan_revision_day"`
	CreatedAt     time.Time
}
