package ingestion

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	domainingestion "quorum/internal/domain/ingestion"

	"github.com/google/uuid"
)

func TestAcceptedPlansMatchDestinationContracts(t *testing.T) {
	site := domainingestion.Site("stackoverflow.com")
	timestamp := time.Date(2026, time.January, 2, 3, 4, 5, 6_000_000, time.UTC)
	revision := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")

	tests := []struct {
		table        domainingestion.Table
		attributes   map[string]string
		destinations []string
		columns      [][]string
		rows         [][]any
	}{
		{
			table: domainingestion.TableUsers,
			attributes: map[string]string{
				"Id": "1", "Reputation": "2", "CreationDate": "2026-01-02T03:04:05.006", "DisplayName": "Đỗ 🚀",
				"LastAccessDate": "2026-01-02T03:04:05.006", "WebsiteUrl": "", "Location": "Hà Nội", "AboutMe": "about",
				"Views": "3", "UpVotes": "4", "DownVotes": "5", "ProfileImageUrl": "", "AccountId": "6",
			},
			destinations: []string{"users"},
			columns:      [][]string{{"site", "id", "reputation", "created_at", "display_name", "last_access_at", "website_url", "location", "about_me", "views", "up_votes", "down_votes", "profile_image_url", "email_hash", "account_id", "source_offset"}},
			rows:         [][]any{{site.String(), int64(1), int32(2), timestamp, "Đỗ 🚀", timestamp, "", "Hà Nội", "about", int32(3), int32(4), int32(5), "", nil, int64(6), int64(840)}},
		},
		{
			table: domainingestion.TablePosts,
			attributes: map[string]string{
				"Id": "10", "PostTypeId": "1", "CreationDate": "2026-01-02T03:04:05.006", "Score": "7",
				"LastActivityDate": "2026-01-02T03:04:05.006", "Title": "", "Tags": "<go><postgresql>", "Body": "Xin chào 🌏",
			},
			destinations: []string{"posts", "post_bodies"},
			columns: [][]string{
				{"site", "id", "post_type_id", "accepted_answer_id", "parent_id", "created_at", "deleted_at", "score", "view_count", "owner_user_id", "owner_display_name", "last_editor_user_id", "last_editor_display_name", "last_edit_at", "last_activity_at", "title", "tags", "answer_count", "comment_count", "favorite_count", "closed_at", "community_owned_at", "content_license", "source_offset"},
				{"site", "post_id", "body"},
			},
			rows: [][]any{
				{site.String(), int64(10), int16(1), nil, nil, timestamp, nil, int32(7), int64(0), nil, nil, nil, nil, nil, timestamp, "", []string{"go", "postgresql"}, int32(0), int32(0), nil, nil, nil, nil, int64(840)},
				{site.String(), int64(10), "Xin chào 🌏"},
			},
		},
		{
			table: domainingestion.TableComments,
			attributes: map[string]string{
				"Id": "20", "PostId": "10", "Score": "3", "Text": "", "CreationDate": "2026-01-02T03:04:05.006", "UserDisplayName": "Nguyễn",
			},
			destinations: []string{"comments"},
			columns:      [][]string{{"site", "id", "post_id", "score", "text", "created_at", "user_display_name", "user_id", "content_license", "source_offset"}},
			rows:         [][]any{{site.String(), int64(20), int64(10), int32(3), "", timestamp, "Nguyễn", nil, nil, int64(840)}},
		},
		{
			table: domainingestion.TableVotes,
			attributes: map[string]string{
				"Id": "30", "PostId": "10", "VoteTypeId": "2", "CreationDate": "2026-01-02T03:04:05.006", "BountyAmount": "50",
			},
			destinations: []string{"votes"},
			columns:      [][]string{{"site", "id", "post_id", "vote_type_id", "user_id", "created_at", "bounty_amount", "source_offset"}},
			rows:         [][]any{{site.String(), int64(30), int64(10), int16(2), nil, timestamp, int32(50), int64(840)}},
		},
		{
			table: domainingestion.TableBadges,
			attributes: map[string]string{
				"Id": "40", "UserId": "1", "Name": "Teacher", "Date": "2026-01-02T03:04:05.006", "Class": "1", "TagBased": "true",
			},
			destinations: []string{"badges"},
			columns:      [][]string{{"site", "id", "user_id", "name", "awarded_at", "class", "tag_based", "source_offset"}},
			rows:         [][]any{{site.String(), int64(40), int64(1), "Teacher", timestamp, int16(1), true, int64(840)}},
		},
		{
			table: domainingestion.TableTags,
			attributes: map[string]string{
				"Id": "50", "TagName": "go", "Count": "9", "ExcerptPostId": "10", "IsModeratorOnly": "false", "IsRequired": "true",
			},
			destinations: []string{"tags"},
			columns:      [][]string{{"site", "id", "name", "usage_count", "excerpt_post_id", "wiki_post_id", "moderator_only", "required", "source_offset"}},
			rows:         [][]any{{site.String(), int64(50), "go", int32(9), int64(10), nil, false, true, int64(840)}},
		},
		{
			table: domainingestion.TablePostLinks,
			attributes: map[string]string{
				"Id": "60", "CreationDate": "2026-01-02T03:04:05.006", "PostId": "10", "RelatedPostId": "11", "LinkTypeId": "3",
			},
			destinations: []string{"post_links"},
			columns:      [][]string{{"site", "id", "created_at", "post_id", "related_post_id", "link_type_id", "source_offset"}},
			rows:         [][]any{{site.String(), int64(60), timestamp, int64(10), int64(11), int16(3), int64(840)}},
		},
		{
			table: domainingestion.TablePostHistory,
			attributes: map[string]string{
				"Id": "70", "PostHistoryTypeId": "4", "PostId": "10", "RevisionGUID": revision.String(), "CreationDate": "2026-01-02T03:04:05.006", "UserDisplayName": "", "Text": "nội dung",
			},
			destinations: []string{"post_history"},
			columns:      [][]string{{"site", "id", "history_type_id", "post_id", "revision_guid", "created_at", "user_id", "user_display_name", "edit_comment", "text", "content_license", "source_offset"}},
			rows:         [][]any{{site.String(), int64(70), int16(4), int64(10), revision, timestamp, nil, "", nil, "nội dung", nil, int64(840)}},
		},
	}

	if len(acceptedPlans) != len(domainingestion.SupportedTables()) {
		t.Fatalf("acceptedPlans length = %d, want %d", len(acceptedPlans), len(domainingestion.SupportedTables()))
	}

	for _, tt := range tests {
		t.Run(tt.table.String(), func(t *testing.T) {
			plans := acceptedPlans[tt.table]
			if len(plans) != len(tt.rows) {
				t.Fatalf("plan count = %d, want %d", len(plans), len(tt.rows))
			}

			record := domainingestion.NewSourceRecord(tt.table, 840, "<row />", tt.attributes)
			for i, plan := range plans {
				if len(plan.Table) != 1 || plan.Table[0] != tt.destinations[i] {
					t.Errorf("plan %d table = %#v, want %q", i, plan.Table, tt.destinations[i])
				}
				if !reflect.DeepEqual(plan.Columns, tt.columns[i]) {
					t.Errorf("plan %d columns = %#v, want %#v", i, plan.Columns, tt.columns[i])
				}

				row, err := plan.Map(site, record)
				if err != nil {
					t.Fatalf("plan %d Map() error = %v", i, err)
				}
				if len(row) != len(plan.Columns) {
					t.Errorf("plan %d row length = %d, column length = %d", i, len(row), len(plan.Columns))
				}
				if !reflect.DeepEqual(row, tt.rows[i]) {
					t.Errorf("plan %d row = %#v, want %#v", i, row, tt.rows[i])
				}
			}
		})
	}
}

