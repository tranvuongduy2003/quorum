package ingestguard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProductionIngestGraphHasNoViolations(t *testing.T) {
	root := filepath.Join("..", "..", "..")

	violations, err := Check(context.Background(), root, "./cmd/ingest")

	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("violations = %#v, want none", violations)
	}
}

func TestCheckFindsReachableInsertAndIgnoresUnreachableAndTestLiterals(t *testing.T) {
	root := newModule(t)
	writeModuleFile(t, root, "cmd/ingest/main.go", "package main\n\nimport \"quorum/internal/production\"\n\nfunc main() { _ = production.Statement }\n")
	writeModuleFile(t, root, "internal/production/production.go", "package production\n\nconst Statement = \"INSERT INTO records VALUES (1)\"\n")
	writeModuleFile(t, root, "internal/production/production_test.go", "package production\n\nconst testStatement = \"INSERT INTO test_records VALUES (1)\"\n")
	writeModuleFile(t, root, "internal/adapter/repository/postgres/copybenchmark/writer.go", "package copybenchmark\n\nconst Statement = \"INSERT INTO benchmark_records VALUES (1)\"\n")

	violations, err := Check(context.Background(), root, "./cmd/ingest")

	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("violations = %#v, want one", violations)
	}
	violation := violations[0]
	if violation.Kind != KindInsertLiteral || violation.Package != "quorum/internal/production" || violation.Detail != "INSERT INTO records VALUES (1)" {
		t.Fatalf("violation = %#v", violation)
	}
	if !filepath.IsAbs(violation.File) {
		t.Fatalf("violation file = %q, want absolute path", violation.File)
	}
}

func TestCheckFindsReachableComparisonPackage(t *testing.T) {
	root := newModule(t)
	writeModuleFile(t, root, "cmd/ingest/main.go", "package main\n\nimport \"quorum/internal/adapter/repository/postgres/copybenchmark\"\n\nfunc main() { _ = copybenchmark.Name }\n")
	writeModuleFile(t, root, "internal/adapter/repository/postgres/copybenchmark/writer.go", "package copybenchmark\n\nconst Name = \"comparison\"\n")

	violations, err := Check(context.Background(), root, "./cmd/ingest")

	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if len(violations) != 1 || violations[0].Kind != KindComparisonWriter || violations[0].Package != comparisonPackage {
		t.Fatalf("violations = %#v", violations)
	}
}

func TestCheckRejectsEmptyInputs(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		pattern string
	}{
		{name: "directory", pattern: "./cmd/ingest"},
		{name: "pattern", dir: "."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Check(context.Background(), test.dir, test.pattern); err == nil {
				t.Fatal("Check() error = nil")
			}
		})
	}
}

func newModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeModuleFile(t, root, "go.mod", "module quorum\n\ngo 1.25.0\n")
	return root
}

func writeModuleFile(t *testing.T, root string, name string, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
