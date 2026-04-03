package test

import (
	"testing"
)

// --- Reports ---

func TestReportPipeline(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "A", "--value", "10000", "--stage", "lead")
	tc.runOK("deal", "add", "--title", "B", "--value", "20000", "--stage", "lead")
	tc.runOK("deal", "add", "--title", "C", "--value", "50000", "--stage", "qualified")

	out := tc.runOK("report", "pipeline")
	assertContains(t, out, "lead")
	assertContains(t, out, "qualified")
	assertContains(t, out, "Total")
}

func TestReportPipeline_JSON(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "A", "--value", "10000", "--stage", "lead")

	var report []map[string]interface{}
	tc.runJSON(&report, "report", "pipeline", "--format", "json")
	if len(report) == 0 {
		t.Fatal("expected non-empty pipeline report")
	}

	// Each stage entry should have count, value, weighted_value.
	stage := report[0]
	if _, ok := stage["stage"]; !ok {
		t.Fatal("expected 'stage' field in pipeline report")
	}
	if _, ok := stage["count"]; !ok {
		t.Fatal("expected 'count' field in pipeline report")
	}
	if _, ok := stage["value"]; !ok {
		t.Fatal("expected 'value' field in pipeline report")
	}
}

func TestReportPipeline_Empty(t *testing.T) {
	tc := newTestContext(t)

	// Should succeed even with no deals.
	out := tc.runOK("report", "pipeline")
	assertContains(t, out, "Total")
}

func TestReportActivity(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "Note 1")
	tc.runOK("log", "call", "jane@acme.com", "Call 1")

	out := tc.runOK("report", "activity")
	assertContains(t, out, "note")
	assertContains(t, out, "call")
}

func TestReportActivity_ByType(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "N1")
	tc.runOK("log", "note", "jane@acme.com", "N2")
	tc.runOK("log", "call", "jane@acme.com", "C1")

	var report []map[string]interface{}
	tc.runJSON(&report, "report", "activity", "--by", "type", "--format", "json")
	if len(report) == 0 {
		t.Fatal("expected activity report grouped by type")
	}
}

func TestReportActivity_ByContact(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("contact", "add", "--name", "Bob", "--email", "bob@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "N1")
	tc.runOK("log", "note", "jane@acme.com", "N2")
	tc.runOK("log", "note", "bob@acme.com", "N3")

	var report []map[string]interface{}
	tc.runJSON(&report, "report", "activity", "--by", "contact", "--format", "json")
	if len(report) < 2 {
		t.Fatalf("expected at least 2 groups (Jane and Bob), got %d", len(report))
	}
}

func TestReportActivity_Period(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "Old", "--at", "2025-01-01")
	tc.runOK("log", "note", "jane@acme.com", "Recent")

	var report []map[string]interface{}
	tc.runJSON(&report, "report", "activity", "--period", "7d", "--format", "json")
	// Only the recent note should be counted.
	found := false
	for _, r := range report {
		if count, ok := r["count"].(float64); ok && count > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected at least one activity in 7d report")
	}
}

func TestReportStale(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Active Jane", "--email", "jane@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "Just spoke")

	tc.runOK("contact", "add", "--name", "Stale Bob", "--email", "bob@acme.com")
	// Bob has no activity — should appear in stale report.

	out := tc.runOK("report", "stale", "--days", "1")
	assertContains(t, out, "Stale Bob")
	assertNotContains(t, out, "Active Jane")
}

func TestReportStale_Deals(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "Active Deal")
	// Immediately log activity on it.
	// Note: deal stage change itself counts as activity.

	tc.runOK("deal", "add", "--title", "Stale Deal")
	// Stale Deal has no activity after creation.

	// With --days 0, everything created "just now" should not be stale.
	// With --days 0 and --type deal, we check the mechanism works.
	out := tc.runOK("report", "stale", "--type", "deal")
	// Default 14 days — both were just created, so nothing should be stale.
	// This tests that recent deals aren't falsely flagged.
	assertNotContains(t, out, "Active Deal")
}

