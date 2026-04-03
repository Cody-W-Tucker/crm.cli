package test

import (
	"strings"
	"testing"
)

// --- Contact CRUD ---

func TestContactAdd_Basic(t *testing.T) {
	tc := newTestContext(t)

	// Adding a contact with just a name should succeed and print an ID.
	out := tc.runOK("contact", "add", "--name", "Jane Doe")
	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, "ct_") {
		t.Fatalf("expected contact ID starting with ct_, got: %s", out)
	}
}

func TestContactAdd_Full(t *testing.T) {
	tc := newTestContext(t)

	out := tc.runOK("contact", "add",
		"--name", "Jane Doe",
		"--email", "jane@acme.com",
		"--phone", "+1-555-0100",
		"--company", "Acme Corp",
		"--title", "CTO",
		"--source", "conference",
		"--tag", "hot-lead",
		"--tag", "enterprise",
		"--set", "linkedin=linkedin.com/in/janedoe",
	)
	id := strings.TrimSpace(out)
	if !strings.HasPrefix(id, "ct_") {
		t.Fatalf("expected contact ID, got: %s", id)
	}

	// Verify the contact was stored correctly.
	show := tc.runOK("contact", "show", id)
	assertContains(t, show, "Jane Doe")
	assertContains(t, show, "jane@acme.com")
	assertContains(t, show, "+1-555-0100")
	assertContains(t, show, "CTO")
	assertContains(t, show, "conference")
	assertContains(t, show, "hot-lead")
	assertContains(t, show, "enterprise")
	assertContains(t, show, "linkedin.com/in/janedoe")
}

func TestContactAdd_NameRequired(t *testing.T) {
	tc := newTestContext(t)

	// Should fail without --name.
	_, stderr := tc.runFail("contact", "add", "--email", "nobody@example.com")
	assertContains(t, stderr, "name")
}

func TestContactAdd_DuplicateEmail(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com")
	// Adding another contact with the same email should fail.
	_, stderr := tc.runFail("contact", "add", "--name", "Jane Smith", "--email", "jane@acme.com")
	assertContains(t, stderr, "duplicate")
}

func TestContactShow_ByEmail(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com")
	out := tc.runOK("contact", "show", "jane@acme.com")
	assertContains(t, out, "Jane Doe")
}

func TestContactShow_NotFound(t *testing.T) {
	tc := newTestContext(t)

	_, _ = tc.runFail("contact", "show", "nonexistent@example.com")
}

func TestContactList_Empty(t *testing.T) {
	tc := newTestContext(t)

	out := tc.runOK("contact", "list", "--format", "json")
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("expected empty JSON array, got: %s", out)
	}
}

func TestContactList_Multiple(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Alice", "--email", "alice@example.com")
	tc.runOK("contact", "add", "--name", "Bob", "--email", "bob@example.com")
	tc.runOK("contact", "add", "--name", "Charlie", "--email", "charlie@example.com")

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--format", "json")
	if len(contacts) != 3 {
		t.Fatalf("expected 3 contacts, got %d", len(contacts))
	}
}

func TestContactList_FilterByTag(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Alice", "--email", "alice@example.com", "--tag", "vip")
	tc.runOK("contact", "add", "--name", "Bob", "--email", "bob@example.com")

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--tag", "vip", "--format", "json")
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact with tag vip, got %d", len(contacts))
	}
}

func TestContactList_FilterByCompany(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Alice", "--email", "alice@acme.com", "--company", "Acme")
	tc.runOK("contact", "add", "--name", "Bob", "--email", "bob@other.com", "--company", "Other")

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--company", "Acme", "--format", "json")
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact at Acme, got %d", len(contacts))
	}
}

func TestContactList_Sort(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Charlie")
	tc.runOK("contact", "add", "--name", "Alice")
	tc.runOK("contact", "add", "--name", "Bob")

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--sort", "name", "--format", "json")
	names := make([]string, len(contacts))
	for i, c := range contacts {
		names[i] = c["name"].(string)
	}
	if names[0] != "Alice" || names[1] != "Bob" || names[2] != "Charlie" {
		t.Fatalf("expected alphabetical order, got: %v", names)
	}
}

