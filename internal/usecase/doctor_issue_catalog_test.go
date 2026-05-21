package usecase

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestDoctorIssueCatalogIsInSync enforces a two-way contract between the
// runtime code that emits Doctor issues and docs/doctor-issues.md, so the
// catalog stays a credible reference for downstream scripts:
//
//   - every code emitted by addIssue(...) / runtimeStateIssue.Code must
//     appear as a row in the catalog; otherwise scripts see codes that
//     aren't documented and the compatibility promise leaks
//   - every code documented in the catalog must actually appear in the
//     source; otherwise the catalog lists ghost codes the runtime can
//     never emit
//
// When you add a new doctor issue:
//
//  1. add the addIssue(...) / runtimeStateIssue{...} call in source
//  2. add a row to docs/doctor-issues.md AND docs/doctor-issues.zh.md
//  3. re-run `task verify`
func TestDoctorIssueCatalogIsInSync(t *testing.T) {
	emitted := scanEmittedIssueCodes(t)
	documented := scanDocumentedIssueCodes(t, "docs/doctor-issues.md")

	if missing := setDifference(emitted, documented); len(missing) > 0 {
		t.Errorf("doctor codes emitted in source but missing from docs/doctor-issues.md: %v", missing)
	}
	if extra := setDifference(documented, emitted); len(extra) > 0 {
		t.Errorf("doctor codes documented in docs/doctor-issues.md but not emitted from source: %v", extra)
	}

	zhDocumented := scanDocumentedIssueCodes(t, "docs/doctor-issues.zh.md")
	if missing := setDifference(emitted, zhDocumented); len(missing) > 0 {
		t.Errorf("doctor codes emitted in source but missing from docs/doctor-issues.zh.md: %v", missing)
	}
	if extra := setDifference(zhDocumented, emitted); len(extra) > 0 {
		t.Errorf("doctor codes documented in docs/doctor-issues.zh.md but not emitted from source: %v", extra)
	}
}

// scanEmittedIssueCodes greps the usecase package source for:
//
//   - addIssue(<severity>, "<code>", ...) calls
//   - runtimeStateIssue{ ... Code: "<code>" ... } literals
//
// String literals are the source of truth; matching on the literal
// avoids any runtime fixture setup and keeps the test fast and deterministic.
func scanEmittedIssueCodes(t *testing.T) []string {
	t.Helper()

	addIssue := regexp.MustCompile(`addIssue\(\s*severity\w+\s*,\s*"([a-z0-9_]+)"`)
	runtimeIssue := regexp.MustCompile(`Code:\s*"([a-z0-9_]+)"`)

	pkgDir := packageDir(t)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read usecase dir: %v", err)
	}

	codes := map[string]struct{}{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(pkgDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range addIssue.FindAllSubmatch(data, -1) {
			codes[string(m[1])] = struct{}{}
		}
		for _, m := range runtimeIssue.FindAllSubmatch(data, -1) {
			codes[string(m[1])] = struct{}{}
		}
	}

	if len(codes) == 0 {
		t.Fatalf("found no emitted doctor codes; regex likely needs updating")
	}
	return sortedKeys(codes)
}

// scanDocumentedIssueCodes pulls the codes from the markdown catalog by
// matching backtick-quoted table cells in the leading column of the Codes
// table. We deliberately avoid a full markdown parser dependency.
// The Severity table earlier in the doc also uses backtick-quoted leading
// cells (`error`, `warning`), so parsing is anchored to the line that
// introduces the Codes section.
func scanDocumentedIssueCodes(t *testing.T, relPath string) []string {
	t.Helper()

	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}

	codesSection := -1
	for i, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "## Codes") {
			codesSection = i
			break
		}
	}
	if codesSection < 0 {
		t.Fatalf("%s: missing '## Codes' section header", relPath)
	}

	row := regexp.MustCompile("^\\|\\s*`([a-z0-9_]+)`\\s*\\|")
	codes := map[string]struct{}{}
	for _, line := range strings.Split(string(data), "\n")[codesSection:] {
		m := row.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		codes[m[1]] = struct{}{}
	}
	if len(codes) == 0 {
		t.Fatalf("found no documented codes in %s; check the catalog format", relPath)
	}
	return sortedKeys(codes)
}

func setDifference(a, b []string) []string {
	want := map[string]struct{}{}
	for _, v := range b {
		want[v] = struct{}{}
	}
	var out []string
	for _, v := range a {
		if _, ok := want[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func packageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// The test file lives under internal/usecase/, so the repo root is two
	// levels up.
	return filepath.Join(packageDir(t), "..", "..")
}
