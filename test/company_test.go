package test

import (
	"strings"
	"testing"
)

// --- Company CRUD ---

func TestCompanyAdd_Basic(t *testing.T) {
	tc := newTestContext(t)

	out := tc.runOK("company", "add", "--name", "Acme Corp")
	id := strings.TrimSpace(out)
	if !strings.HasPrefix(id, "co_") {
		t.Fatalf("expected company ID starting with co_, got: %s", id)
	}
}

func TestCompanyAdd_Full(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("company", "add",
		"--name", "Acme Corp",
		"--domain", "acme.com",
		"--industry", "SaaS",
		"--size", "50-200",
		"--tag", "enterprise",
		"--set", "founded=2020",
	))

	show := tc.runOK("company", "show", id)
	assertContains(t, show, "Acme Corp")
	assertContains(t, show, "acme.com")
	assertContains(t, show, "SaaS")
	assertContains(t, show, "50-200")
	assertContains(t, show, "enterprise")
	assertContains(t, show, "2020")
}

func TestCompanyAdd_NameRequired(t *testing.T) {
	tc := newTestContext(t)

	_, stderr := tc.runFail("company", "add", "--domain", "acme.com")
	assertContains(t, stderr, "name")
}

func TestCompanyShow_ByDomain(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("company", "add", "--name", "Acme Corp", "--domain", "acme.com")
	out := tc.runOK("company", "show", "acme.com")
	assertContains(t, out, "Acme Corp")
}

func TestCompanyShow_LinkedContacts(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("company", "add", "--name", "Acme Corp", "--domain", "acme.com")
	tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com", "--company", "Acme Corp")
	tc.runOK("contact", "add", "--name", "John Doe", "--email", "john@acme.com", "--company", "Acme Corp")

	show := tc.runOK("company", "show", "acme.com")
	assertContains(t, show, "Jane Doe")
	assertContains(t, show, "John Doe")
}

func TestCompanyList(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("company", "add", "--name", "Acme Corp", "--industry", "SaaS")
	tc.runOK("company", "add", "--name", "Globex", "--industry", "Manufacturing")
	tc.runOK("company", "add", "--name", "Initech", "--industry", "SaaS")

	var companies []map[string]interface{}
	tc.runJSON(&companies, "company", "list", "--format", "json")
	if len(companies) != 3 {
		t.Fatalf("expected 3 companies, got %d", len(companies))
	}
}

func TestCompanyList_FilterByTag(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("company", "add", "--name", "Acme Corp", "--tag", "enterprise")
	tc.runOK("company", "add", "--name", "Small Co")

	var companies []map[string]interface{}
	tc.runJSON(&companies, "company", "list", "--tag", "enterprise", "--format", "json")
	if len(companies) != 1 {
		t.Fatalf("expected 1 enterprise company, got %d", len(companies))
	}
}

func TestCompanyEdit(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("company", "add", "--name", "Acme Corp"))
	tc.runOK("company", "edit", id, "--name", "Acme Inc", "--industry", "Tech")

	show := tc.runOK("company", "show", id)
	assertContains(t, show, "Acme Inc")
	assertContains(t, show, "Tech")
}

func TestCompanyEdit_ByDomain(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("company", "add", "--name", "Acme Corp", "--domain", "acme.com")
	tc.runOK("company", "edit", "acme.com", "--industry", "Fintech")

	show := tc.runOK("company", "show", "acme.com")
	assertContains(t, show, "Fintech")
}

func TestCompanyRm(t *testing.T) {
	tc := newTestContext(t)

	id := strings.TrimSpace(tc.runOK("company", "add", "--name", "Acme Corp"))
	tc.runOK("company", "rm", id, "--force")
	_, _ = tc.runFail("company", "show", id)
}

func TestCompanyRm_DoesNotDeleteLinkedContacts(t *testing.T) {
	tc := newTestContext(t)

	coID := strings.TrimSpace(tc.runOK("company", "add", "--name", "Acme Corp"))
	ctID := strings.TrimSpace(tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com", "--company", "Acme Corp"))

	tc.runOK("company", "rm", coID, "--force")

	// Contact should still exist, just unlinked.
	show := tc.runOK("contact", "show", ctID)
	assertContains(t, show, "Jane")
}

func TestContactAdd_CreatesCompanyStub(t *testing.T) {
	tc := newTestContext(t)

	// Adding a contact with --company should auto-create the company if it doesn't exist.
	tc.runOK("contact", "add", "--name", "Jane", "--company", "NewCo")

	var companies []map[string]interface{}
	tc.runJSON(&companies, "company", "list", "--format", "json")
	if len(companies) != 1 {
		t.Fatalf("expected auto-created company, got %d", len(companies))
	}
}
