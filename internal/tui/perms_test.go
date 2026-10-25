package tui_test

import (
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/preset"
)

// openPerms opens the permission profiles of a preset.
func (h *harness) openPerms(name string, agents int) {
	h.t.Helper()
	h.open(name)
	for i := 0; i < agents+2; i++ { // the agents, add agent, model, effort
		h.press("down")
	}
	h.press("down", "enter") // Permissions
}

func TestPermissionProfilesScreenAt80x24(t *testing.T) {
	h := newHarness(t)

	h.openPerms("solo-indie", 3)

	h.contains("Permissions", "Careful", "Balanced", "Autopilot", "Custom…", "WHAT IT SETS")
	h.golden("perms")
}

func TestTheCurrentProfileIsMarkedAndPreselected(t *testing.T) {
	h := newHarness(t)
	b := h.stored("solo-indie")
	b.Preset.Settings.Permissions = &preset.Permissions{DefaultMode: "acceptEdits", Ask: []string{"Bash(git push *)"}}
	if err := h.ws.Save(*b); err != nil {
		t.Fatal(err)
	}

	h.openPerms("solo-indie", 3)

	h.contains("▌ Balanced", "(current)")
}

func TestPickingAProfileSavesItAndGoesBack(t *testing.T) {
	h := newHarness(t)
	h.openPerms("bug-hunt", 2)

	h.press("down", "enter") // Not set -> Careful

	p := h.stored("bug-hunt").Preset.Settings.Permissions
	if p == nil || p.DefaultMode != "default" || len(p.Deny) != 2 || p.Deny[0] != "Read(./.env)" {
		t.Fatalf("permissions = %+v", p)
	}
	h.contains("AGENTS", "Permissions", "default · 0 allow · 0 ask · 2 deny")

	h.press("enter")         // the cursor is still on the Permissions row
	h.press("down", "enter") // Careful -> Balanced
	if p := h.stored("bug-hunt").Preset.Settings.Permissions; p.DefaultMode != "acceptEdits" || len(p.Ask) != 1 {
		t.Fatalf("permissions = %+v", p)
	}
}

func TestNotSetClearsThePermissions(t *testing.T) {
	h := newHarness(t)
	h.openPerms("solo-indie", 3)

	h.press("up", "up", "up", "enter") // wherever the cursor is, go to Not set

	if p := h.stored("solo-indie").Preset.Settings.Permissions; p != nil {
		t.Fatalf("permissions = %+v, want them cleared", p)
	}
}

func TestCustomPermissionsAreEditedRuleByRule(t *testing.T) {
	h := newHarness(t)
	h.openPerms("bug-hunt", 2)

	h.press("down", "down", "down", "down", "enter") // Custom…
	h.contains("Custom permissions", "ALLOW", "ASK", "DENY", "Default mode", "add rule")
	h.golden("perms-custom")

	h.press("down", "enter") // ALLOW: add rule
	h.contains("allow rule")
	h.typeText("Bash(npm run *)")
	h.press("enter")
	h.contains("Bash(npm run *)")
	p := h.stored("bug-hunt").Preset.Settings.Permissions
	if len(p.Allow) != 1 || p.Allow[0] != "Bash(npm run *)" {
		t.Fatalf("allow = %v", p.Allow)
	}

	h.press("enter") // edit the rule
	h.typeText("!")
	h.press("enter")
	if got := h.stored("bug-hunt").Preset.Settings.Permissions.Allow[0]; got != "Bash(npm run *)!" {
		t.Errorf("edited rule = %q", got)
	}

	h.press("x")
	if got := h.stored("bug-hunt").Preset.Settings.Permissions.Allow; len(got) != 0 {
		t.Errorf("allow = %v, want the rule deleted", got)
	}
}

func TestEmptyRulesAreRefused(t *testing.T) {
	h := newHarness(t)
	h.openPerms("bug-hunt", 2)
	h.press("down", "down", "down", "down", "enter", "down", "enter")

	h.press("enter")

	h.contains("a rule cannot be empty")
}

func TestBypassPermissionsNeedsAnExplicitConfirmation(t *testing.T) {
	h := newHarness(t)
	h.openPerms("bug-hunt", 2)
	h.press("down", "down", "down", "down", "enter") // Custom…

	h.press("enter") // Default mode
	h.contains("Default mode", "bypassPermissions", "dangerous")
	for i := 0; i < 4; i++ { // default, acceptEdits, auto, dontAsk, bypassPermissions
		h.press("down")
	}
	h.press("enter")
	h.contains("Turn off permission prompts?", "without asking")
	h.press("enter") // the default button is Cancel
	if p := h.stored("bug-hunt").Preset.Settings.Permissions; p != nil && p.DefaultMode == "bypassPermissions" {
		t.Fatal("bypassPermissions was set without confirmation")
	}

	h.press("enter")
	for i := 0; i < 4; i++ {
		h.press("down")
	}
	h.press("enter", "y")
	if p := h.stored("bug-hunt").Preset.Settings.Permissions; p.DefaultMode != "bypassPermissions" {
		t.Fatalf("mode = %q after confirming", p.DefaultMode)
	}
}

func TestLeavingTheCustomScreenWithoutEditingChangesNothing(t *testing.T) {
	h := newHarness(t)
	h.openPerms("bug-hunt", 2)
	h.press("down", "down", "down", "down", "enter")                    // Custom…
	h.press("esc")                                                      // nothing edited
	h.press("esc")                                                      // back to the preset
	h.press("up", "up", "up", "up", "up", "up", "up", "enter", "enter") // change the model

	if p := h.stored("bug-hunt").Preset.Settings.Permissions; p != nil {
		t.Fatalf("permissions = %+v: opening Custom must not set anything", p)
	}
}
