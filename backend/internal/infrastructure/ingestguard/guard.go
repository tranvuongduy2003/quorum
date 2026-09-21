package ingestguard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const KindComparisonWriter = "comparison_writer"
const KindInsertLiteral = "insert_literal"

const comparisonPackage = "quorum/internal/adapter/repository/postgres/copybenchmark"
const modulePath = "quorum"

var insertPattern = regexp.MustCompile(`(?i)\binsert\b`)
var checkpointInsertPattern = regexp.MustCompile(`(?i)\binsert\s+into\s+ingest_checkpoints\b`)

type Violation struct {
	Kind    string
	Package string
	File    string
	Detail  string
}

type listedModule struct {
	Path string `json:"Path"`
}

type listedPackage struct {
	Dir        string        `json:"Dir"`
	ImportPath string        `json:"ImportPath"`
	GoFiles    []string      `json:"GoFiles"`
	CgoFiles   []string      `json:"CgoFiles"`
	Module     *listedModule `json:"Module"`
}

func Check(ctx context.Context, workingDir string, packagePattern string) ([]Violation, error) {
	if strings.TrimSpace(workingDir) == "" {
		return nil, errors.New("working directory is required")
	}
	if strings.TrimSpace(packagePattern) == "" {
		return nil, errors.New("package pattern is required")
	}

	command := exec.CommandContext(ctx, "go", "list", "-deps", "-json", packagePattern)
	command.Dir = workingDir
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return nil, fmt.Errorf("list production ingest dependencies: %w", err)
		}
		return nil, fmt.Errorf("list production ingest dependencies: %w: %s", err, detail)
	}

	decoder := json.NewDecoder(bytes.NewReader(output))
	violations := make([]Violation, 0)
	for {
		var pkg listedPackage
		err := decoder.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode production ingest dependency: %w", err)
		}
		if pkg.Module == nil || pkg.Module.Path != modulePath {
			continue
		}

		if pkg.ImportPath == comparisonPackage || strings.HasPrefix(pkg.ImportPath, comparisonPackage+"/") {
			violations = append(violations, Violation{
				Kind:    KindComparisonWriter,
				Package: pkg.ImportPath,
				Detail:  "comparison package is reachable",
			})
		}

		files := append(append([]string(nil), pkg.GoFiles...), pkg.CgoFiles...)
		for _, name := range files {
			path := filepath.Join(pkg.Dir, name)
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return nil, fmt.Errorf("parse production ingest file %q: %w", path, err)
			}

			var inspectErr error
			ast.Inspect(parsed, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}

				value, err := strconv.Unquote(literal.Value)
				if err != nil {
					inspectErr = fmt.Errorf("unquote string literal in %q: %w", path, err)
					return false
				}
				withoutCheckpointUpserts := checkpointInsertPattern.ReplaceAllString(value, "")
				if insertPattern.MatchString(withoutCheckpointUpserts) {
					violations = append(violations, Violation{
						Kind:    KindInsertLiteral,
						Package: pkg.ImportPath,
						File:    path,
						Detail:  value,
					})
				}
				return true
			})
			if inspectErr != nil {
				return nil, inspectErr
			}
		}
	}

	slices.SortFunc(violations, func(left, right Violation) int {
		if value := strings.Compare(left.Package, right.Package); value != 0 {
			return value
		}
		if value := strings.Compare(left.File, right.File); value != 0 {
			return value
		}
		if value := strings.Compare(left.Kind, right.Kind); value != 0 {
			return value
		}
		return strings.Compare(left.Detail, right.Detail)
	})

	return violations, nil
}