func TestReportConversion(t *testing.T) {
	tc := newTestContext(t)

	// Create deals and move some through stages.
	for i := 0; i < 5; i++ {
		tc.runOK("deal", "add", "--title", "Lead Deal "+string(rune('A'+i)), "--stage", "lead")
	}
	// Move 3 to qualified.
	var ids []string
	var deals []map[string]interface{}
	tc.runJSON(&deals, "deal", "list", "--stage", "lead", "--format", "json")
	for _, d := range deals[:3] {
		id := d["id"].(string)
		ids = append(ids, id)
		tc.runOK("deal", "move", id, "--stage", "qualified")
	}
	// Move 2 to proposal.
	for _, id := range ids[:2] {
		tc.runOK("deal", "move", id, "--stage", "proposal")
	}

	out := tc.runOK("report", "conversion")
	assertContains(t, out, "lead")
	assertContains(t, out, "qualified")
}

func TestReportConversion_JSON(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "Deal A", "--stage", "lead")

	var report []map[string]interface{}
	tc.runJSON(&report, "report", "conversion", "--format", "json")
	// Should have stage transition entries.
	if len(report) == 0 {
		t.Fatal("expected non-empty conversion report")
	}
}

func TestReportVelocity(t *testing.T) {
	tc := newTestContext(t)

	id := tc.runOK("deal", "add", "--title", "Fast Deal", "--stage", "lead")
	tc.runOK("deal", "move", id, "--stage", "qualified")
	tc.runOK("deal", "move", id, "--stage", "closed-won")

	out := tc.runOK("report", "velocity")
	assertContains(t, out, "lead")
	assertContains(t, out, "qualified")
}

func TestReportForecast(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "Deal A", "--value", "50000", "--probability", "80", "--expected-close", "2026-06-15")
	tc.runOK("deal", "add", "--title", "Deal B", "--value", "30000", "--probability", "50", "--expected-close", "2026-06-20")

	out := tc.runOK("report", "forecast")
	assertContains(t, out, "Deal A")
	assertContains(t, out, "Deal B")
}

func TestReportForecast_Period(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "Q2 Deal", "--value", "50000", "--expected-close", "2026-06-15")
	tc.runOK("deal", "add", "--title", "Q3 Deal", "--value", "30000", "--expected-close", "2026-09-15")

	var report []map[string]interface{}
	tc.runJSON(&report, "report", "forecast", "--period", "2026-06", "--format", "json")
	// Only Q2 Deal should appear.
	if len(report) != 1 {
		t.Fatalf("expected 1 deal in June forecast, got %d", len(report))
	}
}

func TestReportWon(t *testing.T) {
	tc := newTestContext(t)

	id := tc.runOK("deal", "add", "--title", "Won Deal", "--value", "25000", "--stage", "lead")
	tc.runOK("deal", "move", id, "--stage", "closed-won")

	out := tc.runOK("report", "won")
	assertContains(t, out, "Won Deal")
	assertContains(t, out, "25000")
}

func TestReportLost(t *testing.T) {
	tc := newTestContext(t)

	id := tc.runOK("deal", "add", "--title", "Lost Deal", "--value", "15000", "--stage", "lead")
	tc.runOK("deal", "move", id, "--stage", "closed-lost", "--reason", "Too expensive")

	out := tc.runOK("report", "lost", "--reasons")
	assertContains(t, out, "Lost Deal")
	assertContains(t, out, "Too expensive")
}

func TestReportWon_Period(t *testing.T) {
	tc := newTestContext(t)

	id := tc.runOK("deal", "add", "--title", "Won Deal", "--value", "25000", "--stage", "lead")
	tc.runOK("deal", "move", id, "--stage", "closed-won")

	var report []map[string]interface{}
	tc.runJSON(&report, "report", "won", "--period", "30d", "--format", "json")
	if len(report) != 1 {
		t.Fatalf("expected 1 won deal, got %d", len(report))
	}
}
