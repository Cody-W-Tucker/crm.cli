package test

import (
	"strings"
	"testing"
)

// --- Deal CRUD & Pipeline ---

func TestDealAdd_Basic(t *testing.T) {
	tc := newTestContext(t)

	out := tc.runOK("deal", "add", "--title", "Acme Enterprise")
	id := strings.TrimSpace(out)
	if !strings.HasPrefix(id, "dl_") {
		t.Fatalf("expected deal ID starting with dl_, got: %s", id)
	}
}

func TestDealAdd_Full(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("company", "add", "--name", "Acme", "--domain", "acme.com")

	id := strings.TrimSpace(tc.runOK("deal", "add",
		"--title", "Acme Enterprise",
		"--value", "50000",
		"--stage", "qualified",
		"--contact", "jane@acme.com",
		"--company", "acme.com",
		"--expected-close", "2026-06-01",
		"--probability", "60",
		"--tag", "q2",
		"--set", "source=outbound",
	))

	show := tc.runOK("deal", "show", id)
	assertContains(t, show, "Acme Enterprise")
	assertContains(t, show, "50000")
	assertContains(t, show, "qualified")
	assertContains(t, show, "jane@acme.com")
	assertContains(t, show, "2026-06-01")
	assertContains(t, show, "q2")
}

func TestDealAdd_TitleRequired(t *testing.T) {
	tc := newTestContext(t)

	_, stderr := tc.runFail("deal", "add", "--value", "10000")
	assertContains(t, stderr, "title")
}

func TestDealAdd_DefaultStage(t *testing.T) {
	tc := newTestContext(t)

	// Without --stage, deal should be assigned the first configured stage ("lead").
	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "New Deal"))
	show := tc.runOK("deal", "show", id)
	assertContains(t, show, "lead")
}

func TestDealList(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "Deal A", "--value", "10000", "--stage", "lead")
	tc.runOK("deal", "add", "--title", "Deal B", "--value", "50000", "--stage", "qualified")
	tc.runOK("deal", "add", "--title", "Deal C", "--value", "20000", "--stage", "lead")

	var deals []map[string]interface{}
	tc.runJSON(&deals, "deal", "list", "--format", "json")
	if len(deals) != 3 {
		t.Fatalf("expected 3 deals, got %d", len(deals))
	}
}

func TestDealList_FilterByStage(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "Deal A", "--stage", "lead")
	tc.runOK("deal", "add", "--title", "Deal B", "--stage", "qualified")

	var deals []map[string]interface{}
	tc.runJSON(&deals, "deal", "list", "--stage", "lead", "--format", "json")
	if len(deals) != 1 {
		t.Fatalf("expected 1 deal in lead stage, got %d", len(deals))
	}
}

func TestDealList_FilterByValue(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "Small", "--value", "5000")
	tc.runOK("deal", "add", "--title", "Medium", "--value", "25000")
	tc.runOK("deal", "add", "--title", "Large", "--value", "100000")

	var deals []map[string]interface{}
	tc.runJSON(&deals, "deal", "list", "--min-value", "10000", "--max-value", "50000", "--format", "json")
	if len(deals) != 1 {
		t.Fatalf("expected 1 deal in value range, got %d", len(deals))
	}
}

func TestDealList_FilterByContact(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("deal", "add", "--title", "Jane's Deal", "--contact", "jane@acme.com")
	tc.runOK("deal", "add", "--title", "Unlinked Deal")

	var deals []map[string]interface{}
	tc.runJSON(&deals, "deal", "list", "--contact", "jane@acme.com", "--format", "json")
	if len(deals) != 1 {
		t.Fatalf("expected 1 deal linked to Jane, got %d", len(deals))
	}
}

func TestDealList_SortByValue(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "Small", "--value", "5000")
	tc.runOK("deal", "add", "--title", "Large", "--value", "100000")
	tc.runOK("deal", "add", "--title", "Medium", "--value", "25000")

	var deals []map[string]interface{}
	tc.runJSON(&deals, "deal", "list", "--sort", "value", "--format", "json")
	v0 := deals[0]["value"].(float64)
	v1 := deals[1]["value"].(float64)
	v2 := deals[2]["value"].(float64)
	if !(v0 <= v1 && v1 <= v2) {
		t.Fatalf("expected ascending value sort, got: %.0f, %.0f, %.0f", v0, v1, v2)
	}
}

func TestDealMove(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "Moving Deal", "--stage", "lead"))

	tc.runOK("deal", "move", id, "--stage", "qualified")
	show := tc.runOK("deal", "show", id)
	assertContains(t, show, "qualified")
}

func TestDealMove_InvalidStage(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "Deal"))
	_, stderr := tc.runFail("deal", "move", id, "--stage", "nonexistent")
	assertContains(t, stderr, "stage")
}

func TestDealMove_RecordsHistory(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "History Deal", "--stage", "lead"))
	tc.runOK("deal", "move", id, "--stage", "qualified")
	tc.runOK("deal", "move", id, "--stage", "proposal")

	// Show should include stage history with timestamps.
	show := tc.runOK("deal", "show", id)
	assertContains(t, show, "lead")
	assertContains(t, show, "qualified")
	assertContains(t, show, "proposal")
}

func TestDealMove_WithNote(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "Deal"))
	tc.runOK("deal", "move", id, "--stage", "closed-won", "--note", "Signed annual contract")

	show := tc.runOK("deal", "show", id)
	assertContains(t, show, "Signed annual contract")
}

func TestDealMove_ClosedLostWithReason(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "Lost Deal"))
	tc.runOK("deal", "move", id, "--stage", "closed-lost", "--reason", "Budget cut")

	show := tc.runOK("deal", "show", id)
	assertContains(t, show, "closed-lost")
	assertContains(t, show, "Budget cut")
}

func TestDealEdit(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "Old Title", "--value", "10000"))
	tc.runOK("deal", "edit", id, "--title", "New Title", "--value", "20000")

	show := tc.runOK("deal", "show", id)
	assertContains(t, show, "New Title")
	assertContains(t, show, "20000")
}

func TestDealRm(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "Delete Me"))
	tc.runOK("deal", "rm", id, "--force")
	_, _ = tc.runFail("deal", "show", id)
}

func TestPipeline(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "A", "--value", "10000", "--stage", "lead")
	tc.runOK("deal", "add", "--title", "B", "--value", "20000", "--stage", "lead")
	tc.runOK("deal", "add", "--title", "C", "--value", "50000", "--stage", "qualified")

	out := tc.runOK("pipeline")
	assertContains(t, out, "lead")
	assertContains(t, out, "qualified")
	// Should show totals.
	assertContains(t, out, "Total")
}

func TestPipeline_JSON(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "A", "--value", "10000", "--stage", "lead")

	var pipeline []map[string]interface{}
	tc.runJSON(&pipeline, "pipeline", "--format", "json")
	if len(pipeline) == 0 {
		t.Fatal("expected non-empty pipeline JSON")
	}
}

func TestDealShow_LinkedInCompany(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("company", "add", "--name", "Acme", "--domain", "acme.com")
	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "Acme Deal", "--company", "acme.com"))

	// Company show should include the deal.
	coShow := tc.runOK("company", "show", "acme.com")
	assertContains(t, coShow, "Acme Deal")

	// Deal show should reference the company.
	dlShow := tc.runOK("deal", "show", id)
	assertContains(t, dlShow, "Acme")
}