func TestMapperDistinguishesMissingEmptyAndInvalidAttributes(t *testing.T) {
	validVote := map[string]string{
		"Id": "30", "PostId": "10", "VoteTypeId": "2", "CreationDate": "2026-01-02T03:04:05.006",
	}

	tests := []struct {
		name       string
		attributes map[string]string
		wantError  error
		wantText   []string
	}{
		{name: "missing required", attributes: map[string]string{"PostId": "10", "VoteTypeId": "2", "CreationDate": "2026-01-02T03:04:05.006"}, wantError: ErrMissingWriteAttribute, wantText: []string{"table votes", "offset 840", `attribute "Id"`}},
		{name: "present empty integer", attributes: map[string]string{"Id": "", "PostId": "10", "VoteTypeId": "2", "CreationDate": "2026-01-02T03:04:05.006"}, wantError: ErrInvalidWriteAttribute, wantText: []string{"table votes", "offset 840", `attribute "Id"`}},
		{name: "present empty optional integer", attributes: map[string]string{"Id": "30", "PostId": "10", "VoteTypeId": "2", "CreationDate": "2026-01-02T03:04:05.006", "BountyAmount": ""}, wantError: ErrInvalidWriteAttribute, wantText: []string{"table votes", "offset 840", `attribute "BountyAmount"`}},
		{name: "integer overflow", attributes: map[string]string{"Id": "30", "PostId": "10", "VoteTypeId": "32768", "CreationDate": "2026-01-02T03:04:05.006"}, wantError: ErrInvalidWriteAttribute, wantText: []string{"table votes", "offset 840", `attribute "VoteTypeId"`}},
		{name: "optional integer overflow", attributes: map[string]string{"Id": "30", "PostId": "10", "VoteTypeId": "2", "CreationDate": "2026-01-02T03:04:05.006", "BountyAmount": "2147483648"}, wantError: ErrInvalidWriteAttribute, wantText: []string{"table votes", "offset 840", `attribute "BountyAmount"`}},
		{name: "invalid time", attributes: map[string]string{"Id": "30", "PostId": "10", "VoteTypeId": "2", "CreationDate": "2026-01-02T03:04:05Z"}, wantError: ErrInvalidWriteAttribute, wantText: []string{"table votes", "offset 840", `attribute "CreationDate"`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := domainingestion.NewSourceRecord(domainingestion.TableVotes, 840, "secret raw row", tt.attributes)
			_, err := mapVote(domainingestion.Site("stackoverflow.com"), record)
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("mapVote() error = %v, want %v", err, tt.wantError)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("mapVote() error = %q, want substring %q", err, want)
				}
			}
			if strings.Contains(err.Error(), "secret raw row") {
				t.Errorf("mapVote() error exposed the raw row: %q", err)
			}
		})
	}

	record := domainingestion.NewSourceRecord(domainingestion.TableVotes, 840, "<row />", validVote)
	row, err := mapVote(domainingestion.Site("stackoverflow.com"), record)
	if err != nil {
		t.Fatalf("mapVote() error = %v", err)
	}
	if row[4] != nil || row[6] != nil {
		t.Fatalf("optional missing values = %#v, %#v, want nil, nil", row[4], row[6])
	}
}

