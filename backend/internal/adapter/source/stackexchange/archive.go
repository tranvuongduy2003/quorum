package stackexchange

import (
	"context"
	"fmt"
	domainingestion "quorum/internal/domain/ingestion"
	"quorum/internal/usecase/ingestion"

	"github.com/bodgit/sevenzip"
)

var tableToMember = map[domainingestion.Table]string{
	"badges":       "Badges.xml",
	"comments":     "Comments.xml",
	"post_history": "PostHistory.xml",
	"post_links":   "PostLinks.xml",
	"posts":        "Posts.xml",
	"tags":         "Tags.xml",
	"users":        "Users.xml",
	"votes":        "Votes.xml",
}

var tableToRoot = map[domainingestion.Table]string{
	domainingestion.TableBadges:      "badges",
	domainingestion.TableComments:    "comments",
	domainingestion.TablePostHistory: "posthistory",
	domainingestion.TablePostLinks:   "postlinks",
	domainingestion.TablePosts:       "posts",
	domainingestion.TableTags:        "tags",
	domainingestion.TableUsers:       "users",
	domainingestion.TableVotes:       "votes",
}

type Factory struct{}

func NewFactory() Factory {
	return Factory{}
}
func (Factory) Open(path string) (ingestion.Archive, error) {
	reader, err := sevenzip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open Stack Exchange archive: %w", err)
	}

	nameToFileMap := make(map[string]*sevenzip.File)

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		nameToFileMap[file.Name] = file
	}

	return &Archive{reader, nameToFileMap}, nil
}

type Archive struct {
	reader *sevenzip.ReadCloser
	files  map[string]*sevenzip.File
}

func (a *Archive) ValidateTables(tables []domainingestion.Table) error {
	for _, table := range tables {
		member, ok := tableToMember[table]
		if !ok {
			return ingestion.SourceError{
				Table:  table,
				Offset: 0,
				Err:    fmt.Errorf("%w: unsupported table", ingestion.ErrArchiveMemberMissing),
			}
		}

		_, ok = a.files[member]

		if !ok {
			return ingestion.SourceError{
				Table:  table,
				Offset: 0,
				Err:    fmt.Errorf("%w: %s", ingestion.ErrArchiveMemberMissing, member),
			}
		}
	}

	return nil
}
func (a *Archive) OpenTable(ctx context.Context, table domainingestion.Table, maxRecordBytes int) (ingestion.RecordStream, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	tableMember, ok := tableToMember[table]

	if !ok {
		return nil, ingestion.SourceError{
			Table:  table,
			Offset: 0,
			Err:    ingestion.ErrArchiveMemberMissing,
		}
	}

	file, ok := a.files[tableMember]
	if !ok {
		return nil, ingestion.SourceError{
			Table:  table,
			Offset: 0,
			Err:    fmt.Errorf("%w: %s", ingestion.ErrArchiveMemberMissing, tableMember),
		}
	}
	member, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open Stack Exchange archive: %w", err)
	}

	recordStream := newRecordStream(table, member, maxRecordBytes)

	return recordStream, nil
}
func (a *Archive) Close() error {
	err := a.reader.Close()
	if err != nil {
		return err
	}
	return nil
}
