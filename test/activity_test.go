package test

import (
	"testing"
)

// --- Activity Logging ---

func TestLogNote(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "Had a great intro call")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--contact", "jane@acme.com", "--format", "json")
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0]["type"] != "note" {
		t.Fatalf("expected type=note, got %s", activities[0]["type"])
	}
}

func TestLogCall(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "call", "jane@acme.com", "Demo scheduled", "--duration", "15m")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--contact", "jane@acme.com", "--format", "json")
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0]["type"] != "call" {
		t.Fatalf("expected type=call, got %s", activities[0]["type"])
	}
}

func TestLogMeeting(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "meeting", "jane@acme.com", "Went through pricing")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--contact", "jane@acme.com", "--format", "json")
	if activities[0]["type"] != "meeting" {
		t.Fatalf("expected type=meeting, got %s", activities[0]["type"])
	}
}

func TestLogEmail(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "email", "jane@acme.com", "Sent proposal PDF")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--contact", "jane@acme.com", "--format", "json")
	if activities[0]["type"] != "email" {
		t.Fatalf("expected type=email, got %s", activities[0]["type"])
	}
}

func TestLogInvalidType(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	_, _ = tc.runFail("log", "tweet", "jane@acme.com", "Hello")
}

func TestLogToNonexistentContact(t *testing.T) {
	tc := newTestContext(t)

	_, _ = tc.runFail("log", "note", "nobody@example.com", "This should fail")
}

func TestLogWithDealLink(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	dealID := tc.runOK("deal", "add", "--title", "Big Deal", "--contact", "jane@acme.com")

	tc.runOK("log", "note", "jane@acme.com", "Discussed pricing", "--deal", dealID)

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--deal", dealID, "--format", "json")
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity linked to deal, got %d", len(activities))
	}
}

func TestLogWithCustomTimestamp(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "Backdated note", "--at", "2026-01-15")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--contact", "jane@acme.com", "--format", "json")
	assertContains(t, activities[0]["created_at"].(string), "2026-01-15")
}

func TestActivityList_FilterByType(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "A note")
	tc.runOK("log", "call", "jane@acme.com", "A call")
	tc.runOK("log", "note", "jane@acme.com", "Another note")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--type", "note", "--format", "json")
	if len(activities) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(activities))
	}
}

func TestActivityList_FilterBySince(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "Old note", "--at", "2025-01-01")
	tc.runOK("log", "note", "jane@acme.com", "New note", "--at", "2026-03-01")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--since", "2026-01-01", "--format", "json")
	if len(activities) != 1 {
		t.Fatalf("expected 1 recent activity, got %d", len(activities))
	}
}

func TestActivityList_Limit(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("log", "note", "jane@acme.com", "Note 1")
	tc.runOK("log", "note", "jane@acme.com", "Note 2")
	tc.runOK("log", "note", "jane@acme.com", "Note 3")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--contact", "jane@acme.com", "--limit", "2", "--format", "json")
	if len(activities) != 2 {
		t.Fatalf("expected 2 activities with limit, got %d", len(activities))
	}
}

func TestActivityOnCompany(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("company", "add", "--name", "Acme", "--domain", "acme.com")
	tc.runOK("log", "note", "acme.com", "Company-level note")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--company", "acme.com", "--format", "json")
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity on company, got %d", len(activities))
	}
}

func TestActivityOnDeal(t *testing.T) {
	tc := newTestContext(t)

	dealID := tc.runOK("deal", "add", "--title", "Big Deal")
	tc.runOK("log", "note", dealID, "Deal-level note")

	var activities []map[string]interface{}
	tc.runJSON(&activities, "activity", "list", "--deal", dealID, "--format", "json")
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity on deal, got %d", len(activities))
	}
}