func TestMapperRejectsInvalidUUIDAndBoolWithLocation(t *testing.T) {
	history := domainingestion.NewSourceRecord(domainingestion.TablePostHistory, 91, "<row />", map[string]string{
		"Id": "1", "PostHistoryTypeId": "2", "PostId": "3", "RevisionGUID": "invalid", "CreationDate": "2026-01-02T03:04:05.006",
	})
	_, err := mapPostHistory(domainingestion.Site("stackoverflow.com"), history)
	if !errors.Is(err, ErrInvalidWriteAttribute) || !strings.Contains(err.Error(), "table post_history offset 91") || !strings.Contains(err.Error(), `attribute "RevisionGUID"`) {
		t.Fatalf("mapPostHistory() error = %v", err)
	}

	badge := domainingestion.NewSourceRecord(domainingestion.TableBadges, 92, "<row />", map[string]string{
		"Id": "1", "UserId": "2", "Name": "Teacher", "Date": "2026-01-02T03:04:05.006", "Class": "1", "TagBased": "yes",
	})
	_, err = mapBadge(domainingestion.Site("stackoverflow.com"), badge)
	if !errors.Is(err, ErrInvalidWriteAttribute) || !strings.Contains(err.Error(), "table badges offset 92") || !strings.Contains(err.Error(), `attribute "TagBased"`) {
		t.Fatalf("mapBadge() error = %v", err)
	}

	tag := domainingestion.NewSourceRecord(domainingestion.TableTags, 93, "<row />", map[string]string{
		"Id": "1", "TagName": "go", "Count": "2",
	})
	row, err := mapTag(domainingestion.Site("stackoverflow.com"), tag)
	if err != nil {
		t.Fatalf("mapTag() error = %v", err)
	}
	if row[4] != nil || row[5] != nil || row[6] != nil || row[7] != nil {
		t.Fatalf("mapTag() optional values = %#v, %#v, %#v, %#v, want nil values", row[4], row[5], row[6], row[7])
	}
}

func TestPostMapperRejectsPresentEmptyDefaultedCounterWithLocation(t *testing.T) {
	record := domainingestion.NewSourceRecord(domainingestion.TablePosts, 94, "<row />", map[string]string{
		"Id": "1", "PostTypeId": "1", "CreationDate": "2026-01-02T03:04:05.006", "Score": "0", "ViewCount": "", "LastActivityDate": "2026-01-02T03:04:05.006", "Body": "body",
	})

	_, err := mapPost(domainingestion.Site("stackoverflow.com"), record)
	if !errors.Is(err, ErrInvalidWriteAttribute) || !strings.Contains(err.Error(), "table posts offset 94") || !strings.Contains(err.Error(), `attribute "ViewCount"`) {
		t.Fatalf("mapPost() error = %v", err)
	}
}

func TestTagsOrEmptyPreservesSourceSemantics(t *testing.T) {
	tests := []struct {
		name       string
		attributes map[string]string
		want       []string
		wantError  bool
	}{
		{name: "missing", attributes: map[string]string{}, want: []string{}},
		{name: "empty", attributes: map[string]string{"Tags": ""}, want: []string{}},
		{name: "ordered list", attributes: map[string]string{"Tags": "<go><post gre sql><go>"}, want: []string{"go", "post gre sql", "go"}},
		{name: "missing opening boundary", attributes: map[string]string{"Tags": "go>"}, wantError: true},
		{name: "missing closing boundary", attributes: map[string]string{"Tags": "<go"}, wantError: true},
		{name: "empty member", attributes: map[string]string{"Tags": "<go><><sql>"}, wantError: true},
		{name: "broken delimiter", attributes: map[string]string{"Tags": "<go>broken<sql>"}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := domainingestion.NewSourceRecord(domainingestion.TablePosts, 1, "<row />", tt.attributes)
			got, err := tagsOrEmpty(record, "Tags")
			if tt.wantError {
				if !errors.Is(err, ErrInvalidWriteAttribute) {
					t.Fatalf("tagsOrEmpty() error = %v, want invalid attribute", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("tagsOrEmpty() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("tagsOrEmpty() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
