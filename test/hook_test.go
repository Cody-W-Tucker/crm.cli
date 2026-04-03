package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Hooks ---

func TestHook_PostContactAdd(t *testing.T) {
	tc := newTestContext(t)

	// Create a hook script that writes the received JSON to a file.
	hookOutput := filepath.Join(tc.dir, "hook-output.json")
	hookScript := filepath.Join(tc.dir, "hook.sh")
	os.WriteFile(hookScript, []byte("#!/bin/sh\ncat > "+hookOutput+"\n"), 0755)

	// Create config with the hook.
	configPath := filepath.Join(tc.dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[hooks]
post-contact-add = "`+hookScript+`"
`), 0644)

	tc.runOK("--config", configPath, "contact", "add", "--name", "Jane", "--email", "jane@acme.com")

	// Verify the hook was called and received data.
	data, err := os.ReadFile(hookOutput)
	if err != nil {
		t.Fatalf("hook output file not created: %v", err)
	}
	assertContains(t, string(data), "Jane")
	assertContains(t, string(data), "jane@acme.com")
}

func TestHook_PreContactRm_Abort(t *testing.T) {
	tc := newTestContext(t)

	// Create a pre-hook that always fails (exit 1) to abort the operation.
	hookScript := filepath.Join(tc.dir, "abort.sh")
	os.WriteFile(hookScript, []byte("#!/bin/sh\nexit 1\n"), 0755)

	configPath := filepath.Join(tc.dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[hooks]
pre-contact-rm = "`+hookScript+`"
`), 0644)

	id := strings.TrimSpace(tc.runOK("--config", configPath, "contact", "add", "--name", "Jane"))

	// Attempting to delete should fail because the pre-hook aborts.
	_, _ = tc.runFail("--config", configPath, "contact", "rm", id, "--force")

	// Contact should still exist.
	tc.runOK("--config", configPath, "contact", "show", id)
}

func TestHook_PostDealStageChange(t *testing.T) {
	tc := newTestContext(t)

	hookOutput := filepath.Join(tc.dir, "stage-change.json")
	hookScript := filepath.Join(tc.dir, "stage-hook.sh")
	os.WriteFile(hookScript, []byte("#!/bin/sh\ncat > "+hookOutput+"\n"), 0755)

	configPath := filepath.Join(tc.dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[hooks]
post-deal-stage-change = "`+hookScript+`"
`), 0644)

	id := strings.TrimSpace(tc.runOK("--config", configPath, "deal", "add", "--title", "Hook Deal", "--stage", "lead"))
	tc.runOK("--config", configPath, "deal", "move", id, "--stage", "qualified")

	data, err := os.ReadFile(hookOutput)
	if err != nil {
		t.Fatalf("hook output file not created: %v", err)
	}
	// Should contain deal data with the new stage.
	assertContains(t, string(data), "qualified")
}

func TestHook_NoConfig(t *testing.T) {
	tc := newTestContext(t)

	// Without any hooks configured, operations should work fine.
	tc.runOK("contact", "add", "--name", "Jane")
}
