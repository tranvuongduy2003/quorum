package ingestion

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainingestion "quorum/internal/domain/ingestion"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrMissingWriteAttribute = errors.New("required write attribute is missing")
var ErrInvalidWriteAttribute = errors.New("write attribute has an invalid value")

const sourceTimestampLayout = "2006-01-02T15:04:05.000"

type rowMapper func(domainingestion.Site, domainingestion.SourceRecord) ([]any, error)

type destinationPlan struct {
	Table   pgx.Identifier
	Columns []string
	Map     rowMapper
}

var acceptedPlans = map[domainingestion.Table][]destinationPlan{
	domainingestion.TableUsers: {
		{
			Table:   pgx.Identifier{"users"},
			Columns: []string{"site", "id", "reputation", "created_at", "display_name", "last_access_at", "website_url", "location", "about_me", "views", "up_votes", "down_votes", "profile_image_url", "email_hash", "account_id", "source_offset"},
			Map:     mapUser,
		},
	},
	domainingestion.TablePosts: {
		{
			Table:   pgx.Identifier{"posts"},
			Columns: []string{"site", "id", "post_type_id", "accepted_answer_id", "parent_id", "created_at", "deleted_at", "score", "view_count", "owner_user_id", "owner_display_name", "last_editor_user_id", "last_editor_display_name", "last_edit_at", "last_activity_at", "title", "tags", "answer_count", "comment_count", "favorite_count", "closed_at", "community_owned_at", "content_license", "source_offset"},
			Map:     mapPost,
		},
		{
			Table:   pgx.Identifier{"post_bodies"},
			Columns: []string{"site", "post_id", "body"},
			Map:     mapPostBody,
		},
	},
	domainingestion.TableComments: {
		{
			Table:   pgx.Identifier{"comments"},
			Columns: []string{"site", "id", "post_id", "score", "text", "created_at", "user_display_name", "user_id", "content_license", "source_offset"},
			Map:     mapComment,
		},
	},
	domainingestion.TableVotes: {
		{
			Table:   pgx.Identifier{"votes"},
			Columns: []string{"site", "id", "post_id", "vote_type_id", "user_id", "created_at", "bounty_amount", "source_offset"},
			Map:     mapVote,
		},
	},
	domainingestion.TableBadges: {
		{
			Table:   pgx.Identifier{"badges"},
			Columns: []string{"site", "id", "user_id", "name", "awarded_at", "class", "tag_based", "source_offset"},
			Map:     mapBadge,
		},
	},
	domainingestion.TableTags: {
		{
			Table:   pgx.Identifier{"tags"},
			Columns: []string{"site", "id", "name", "usage_count", "excerpt_post_id", "wiki_post_id", "moderator_only", "required", "source_offset"},
			Map:     mapTag,
		},
	},
	domainingestion.TablePostLinks: {
		{
			Table:   pgx.Identifier{"post_links"},
			Columns: []string{"site", "id", "created_at", "post_id", "related_post_id", "link_type_id", "source_offset"},
			Map:     mapPostLink,
		},
	},
	domainingestion.TablePostHistory: {
		{
			Table:   pgx.Identifier{"post_history"},
			Columns: []string{"site", "id", "history_type_id", "post_id", "revision_guid", "created_at", "user_id", "user_display_name", "edit_comment", "text", "content_license", "source_offset"},
			Map:     mapPostHistory,
		},
	},
}

var quarantinePlan = destinationPlan{
	Table: pgx.Identifier{"ingest_quarantine"},
	Columns: []string{
		"site",
		"source_table",
		"source_offset",
		"raw_row",
		"reason_code",
		"found_at",
	},
	Map: nil,
}

func requiredText(record domainingestion.SourceRecord, name string) (string, error) {
	value, ok := record.Attributes[name]
	if !ok {
		return "", ErrMissingWriteAttribute
	}

	return value, nil
}

func optionalText(record domainingestion.SourceRecord, name string) any {
	value, ok := record.Attributes[name]
	if !ok {
		return nil
	}

	return value
}

