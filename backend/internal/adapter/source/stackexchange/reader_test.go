package stackexchange

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	domainingestion "quorum/internal/domain/ingestion"
	usecaseingestion "quorum/internal/usecase/ingestion"
)

func TestRecordStreamPreservesRawAndTracksDecompressedOffsets(t *testing.T) {
	input := "<?xml version=\"1.0\"?>\r\n<posts>\r\n  <row Id=\"1\" Title=\"Hello\" />\r\n</posts>"
	stream := newRecordStream(domainingestion.TablePosts, io.NopCloser(strings.NewReader(input)), 1024)

	record, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if want := int64(len("<?xml version=\"1.0\"?>\r\n<posts>\r\n")); record.Offset != want {
		t.Fatalf("Offset = %d, want %d", record.Offset, want)
	}
	if want := "  <row Id=\"1\" Title=\"Hello\" />"; record.Raw != want {
		t.Fatalf("Raw = %q, want %q", record.Raw, want)
	}
	if value, ok := record.Attribute("Title"); !ok || value != "Hello" {
		t.Fatalf("Attribute(Title) = %q, %t", value, ok)
	}

	_, err = stream.Next(context.Background())
	if !errors.Is(err, io.EOF) {
		t.Fatalf("final Next() error = %v, want EOF", err)
	}
}

func TestRecordStreamEnforcesInclusiveRecordLimit(t *testing.T) {
	maxBytes := 1024
	row := "<row A=\"" + strings.Repeat("x", maxBytes-len("<row A=\"\" />")) + "\" />"
	stream := newRecordStream(domainingestion.TablePosts, io.NopCloser(strings.NewReader(row+"\r\n")), maxBytes)

	if _, err := stream.Next(context.Background()); err != nil {
		t.Fatalf("exact-limit Next() error = %v", err)
	}

	overLimit := row[:len(row)-3] + "x\" />\n"
	stream = newRecordStream(domainingestion.TablePosts, io.NopCloser(strings.NewReader(overLimit)), maxBytes)
	_, err := stream.Next(context.Background())
	var sourceErr usecaseingestion.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("over-limit Next() error = %T %v, want SourceError", err, err)
	}
	if sourceErr.Offset != 0 || !errors.Is(sourceErr, domainingestion.ErrRecordTooLarge) {
		t.Fatalf("over-limit error = %#v", sourceErr)
	}
}

func TestRecordStreamRejectsUnexpectedSourceContent(t *testing.T) {
	stream := newRecordStream(domainingestion.TablePosts, io.NopCloser(strings.NewReader("<questions>\n")), 1024)

	_, err := stream.Next(context.Background())
	var sourceErr usecaseingestion.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("Next() error = %T %v, want SourceError", err, err)
	}
	if !errors.Is(sourceErr, domainingestion.ErrUnexpectedSourceLine) {
		t.Fatalf("Next() error = %v, want unexpected source line", err)
	}
}

func TestTableMappingsMatchStackExchangeNames(t *testing.T) {
	wantMembers := map[domainingestion.Table]string{
		domainingestion.TableBadges:      "Badges.xml",
		domainingestion.TableComments:    "Comments.xml",
		domainingestion.TablePostHistory: "PostHistory.xml",
		domainingestion.TablePostLinks:   "PostLinks.xml",
		domainingestion.TablePosts:       "Posts.xml",
		domainingestion.TableTags:        "Tags.xml",
		domainingestion.TableUsers:       "Users.xml",
		domainingestion.TableVotes:       "Votes.xml",
	}
	wantRoots := map[domainingestion.Table]string{
		domainingestion.TableBadges:      "badges",
		domainingestion.TableComments:    "comments",
		domainingestion.TablePostHistory: "posthistory",
		domainingestion.TablePostLinks:   "postlinks",
		domainingestion.TablePosts:       "posts",
		domainingestion.TableTags:        "tags",
		domainingestion.TableUsers:       "users",
		domainingestion.TableVotes:       "votes",
	}

	for table, member := range wantMembers {
		if got := tableToMember[table]; got != member {
			t.Fatalf("member for %s = %q, want %q", table, got, member)
		}
		if got := tableToRoot[table]; got != wantRoots[table] {
			t.Fatalf("root for %s = %q, want %q", table, got, wantRoots[table])
		}
	}
}
