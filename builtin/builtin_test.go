package builtin

import (
	"testing"

	"github.com/mudler/nib/types"
)

func TestSkillsReturnsAboutNib(t *testing.T) {
	skills := Skills()
	if len(skills) == 0 {
		t.Fatal("Skills() returned empty slice")
	}
	var found bool
	for _, s := range skills {
		if s.Name == "about-nib" {
			found = true
			if s.Description == "" {
				t.Error("about-nib skill has empty description")
			}
			if s.Instructions == "" {
				t.Error("about-nib skill has empty instructions")
			}
		}
	}
	if !found {
		t.Error("Skills() missing about-nib skill")
	}
}

func TestApplyAddsBuiltinWhenAbsent(t *testing.T) {
	cfg := types.Config{}
	Apply(&cfg)
	var found bool
	for _, s := range cfg.Skills {
		if s.Name == "about-nib" {
			found = true
		}
	}
	if !found {
		t.Error("Apply() did not add about-nib to empty cfg.Skills")
	}
}

func TestApplySkipsBuiltinWhenUserDefinedSameName(t *testing.T) {
	userSkill := types.Skill{
		Name:         "about-nib",
		Description:  "user override",
		Instructions: "custom instructions",
	}
	cfg := types.Config{Skills: []types.Skill{userSkill}}
	Apply(&cfg)
	// Should have exactly one about-nib skill, and it should be the user's.
	var count, userCount int
	for _, s := range cfg.Skills {
		if s.Name == "about-nib" {
			count++
			if s.Instructions == "custom instructions" {
				userCount++
			}
		}
	}
	if count != 1 {
		t.Errorf("expected 1 about-nib skill, got %d", count)
	}
	if userCount != 1 {
		t.Error("user's about-nib skill was not preserved")
	}
}

func TestApplyPreservesUserSkills(t *testing.T) {
	cfg := types.Config{Skills: []types.Skill{
		{Name: "my-skill", Description: "d", Instructions: "i"},
	}}
	Apply(&cfg)
	var foundMy, foundAbout bool
	for _, s := range cfg.Skills {
		if s.Name == "my-skill" {
			foundMy = true
		}
		if s.Name == "about-nib" {
			foundAbout = true
		}
	}
	if !foundMy {
		t.Error("user skill was lost after Apply")
	}
	if !foundAbout {
		t.Error("built-in was not added alongside user skill")
	}
}
