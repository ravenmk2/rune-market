package hub

import (
	"context"
	"fmt"

	"github.com/ravenmk2/rune-market/internal/store"
)

// Admin operations on artifacts (§8.5): official flag and review approval.
// Callers are already admin-gated at the route level.

// SetSkillOfficial toggles the official flag of a skill.
func (s *Skills) SetOfficial(ctx context.Context, skillID string, official bool) (*store.Skill, error) {
	sk, err := s.getSkill(ctx, skillID)
	if err != nil {
		return nil, err
	}
	if err := store.NewSkillStore(s.db).UpdateOfficial(ctx, skillID, official, store.Now()); err != nil {
		return nil, err
	}
	sk.Official = official
	return sk, nil
}

// ApproveSkill moves a pending skill to published (artifact_review flow).
func (s *Skills) ApproveSkill(ctx context.Context, skillID string) (*store.Skill, error) {
	sk, err := s.getSkill(ctx, skillID)
	if err != nil {
		return nil, err
	}
	if sk.Status != store.SkillStatusPending {
		return nil, fmt.Errorf("%w: skill is not pending", ErrInvalidInput)
	}
	if err := store.NewSkillStore(s.db).UpdateStatus(ctx, skillID, store.SkillStatusPublished, store.Now()); err != nil {
		return nil, err
	}
	sk.Status = store.SkillStatusPublished
	return sk, nil
}

// SetDesignOfficial toggles the official flag of a designmd.
func (s *Designs) SetOfficial(ctx context.Context, designID string, official bool) (*store.Designmd, error) {
	d, err := s.getDesign(ctx, designID)
	if err != nil {
		return nil, err
	}
	if err := store.NewDesignmdStore(s.db).UpdateOfficial(ctx, designID, official, store.Now()); err != nil {
		return nil, err
	}
	d.Official = official
	return d, nil
}

// ApproveDesign moves a pending designmd to published.
func (s *Designs) ApproveDesign(ctx context.Context, designID string) (*store.Designmd, error) {
	d, err := s.getDesign(ctx, designID)
	if err != nil {
		return nil, err
	}
	if d.Status != store.DesignStatusPending {
		return nil, fmt.Errorf("%w: design is not pending", ErrInvalidInput)
	}
	if err := store.NewDesignmdStore(s.db).UpdateStatus(ctx, designID, store.DesignStatusPublished, store.Now()); err != nil {
		return nil, err
	}
	d.Status = store.DesignStatusPublished
	return d, nil
}
