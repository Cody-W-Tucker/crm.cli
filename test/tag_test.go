package test

import (
	"strings"
	"testing"
)

// --- Tags ---

func TestTagContact(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com"))
	tc.runOK("tag", id, "hot-lead", "enterprise")

	show := tc.runOK("contact", "show", id)
	assertContains(t, show, "hot-lead")
	assertContains(t, show, "enterprise")
}

func TestTagCompany(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("company", "add", "--name", "Acme", "--domain", "acme.com")
	tc.runOK("tag", "acme.com", "target-account")

	show := tc.runOK("company", "show", "acme.com")
	assertContains(t, show, "target-account")
}

func TestTagDeal(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("deal", "add", "--title", "Big Deal"))
	tc.runOK("tag", id, "q2", "priority")

	show := tc.runOK("deal", "show", id)
	assertContains(t, show, "q2")
	assertContains(t, show, "priority")
}

func TestUntag(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("contact", "add", "--name", "Jane", "--tag", "vip", "--tag", "cold"))
	tc.runOK("untag", id, "cold")

	show := tc.runOK("contact", "show", id)
	assertContains(t, show, "vip")
	assertNotContains(t, show, "cold")
}

func TestTagList(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Alice", "--tag", "vip", "--tag", "hot-lead")
	tc.runOK("contact", "add", "--name", "Bob", "--tag", "vip")
	tc.runOK("company", "add", "--name", "Acme", "--tag", "enterprise")

	out := tc.runOK("tag", "list")
	assertContains(t, out, "vip")
	assertContains(t, out, "hot-lead")
	assertContains(t, out, "enterprise")
}

func TestTagList_FilterByType(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Alice", "--tag", "person-tag")
	tc.runOK("company", "add", "--name", "Acme", "--tag", "company-tag")

	out := tc.runOK("tag", "list", "--type", "contact")
	assertContains(t, out, "person-tag")
	assertNotContains(t, out, "company-tag")
}

func TestTagIdempotent(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("contact", "add", "--name", "Jane", "--tag", "vip"))
	// Tagging again with same tag should not error or duplicate.
	tc.runOK("tag", id, "vip")

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--tag", "vip", "--format", "json")
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(contacts))
	}
}

func TestTagByEmail(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("tag", "jane@acme.com", "vip")

	show := tc.runOK("contact", "show", "jane@acme.com")
	assertContains(t, show, "vip")
}
