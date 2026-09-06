package config

import (
	"strings"
	"testing"
)

func TestLoadPostgresLoadsOnlyPostgresConfiguration(t *testing.T) {
	setValidPostgresEnvironment(t)
	t.Setenv("REDIS_HOST", "")

	cfg, err := LoadPostgres()

	if err != nil {
		t.Fatalf("LoadPostgres() error = %v", err)
	}
	if got, want := cfg.URL, "postgres://app:secret@database.internal:5433/quorum?sslmode=require"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	if cfg.MaxConns != 12 || cfg.MinConns != 3 {
		t.Fatalf("pool bounds = %d/%d, want 12/3", cfg.MaxConns, cfg.MinConns)
	}
}

func TestLoadPostgresReturnsAllValidationErrors(t *testing.T) {
	setValidPostgresEnvironment(t)
	t.Setenv("POSTGRES_DB", "")
	t.Setenv("POSTGRES_MAX_CONNS", "1")
	t.Setenv("POSTGRES_MIN_CONNS", "2")

	cfg, err := LoadPostgres()

	if err == nil {
		t.Fatal("LoadPostgres() error = nil")
	}
	if cfg != (PostgresConfig{}) {
		t.Fatalf("config = %#v, want empty", cfg)
	}
	for _, fragment := range []string{"POSTGRES_DB", "POSTGRES_MAX_CONNS must be greater than or equal to POSTGRES_MIN_CONNS"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("error missing %q: %v", fragment, err)
		}
	}
}

func setValidPostgresEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("POSTGRES_DB", "quorum")
	t.Setenv("POSTGRES_USER", "app")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_HOST", "database.internal")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_SSLMODE", "require")
	t.Setenv("POSTGRES_MAX_CONNS", "12")
	t.Setenv("POSTGRES_MIN_CONNS", "3")
}