func TestContactList_LimitOffset(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "A")
	tc.runOK("contact", "add", "--name", "B")
	tc.runOK("contact", "add", "--name", "C")
	tc.runOK("contact", "add", "--name", "D")

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--limit", "2", "--format", "json")
	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts with limit, got %d", len(contacts))
	}

	tc.runJSON(&contacts, "contact", "list", "--limit", "2", "--offset", "2", "--format", "json")
	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts with offset, got %d", len(contacts))
	}
}

func TestContactList_FormatIDs(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Alice")
	tc.runOK("contact", "add", "--name", "Bob")

	out := tc.runOK("contact", "list", "--format", "ids")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(lines))
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "ct_") {
			t.Errorf("expected ID starting with ct_, got: %s", line)
		}
	}
}

func TestContactList_FormatCSV(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Alice", "--email", "alice@example.com")

	out := tc.runOK("contact", "list", "--format", "csv")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header + data row, got %d lines", len(lines))
	}
	// First line should be headers.
	assertContains(t, lines[0], "name")
	assertContains(t, lines[0], "email")
	// Second line should have the data.
	assertContains(t, lines[1], "Alice")
	assertContains(t, lines[1], "alice@example.com")
}

func TestContactEdit(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com", "--title", "Engineer"))

	tc.runOK("contact", "edit", id, "--name", "Jane Smith", "--title", "CTO")

	show := tc.runOK("contact", "show", id)
	assertContains(t, show, "Jane Smith")
	assertContains(t, show, "CTO")
	assertNotContains(t, show, "Jane Doe")
}

func TestContactEdit_ByEmail(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com")
	tc.runOK("contact", "edit", "jane@acme.com", "--title", "CEO")

	show := tc.runOK("contact", "show", "jane@acme.com")
	assertContains(t, show, "CEO")
}

func TestContactEdit_CustomFields(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("contact", "add", "--name", "Jane", "--set", "github=janedoe"))

	// Update a custom field.
	tc.runOK("contact", "edit", id, "--set", "github=janesmith")
	show := tc.runOK("contact", "show", id)
	assertContains(t, show, "janesmith")

	// Remove a custom field.
	tc.runOK("contact", "edit", id, "--unset", "github")
	show = tc.runOK("contact", "show", id)
	assertNotContains(t, show, "github")
}

func TestContactEdit_Tags(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("contact", "add", "--name", "Jane", "--tag", "lead"))

	tc.runOK("contact", "edit", id, "--add-tag", "vip", "--rm-tag", "lead")

	show := tc.runOK("contact", "show", id)
	assertContains(t, show, "vip")
	assertNotContains(t, show, "lead")
}

func TestContactRm(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com"))

	tc.runOK("contact", "rm", id, "--force")

	_, _ = tc.runFail("contact", "show", id)
}

func TestContactRm_ByEmail(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("contact", "rm", "jane@acme.com", "--force")
	_, _ = tc.runFail("contact", "show", "jane@acme.com")
}

func TestContactList_Filter(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Alice", "--title", "CTO", "--source", "conference")
	tc.runOK("contact", "add", "--name", "Bob", "--title", "Engineer", "--source", "inbound")
	tc.runOK("contact", "add", "--name", "Charlie", "--title", "CTO", "--source", "inbound")

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--filter", "title=CTO AND source=inbound", "--format", "json")
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact matching filter, got %d", len(contacts))
	}
}

func TestContactMerge(t *testing.T) {
	tc := newTestContext(t)

	id1 := strings.TrimSpace(tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com", "--tag", "vip"))
	id2 := strings.TrimSpace(tc.runOK("contact", "add", "--name", "J. Doe", "--email", "jane.doe@gmail.com", "--tag", "enterprise"))

	tc.runOK("contact", "merge", id1, id2, "--keep-first")

	// Primary contact should have both emails and both tags.
	show := tc.runOK("contact", "show", id1)
	assertContains(t, show, "jane@acme.com")
	assertContains(t, show, "jane.doe@gmail.com")
	assertContains(t, show, "vip")
	assertContains(t, show, "enterprise")

	// Second contact should no longer exist.
	_, _ = tc.runFail("contact", "show", id2)
}