func requiredInt64(record domainingestion.SourceRecord, name string) (int64, error) {
	value, err := requiredText(record, name)
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return parsed, nil
}
func requiredInt32(record domainingestion.SourceRecord, name string) (int32, error) {
	value, err := requiredText(record, name)
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return int32(parsed), nil
}
func requiredInt16(record domainingestion.SourceRecord, name string) (int16, error) {
	value, err := requiredText(record, name)
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return int16(parsed), nil
}
func optionalInt64(record domainingestion.SourceRecord, name string) (any, error) {
	value := optionalText(record, name)
	if value == nil {
		return nil, nil
	}

	parsed, err := strconv.ParseInt(value.(string), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return parsed, nil
}
func optionalInt32(record domainingestion.SourceRecord, name string) (any, error) {
	value := optionalText(record, name)
	if value == nil {
		return nil, nil
	}

	parsed, err := strconv.ParseInt(value.(string), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return int32(parsed), nil
}
func int64OrZero(record domainingestion.SourceRecord, name string) (int64, error) {
	value := optionalText(record, name)
	if value == nil {
		return 0, nil
	}

	parsed, err := strconv.ParseInt(value.(string), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return parsed, nil
}
func int32OrZero(record domainingestion.SourceRecord, name string) (int32, error) {
	value := optionalText(record, name)
	if value == nil {
		return 0, nil
	}

	parsed, err := strconv.ParseInt(value.(string), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return int32(parsed), nil
}
func requiredTime(record domainingestion.SourceRecord, name string) (time.Time, error) {
	value, err := requiredText(record, name)
	if err != nil {
		return time.Time{}, err
	}

	parsed, err := time.ParseInLocation(sourceTimestampLayout, value, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return parsed, nil
}
func optionalTime(record domainingestion.SourceRecord, name string) (any, error) {
	value := optionalText(record, name)
	if value == nil {
		return nil, nil
	}

	parsed, err := time.ParseInLocation(sourceTimestampLayout, value.(string), time.UTC)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return parsed, nil
}
func requiredBool(record domainingestion.SourceRecord, name string) (bool, error) {
	value, err := requiredText(record, name)
	if err != nil {
		return false, err
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return parsed, nil
}
func optionalBool(record domainingestion.SourceRecord, name string) (any, error) {
	value := optionalText(record, name)
	if value == nil {
		return nil, nil
	}

	parsed, err := strconv.ParseBool(value.(string))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return parsed, nil
}
func requiredUUID(record domainingestion.SourceRecord, name string) (uuid.UUID, error) {
	value, err := requiredText(record, name)
	if err != nil {
		return uuid.UUID{}, err
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("%w: %w", ErrInvalidWriteAttribute, err)
	}

	return parsed, nil
}
func tagsOrEmpty(record domainingestion.SourceRecord, name string) ([]string, error) {
	raw, ok := record.Attributes[name]
	if !ok || raw == "" {
		return []string{}, nil
	}

	if len(raw) < 2 || raw[0] != '<' || raw[len(raw)-1] != '>' {
		return nil, ErrInvalidWriteAttribute
	}

	parts := strings.Split(raw[1:len(raw)-1], "><")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || strings.ContainsAny(part, "<>") {
			return nil, ErrInvalidWriteAttribute
		}
		tags = append(tags, part)
	}

	return tags, nil
}

func attributeError(record domainingestion.SourceRecord, name string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("table %s offset %d attribute %q: %w", record.Table, record.Offset, name, err)
}

func mapUser(site domainingestion.Site, record domainingestion.SourceRecord) ([]any, error) {
	id, err := requiredInt64(record, "Id")
	if err != nil {
		return nil, attributeError(record, "Id", err)
	}
	reputation, err := requiredInt32(record, "Reputation")
	if err != nil {
		return nil, attributeError(record, "Reputation", err)
	}
	createdAt, err := requiredTime(record, "CreationDate")
	if err != nil {
		return nil, attributeError(record, "CreationDate", err)
	}
	displayName, err := requiredText(record, "DisplayName")
	if err != nil {
		return nil, attributeError(record, "DisplayName", err)
	}
	lastAccessAt, err := optionalTime(record, "LastAccessDate")
	if err != nil {
		return nil, attributeError(record, "LastAccessDate", err)
	}
	websiteURL := optionalText(record, "WebsiteUrl")
	location := optionalText(record, "Location")
	aboutMe := optionalText(record, "AboutMe")
	views, err := requiredInt32(record, "Views")
	if err != nil {
		return nil, attributeError(record, "Views", err)
	}
	upVotes, err := requiredInt32(record, "UpVotes")
	if err != nil {
		return nil, attributeError(record, "UpVotes", err)
	}
	downVotes, err := requiredInt32(record, "DownVotes")
	if err != nil {
		return nil, attributeError(record, "DownVotes", err)
	}
	profileImageURL := optionalText(record, "ProfileImageUrl")
	emailHash := optionalText(record, "EmailHash")
	accountID, err := optionalInt64(record, "AccountId")
	if err != nil {
		return nil, attributeError(record, "AccountId", err)
	}

	return []any{
		site.String(),
		id,
		reputation,
		createdAt,
		displayName,
		lastAccessAt,
		websiteURL,
		location,
		aboutMe,
		views,
		upVotes,
		downVotes,
		profileImageURL,
		emailHash,
		accountID,
		record.Offset,
	}, nil
}

func mapPost(site domainingestion.Site, record domainingestion.SourceRecord) ([]any, error) {
	id, err := requiredInt64(record, "Id")
	if err != nil {
		return nil, attributeError(record, "Id", err)
	}
	postTypeID, err := requiredInt16(record, "PostTypeId")
	if err != nil {
		return nil, attributeError(record, "PostTypeId", err)
	}
	acceptedAnswerID, err := optionalInt64(record, "AcceptedAnswerId")
	if err != nil {
		return nil, attributeError(record, "AcceptedAnswerId", err)
	}
	parentID, err := optionalInt64(record, "ParentId")
	if err != nil {
		return nil, attributeError(record, "ParentId", err)
	}
	createdAt, err := requiredTime(record, "CreationDate")
	if err != nil {
		return nil, attributeError(record, "CreationDate", err)
	}
	deletedAt, err := optionalTime(record, "DeletionDate")
	if err != nil {
		return nil, attributeError(record, "DeletionDate", err)
	}
	score, err := requiredInt32(record, "Score")
	if err != nil {
		return nil, attributeError(record, "Score", err)
	}
	viewCount, err := int64OrZero(record, "ViewCount")
	if err != nil {
		return nil, attributeError(record, "ViewCount", err)
	}
	ownerUserID, err := optionalInt64(record, "OwnerUserId")
	if err != nil {
		return nil, attributeError(record, "OwnerUserId", err)
	}
	ownerDisplayName := optionalText(record, "OwnerDisplayName")
	lastEditorUserID, err := optionalInt64(record, "LastEditorUserId")
	if err != nil {
		return nil, attributeError(record, "LastEditorUserId", err)
	}
	lastEditorDisplayName := optionalText(record, "LastEditorDisplayName")
	lastEditAt, err := optionalTime(record, "LastEditDate")
	if err != nil {
		return nil, attributeError(record, "LastEditDate", err)
	}
	lastActivityAt, err := requiredTime(record, "LastActivityDate")
	if err != nil {
		return nil, attributeError(record, "LastActivityDate", err)
	}
	title := optionalText(record, "Title")
	tags, err := tagsOrEmpty(record, "Tags")
	if err != nil {
		return nil, attributeError(record, "Tags", err)
	}
	answerCount, err := int32OrZero(record, "AnswerCount")
	if err != nil {
		return nil, attributeError(record, "AnswerCount", err)
	}
	commentCount, err := int32OrZero(record, "CommentCount")
	if err != nil {
		return nil, attributeError(record, "CommentCount", err)
	}
	favoriteCount, err := optionalInt32(record, "FavoriteCount")
	if err != nil {
		return nil, attributeError(record, "FavoriteCount", err)
	}
	closedAt, err := optionalTime(record, "ClosedDate")
	if err != nil {
		return nil, attributeError(record, "ClosedDate", err)
	}
	communityOwnedAt, err := optionalTime(record, "CommunityOwnedDate")
	if err != nil {
		return nil, attributeError(record, "CommunityOwnedDate", err)
	}
	contentLicense := optionalText(record, "ContentLicense")

	return []any{
		site.String(),
		id,
		postTypeID,
		acceptedAnswerID,
		parentID,
		createdAt,
		deletedAt,
		score,
		viewCount,
		ownerUserID,
		ownerDisplayName,
		lastEditorUserID,
		lastEditorDisplayName,
		lastEditAt,
		lastActivityAt,
		title,
		tags,
		answerCount,
		commentCount,
		favoriteCount,
		closedAt,
		communityOwnedAt,
		contentLicense,
		record.Offset,
	}, nil
}

func mapPostBody(site domainingestion.Site, record domainingestion.SourceRecord) ([]any, error) {
	postID, err := requiredInt64(record, "Id")
	if err != nil {
		return nil, attributeError(record, "Id", err)
	}
	body, err := requiredText(record, "Body")
	if err != nil {
		return nil, attributeError(record, "Body", err)
	}

	return []any{
		site.String(),
		postID,
		body,
	}, nil
}

func mapComment(site domainingestion.Site, record domainingestion.SourceRecord) ([]any, error) {
	id, err := requiredInt64(record, "Id")
	if err != nil {
		return nil, attributeError(record, "Id", err)
	}
	postID, err := requiredInt64(record, "PostId")
	if err != nil {
		return nil, attributeError(record, "PostId", err)
	}
	score, err := requiredInt32(record, "Score")
	if err != nil {
		return nil, attributeError(record, "Score", err)
	}
	text, err := requiredText(record, "Text")
	if err != nil {
		return nil, attributeError(record, "Text", err)
	}
	createdAt, err := requiredTime(record, "CreationDate")
	if err != nil {
		return nil, attributeError(record, "CreationDate", err)
	}
	userDisplayName := optionalText(record, "UserDisplayName")
	userID, err := optionalInt64(record, "UserId")
	if err != nil {
		return nil, attributeError(record, "UserId", err)
	}
	contentLicense := optionalText(record, "ContentLicense")

	return []any{
		site.String(),
		id,
		postID,
		score,
		text,
		createdAt,
		userDisplayName,
		userID,
		contentLicense,
		record.Offset,
	}, nil
}

func mapVote(site domainingestion.Site, record domainingestion.SourceRecord) ([]any, error) {
	id, err := requiredInt64(record, "Id")
	if err != nil {
		return nil, attributeError(record, "Id", err)
	}
	postID, err := requiredInt64(record, "PostId")
	if err != nil {
		return nil, attributeError(record, "PostId", err)
	}
	voteTypeID, err := requiredInt16(record, "VoteTypeId")
	if err != nil {
		return nil, attributeError(record, "VoteTypeId", err)
	}
	userID, err := optionalInt64(record, "UserId")
	if err != nil {
		return nil, attributeError(record, "UserId", err)
	}
	createdAt, err := requiredTime(record, "CreationDate")
	if err != nil {
		return nil, attributeError(record, "CreationDate", err)
	}
	bountyAmount, err := optionalInt32(record, "BountyAmount")
	if err != nil {
		return nil, attributeError(record, "BountyAmount", err)
	}

	return []any{
		site.String(),
		id,
		postID,
		voteTypeID,
		userID,
		createdAt,
		bountyAmount,
		record.Offset,
	}, nil
}

func mapBadge(site domainingestion.Site, record domainingestion.SourceRecord) ([]any, error) {
	id, err := requiredInt64(record, "Id")
	if err != nil {
		return nil, attributeError(record, "Id", err)
	}
	userID, err := requiredInt64(record, "UserId")
	if err != nil {
		return nil, attributeError(record, "UserId", err)
	}
	name, err := requiredText(record, "Name")
	if err != nil {
		return nil, attributeError(record, "Name", err)
	}
	awardedAt, err := requiredTime(record, "Date")
	if err != nil {
		return nil, attributeError(record, "Date", err)
	}
	class, err := requiredInt16(record, "Class")
	if err != nil {
		return nil, attributeError(record, "Class", err)
	}
	tagBased, err := requiredBool(record, "TagBased")
	if err != nil {
		return nil, attributeError(record, "TagBased", err)
	}

	return []any{
		site.String(),
		id,
		userID,
		name,
		awardedAt,
		class,
		tagBased,
		record.Offset,
	}, nil
}

func mapTag(site domainingestion.Site, record domainingestion.SourceRecord) ([]any, error) {
	id, err := requiredInt64(record, "Id")
	if err != nil {
		return nil, attributeError(record, "Id", err)
	}
	name, err := requiredText(record, "TagName")
	if err != nil {
		return nil, attributeError(record, "TagName", err)
	}
	usageCount, err := requiredInt32(record, "Count")
	if err != nil {
		return nil, attributeError(record, "Count", err)
	}
	excerptPostID, err := optionalInt64(record, "ExcerptPostId")
	if err != nil {
		return nil, attributeError(record, "ExcerptPostId", err)
	}
	wikiPostID, err := optionalInt64(record, "WikiPostId")
	if err != nil {
		return nil, attributeError(record, "WikiPostId", err)
	}
	moderatorOnly, err := optionalBool(record, "IsModeratorOnly")
	if err != nil {
		return nil, attributeError(record, "IsModeratorOnly", err)
	}
	isRequired, err := optionalBool(record, "IsRequired")
	if err != nil {
		return nil, attributeError(record, "IsRequired", err)
	}

	return []any{
		site.String(),
		id,
		name,
		usageCount,
		excerptPostID,
		wikiPostID,
		moderatorOnly,
		isRequired,
		record.Offset,
	}, nil
}

func mapPostLink(site domainingestion.Site, record domainingestion.SourceRecord) ([]any, error) {
	id, err := requiredInt64(record, "Id")
	if err != nil {
		return nil, attributeError(record, "Id", err)
	}
	createdAt, err := requiredTime(record, "CreationDate")
	if err != nil {
		return nil, attributeError(record, "CreationDate", err)
	}
	postID, err := requiredInt64(record, "PostId")
	if err != nil {
		return nil, attributeError(record, "PostId", err)
	}
	relatedPostID, err := requiredInt64(record, "RelatedPostId")
	if err != nil {
		return nil, attributeError(record, "RelatedPostId", err)
	}
	linkTypeID, err := requiredInt16(record, "LinkTypeId")
	if err != nil {
		return nil, attributeError(record, "LinkTypeId", err)
	}

	return []any{
		site.String(),
		id,
		createdAt,
		postID,
		relatedPostID,
		linkTypeID,
		record.Offset,
	}, nil
}

func mapPostHistory(site domainingestion.Site, record domainingestion.SourceRecord) ([]any, error) {
	id, err := requiredInt64(record, "Id")
	if err != nil {
		return nil, attributeError(record, "Id", err)
	}
	historyTypeID, err := requiredInt16(record, "PostHistoryTypeId")
	if err != nil {
		return nil, attributeError(record, "PostHistoryTypeId", err)
	}
	postID, err := requiredInt64(record, "PostId")
	if err != nil {
		return nil, attributeError(record, "PostId", err)
	}
	revisionGUID, err := requiredUUID(record, "RevisionGUID")
	if err != nil {
		return nil, attributeError(record, "RevisionGUID", err)
	}
	createdAt, err := requiredTime(record, "CreationDate")
	if err != nil {
		return nil, attributeError(record, "CreationDate", err)
	}
	userID, err := optionalInt64(record, "UserId")
	if err != nil {
		return nil, attributeError(record, "UserId", err)
	}
	userDisplayName := optionalText(record, "UserDisplayName")
	editComment := optionalText(record, "Comment")
	text := optionalText(record, "Text")
	contentLicense := optionalText(record, "ContentLicense")

	return []any{
		site.String(),
		id,
		historyTypeID,
		postID,
		revisionGUID,
		createdAt,
		userID,
		userDisplayName,
		editComment,
		text,
		contentLicense,
		record.Offset,
	}, nil
}

func mapQuarantineRow(record domainingestion.QuarantineRecord) []any {
	return []any{
		record.Site.String(),
		record.Source.Table.String(),
		record.Source.Offset,
		record.Source.Raw,
		string(record.Reason),
		record.FoundAt,
	}
}
