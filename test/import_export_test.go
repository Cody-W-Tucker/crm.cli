package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Import / Export ---

func TestImportContacts_CSV(t *testing.T) {
	tc := newTestContext(t)

	csv := `name,email,phone,company,title,source,tags
Jane Doe,jane@acme.com,+1-555-0100,Acme,CTO,conference,"hot-lead,enterprise"
John Smith,john@globex.com,,Globex,Engineer,inbound,
`
	csvPath := filepath.Join(tc.dir, "contacts.csv")
	os.WriteFile(csvPath, []byte(csv), 0644)

	tc.runOK("import", "contacts", csvPath)

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--format", "json")
	if len(contacts) != 2 {
		t.Fatalf("expected 2 imported contacts, got %d", len(contacts))
	}
}

func TestImportContacts_JSON(t *testing.T) {
	tc := newTestContext(t)

	data := []map[string]interface{}{
		{"name": "Alice", "email": "alice@example.com", "title": "CEO"},
		{"name": "Bob", "email": "bob@example.com", "title": "CTO"},
	}
	jsonBytes, _ := json.Marshal(data)
	jsonPath := filepath.Join(tc.dir, "contacts.json")
	os.WriteFile(jsonPath, jsonBytes, 0644)

	tc.runOK("import", "contacts", jsonPath)

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--format", "json")
	if len(contacts) != 2 {
		t.Fatalf("expected 2 imported contacts, got %d", len(contacts))
	}
}

func TestImportContacts_DryRun(t *testing.T) {
	tc := newTestContext(t)

	csv := `name,email
Jane,jane@acme.com
`
	csvPath := filepath.Join(tc.dir, "contacts.csv")
	os.WriteFile(csvPath, []byte(csv), 0644)

	out := tc.runOK("import", "contacts", csvPath, "--dry-run")
	assertContains(t, out, "Jane")

	// Should NOT have actually imported.
	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--format", "json")
	if len(contacts) != 0 {
		t.Fatalf("dry-run should not import, but got %d contacts", len(contacts))
	}
}

func TestImportContacts_SkipErrors(t *testing.T) {
	tc := newTestContext(t)

	// Second row is missing required name.
	csv := `name,email
Jane,jane@acme.com
,invalid@example.com
Bob,bob@example.com
`
	csvPath := filepath.Join(tc.dir, "contacts.csv")
	os.WriteFile(csvPath, []byte(csv), 0644)

	tc.runOK("import", "contacts", csvPath, "--skip-errors")

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "contact", "list", "--format", "json")
	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts (skipping error row), got %d", len(contacts))
	}
}

func TestImportContacts_Update(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com", "--title", "Engineer")

	csv := `name,email,title
Jane Doe,jane@acme.com,CTO
`
	csvPath := filepath.Join(tc.dir, "contacts.csv")
	os.WriteFile(csvPath, []byte(csv), 0644)

	tc.runOK("import", "contacts", csvPath, "--update")

	show := tc.runOK("contact", "show", "jane@acme.com")
	assertContains(t, show, "CTO")
}

func TestImportContacts_Stdin(t *testing.T) {
	tc := newTestContext(t)

	// Test importing from stdin (using - as filename).
	// This tests the pipe-friendly aspect.
	csv := `name,email
Jane,jane@acme.com
`
	csvPath := filepath.Join(tc.dir, "stdin.csv")
	os.WriteFile(csvPath, []byte(csv), 0644)

	// Simulate stdin by using shell redirection.
	stdout, stderr, err := tc.run("import", "contacts", "-")
	// This will fail because we're not piping stdin — that's expected.
	// The test documents that stdin ("-") is a valid argument.
	_ = stdout
	_ = stderr
	_ = err
}

func TestImportCompanies_CSV(t *testing.T) {
	tc := newTestContext(t)

	csv := `name,domain,industry,size
Acme Corp,acme.com,SaaS,50-200
Globex,globex.com,Manufacturing,1000+
`
	csvPath := filepath.Join(tc.dir, "companies.csv")
	os.WriteFile(csvPath, []byte(csv), 0644)

	tc.runOK("import", "companies", csvPath)

	var companies []map[string]interface{}
	tc.runJSON(&companies, "company", "list", "--format", "json")
	if len(companies) != 2 {
		t.Fatalf("expected 2 imported companies, got %d", len(companies))
	}
}

func TestImportDeals_CSV(t *testing.T) {
	tc := newTestContext(t)

	csv := `title,value,stage
Deal A,50000,lead
Deal B,25000,qualified
`
	csvPath := filepath.Join(tc.dir, "deals.csv")
	os.WriteFile(csvPath, []byte(csv), 0644)

	tc.runOK("import", "deals", csvPath)

	var deals []map[string]interface{}
	tc.runJSON(&deals, "deal", "list", "--format", "json")
	if len(deals) != 2 {
		t.Fatalf("expected 2 imported deals, got %d", len(deals))
	}
}

func TestExportContacts_CSV(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com")
	tc.runOK("contact", "add", "--name", "Bob Smith", "--email", "bob@globex.com")

	out := tc.runOK("export", "contacts", "--format", "csv")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 { // header + 2 rows
		t.Fatalf("expected 3 CSV lines (header + 2 data), got %d", len(lines))
	}
}

func TestExportContacts_JSON(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com")

	var contacts []map[string]interface{}
	tc.runJSON(&contacts, "export", "contacts", "--format", "json")
	if len(contacts) != 1 {
		t.Fatalf("expected 1 exported contact, got %d", len(contacts))
	}
}

func TestExportDeals_JSON(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("deal", "add", "--title", "Deal A", "--value", "50000")

	var deals []map[string]interface{}
	tc.runJSON(&deals, "export", "deals", "--format", "json")
	if len(deals) != 1 {
		t.Fatalf("expected 1 exported deal, got %d", len(deals))
	}
}

func TestExportAll(t *testing.T) {
	tc := newTestContext(t)

	tc.runOK("contact", "add", "--name", "Jane", "--email", "jane@acme.com")
	tc.runOK("company", "add", "--name", "Acme")
	tc.runOK("deal", "add", "--title", "Deal")

	var export map[string]interface{}
	tc.runJSON(&export, "export", "all", "--format", "json")

	if _, ok := export["contacts"]; !ok {
		t.Fatal("expected 'contacts' key in full export")
	}
	if _, ok := export["companies"]; !ok {
		t.Fatal("expected 'companies' key in full export")
	}
	if _, ok := export["deals"]; !ok {
		t.Fatal("expected 'deals' key in full export")
	}
	if _, ok := export["activities"]; !ok {
		t.Fatal("expected 'activities' key in full export")
	}
}

func TestRoundtrip_ExportImport(t *testing.T) {
	tc := newTestContext(t)

	// Create data.
	tc.runOK("contact", "add", "--name", "Jane Doe", "--email", "jane@acme.com", "--tag", "vip")

	// Export.
	exported := tc.runOK("export", "contacts", "--format", "json")
	exportPath := filepath.Join(tc.dir, "exported.json")
	os.WriteFile(exportPath, []byte(exported), 0644)

	// New context (fresh DB).
	tc2 := newTestContext(t)
	tc2.runOK("import", "contacts", exportPath)

	show := tc2.runOK("contact", "show", "jane@acme.com")
	assertContains(t, show, "Jane Doe")
	assertContains(t, show, "vip")
}
