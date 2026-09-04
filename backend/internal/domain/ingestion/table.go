package ingestion

import (
	"strings"
)

type Site string

func ParseSite(raw string) (Site, error) {
	canonical := strings.ToLower(strings.TrimSpace(raw))

	if canonical == "" {
		return "", ErrInvalidSite
	}

	if len(canonical) > 253 {
		return "", ErrInvalidSite
	}

	if !strings.Contains(canonical, ".") {
		return "", ErrInvalidSite
	}

	labels := strings.Split(canonical, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return "", ErrInvalidSite
		}

		if label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrInvalidSite
		}

		for i := 0; i < len(label); i++ {
			c := label[i]
			isLower := c >= 'a' && c <= 'z'
			isDigit := c >= '0' && c <= '9'
			isDash := c == '-'

			if !(isLower || isDigit || isDash) {
				return "", ErrInvalidSite
			}
		}
	}

	return Site(canonical), nil
}

func (s Site) String() string {
	return string(s)
}

type Table string

const (
	TableBadges      Table = "badges"
	TableComments    Table = "comments"
	TablePostHistory Table = "post_history"
	TablePostLinks   Table = "post_links"
	TablePosts       Table = "posts"
	TableTags        Table = "tags"
	TableUsers       Table = "users"
	TableVotes       Table = "votes"
)

var tablesByName = map[string]Table{
	"badges":       TableBadges,
	"comments":     TableComments,
	"post_history": TablePostHistory,
	"post_links":   TablePostLinks,
	"posts":        TablePosts,
	"tags":         TableTags,
	"users":        TableUsers,
	"votes":        TableVotes,
}

func ParseTable(raw string) (Table, error) {
	canonical := strings.ToLower(strings.TrimSpace(raw))

	if table, ok := tablesByName[canonical]; ok {
		return table, nil
	}

	return "", UnsupportedTableError{Value: canonical}
}

func ParseTables(raw string) ([]Table, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, ErrEmptyTables
	}

	tokens := strings.Split(raw, ",")

	seen := make(map[Table]bool)
	var result []Table

	for _, token := range tokens {
		tbl, err := ParseTable(token)
		if err != nil {
			return nil, err
		}

		if !seen[tbl] {
			seen[tbl] = true
			result = append(result, tbl)
		}
	}

	return result, nil
}

func SupportedTables() []Table {
	return []Table{
		TableBadges,
		TableComments,
		TablePostHistory,
		TablePostLinks,
		TablePosts,
		TableTags,
		TableUsers,
		TableVotes,
	}
}

func (t Table) String() string {
	return string(t)
}
