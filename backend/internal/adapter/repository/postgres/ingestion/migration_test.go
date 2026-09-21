package ingestion

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCorpusMigrationContainsSchemaContract(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}

	path := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "..", "migrations", "000002_corpus_tables.sql")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	sql := strings.Join(strings.Fields(string(contents)), " ")
	required := []string{
		"CREATE TABLE users (",
		"CREATE TABLE posts (",
		"CREATE TABLE post_bodies (",
		"CREATE TABLE comments (",
		"CREATE TABLE votes (",
		"CREATE TABLE badges (",
		"CREATE TABLE tags (",
		"CREATE TABLE post_links (",
		"CREATE TABLE post_history (",
		"FOREIGN KEY (site, post_id) REFERENCES posts (site, id) ON DELETE CASCADE",
		"ALTER TABLE post_bodies ALTER COLUMN body SET COMPRESSION lz4",
		"comments_default PARTITION OF comments DEFAULT",
		"votes_default PARTITION OF votes DEFAULT",
		"source_offset bigint NOT NULL CHECK (source_offset >= 0)",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}

	if got := strings.Count(sql, "PRIMARY KEY (site, id, created_at)"); got != 2 {
		t.Errorf("partitioned primary key count = %d, want 2", got)
	}
	if got := strings.Count(sql, "PRIMARY KEY (site, id)"); got != 6 {
		t.Errorf("unpartitioned primary key count = %d, want 6", got)
	}
	if got := strings.Count(sql, "source_offset bigint NOT NULL CHECK (source_offset >= 0)"); got != 8 {
		t.Errorf("source offset contract count = %d, want 8", got)
	}
	if !strings.Contains(sql, "body text NOT NULL") || !strings.Contains(sql, "tags text[] NOT NULL DEFAULT '{}'") {
		t.Error("migration does not preserve required post body and tags nullability")
	}
	if strings.Contains(sql, "accepted_answer_id bigint NOT NULL") || strings.Contains(sql, "owner_user_id bigint NOT NULL") || strings.Contains(sql, "user_id bigint NOT NULL, created_at timestamptz NOT NULL, bounty_amount") {
		t.Error("migration makes a nullable corpus relationship mandatory")
	}
}

func TestCheckpointMigrationContainsSchemaContract(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}

	path := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "..", "migrations", "000004_ingest_checkpoints.sql")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	sql := strings.Join(strings.Fields(string(contents)), " ")
	for _, fragment := range []string{
		"CREATE TABLE ingest_checkpoints (",
		"site text NOT NULL",
		"source_table text NOT NULL",
		"archive_id text NOT NULL",
		"source_offset bigint NOT NULL CHECK (source_offset >= 0)",
		"confirmed_count bigint NOT NULL CHECK (confirmed_count >= 0)",
		"updated_at timestamptz NOT NULL DEFAULT now()",
		"PRIMARY KEY (site, source_table)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
}
