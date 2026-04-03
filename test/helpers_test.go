package test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// crmBinary is the path to the compiled crm binary, set in TestMain.
var crmBinary string

func TestMain(m *testing.M) {
	// Build the binary once before all tests.
	tmp, err := os.MkdirTemp("", "crm-test-bin")
	if err != nil {
		panic(err)
	}
	crmBinary = filepath.Join(tmp, "crm")

	cmd := exec.Command("go", "build", "-o", crmBinary, "../cmd/crm")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build crm binary: " + err.Error())
	}

	code := m.Run()

	os.RemoveAll(tmp)
	os.Exit(code)
}

// testEnv creates an isolated environment (temp dir with its own DB) for a test.
// Returns a run function and a cleanup function.
type testContext struct {
	t      *testing.T
	dir    string
	dbPath string
}

func newTestContext(t *testing.T) *testContext {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	return &testContext{t: t, dir: dir, dbPath: dbPath}
}

// run executes the crm binary with the given args, pointing at the test database.
// Returns stdout, stderr, and any error.
func (tc *testContext) run(args ...string) (stdout string, stderr string, err error) {
	tc.t.Helper()
	fullArgs := append([]string{"--db", tc.dbPath}, args...)
	cmd := exec.Command(crmBinary, fullArgs...)
	cmd.Dir = tc.dir

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// runOK runs and asserts exit code 0.
func (tc *testContext) runOK(args ...string) string {
	tc.t.Helper()
	stdout, stderr, err := tc.run(args...)
	if err != nil {
		tc.t.Fatalf("crm %s failed: %v\nstderr: %s\nstdout: %s", strings.Join(args, " "), err, stderr, stdout)
	}
	return stdout
}

// runFail runs and asserts non-zero exit code.
func (tc *testContext) runFail(args ...string) (stdout string, stderr string) {
	tc.t.Helper()
	stdout, stderr, err := tc.run(args...)
	if err == nil {
		tc.t.Fatalf("expected crm %s to fail, but it succeeded\nstdout: %s", strings.Join(args, " "), stdout)
	}
	return stdout, stderr
}

// runJSON runs and parses stdout as JSON into the given target.
func (tc *testContext) runJSON(target interface{}, args ...string) {
	tc.t.Helper()
	out := tc.runOK(args...)
	if err := json.Unmarshal([]byte(out), target); err != nil {
		tc.t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out)
	}
}

// assertContains checks that output contains the given substring.
func assertContains(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, output)
	}
}

// assertNotContains checks that output does NOT contain the given substring.
func assertNotContains(t *testing.T, output, substr string) {
	t.Helper()
	if strings.Contains(output, substr) {
		t.Errorf("expected output NOT to contain %q, got:\n%s", substr, output)
	}
}

// assertLineCount checks the number of non-empty lines in output.
func assertLineCount(t *testing.T, output string, expected int) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != expected {
		t.Errorf("expected %d lines, got %d:\n%s", expected, len(lines), output)
	}
}
