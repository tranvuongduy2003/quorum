package ingestion

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSite(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Site
		err  error
	}{
		{name: "canonicalizes", raw: " StackOverflow.COM ", want: "stackoverflow.com"},
		{name: "accepts stack exchange site", raw: "academia.stackexchange.com", want: "academia.stackexchange.com"},
		{name: "rejects empty", raw: "", err: ErrInvalidSite},
		{name: "rejects missing dot", raw: "localhost", err: ErrInvalidSite},
		{name: "rejects path", raw: "../posts", err: ErrInvalidSite},
		{name: "rejects underscore", raw: "bad_label.com", err: ErrInvalidSite},
		{name: "rejects leading hyphen", raw: "-bad.com", err: ErrInvalidSite},
		{name: "rejects trailing hyphen", raw: "bad-.com", err: ErrInvalidSite},
		{name: "rejects long label", raw: strings.Repeat("a", 64) + ".com", err: ErrInvalidSite},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseSite(test.raw)
			if !errors.Is(err, test.err) {
				t.Fatalf("ParseSite(%q) error = %v, want %v", test.raw, err, test.err)
			}
			if got != test.want {
				t.Fatalf("ParseSite(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func TestParseTablesCanonicalizesAndDeduplicates(t *testing.T) {
	got, err := ParseTables("posts, Votes,posts")
	if err != nil {
		t.Fatalf("ParseTables() error = %v", err)
	}

	want := []Table{TablePosts, TableVotes}
	if len(got) != len(want) {
		t.Fatalf("ParseTables() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("ParseTables() = %v, want %v", got, want)
		}
	}
}

func TestSupportedTablesAreCompleteAndIndependent(t *testing.T) {
	tables := SupportedTables()
	if len(tables) != 8 {
		t.Fatalf("SupportedTables() length = %d, want 8", len(tables))
	}

	for _, table := range tables {
		got, err := ParseTable(table.String())
		if err != nil {
			t.Fatalf("ParseTable(%q) error = %v", table, err)
		}
		if got != table {
			t.Fatalf("ParseTable(%q) = %q, want %q", table, got, table)
		}
	}

	tables[0] = TableVotes
	if SupportedTables()[0] != TableBadges {
		t.Fatal("SupportedTables() returned shared storage")
	}
}

func TestParseTableRetainsUnsupportedValue(t *testing.T) {
	_, err := ParseTable(" Widgets ")
	var tableErr UnsupportedTableError
	if !errors.As(err, &tableErr) {
		t.Fatalf("ParseTable() error = %T, want UnsupportedTableError", err)
	}
	if tableErr.Value != "widgets" {
		t.Fatalf("unsupported value = %q, want widgets", tableErr.Value)
	}
	if !errors.Is(err, ErrUnsupportedTable) {
		t.Fatalf("ParseTable() error = %v, want ErrUnsupportedTable", err)
	}
	if err.Error() != `unsupported corpus table "widgets"` {
		t.Fatalf("ParseTable() error = %q", err)
	}
}

func TestParseTablesDistinguishesBlankSelection(t *testing.T) {
	_, err := ParseTables(" \t")
	if !errors.Is(err, ErrEmptyTables) {
		t.Fatalf("ParseTables() error = %v, want ErrEmptyTables", err)
	}

	_, err = ParseTables("posts,")
	var tableErr UnsupportedTableError
	if !errors.As(err, &tableErr) || tableErr.Value != "" {
		t.Fatalf("ParseTables() error = %#v, want empty UnsupportedTableError", err)
	}
}
