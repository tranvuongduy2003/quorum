BEGIN;

CREATE TABLE
    users (
        site text NOT NULL,
        id bigint NOT NULL,
        reputation integer NOT NULL DEFAULT 1,
        created_at timestamptz NOT NULL,
        display_name text NOT NULL,
        last_access_at timestamptz,
        website_url text,
        location text,
        about_me text,
        views integer NOT NULL,
        up_votes integer NOT NULL,
        down_votes integer NOT NULL,
        profile_image_url text,
        email_hash text,
        account_id bigint,
        source_offset bigint NOT NULL CHECK (source_offset >= 0),
        PRIMARY KEY (site, id)
    );

CREATE TABLE
    posts (
        site text NOT NULL,
        id bigint NOT NULL,
        post_type_id smallint NOT NULL,
        accepted_answer_id bigint,
        parent_id bigint,
        created_at timestamptz NOT NULL,
        deleted_at timestamptz,
        score integer NOT NULL DEFAULT 0,
        view_count bigint NOT NULL DEFAULT 0,
        owner_user_id bigint,
        owner_display_name text,
        last_editor_user_id bigint,
        last_editor_display_name text,
        last_edit_at timestamptz,
        last_activity_at timestamptz NOT NULL,
        title text,
        tags text[] NOT NULL DEFAULT '{}',
        answer_count integer NOT NULL DEFAULT 0,
        comment_count integer NOT NULL DEFAULT 0,
        favorite_count integer,
        closed_at timestamptz,
        community_owned_at timestamptz,
        content_license text,
        source_offset bigint NOT NULL CHECK (source_offset >= 0),
        PRIMARY KEY (site, id)
    )
WITH
    (fillfactor = 85);

CREATE TABLE
    post_bodies (
        site text NOT NULL,
        post_id bigint NOT NULL,
        body text NOT NULL,
        PRIMARY KEY (site, post_id),
        FOREIGN KEY (site, post_id) REFERENCES posts (site, id) ON DELETE CASCADE
    );

ALTER TABLE post_bodies
ALTER COLUMN body
SET
    COMPRESSION lz4;

CREATE TABLE
    comments (
        site text NOT NULL,
        id bigint NOT NULL,
        post_id bigint NOT NULL,
        score integer NOT NULL DEFAULT 0,
        text text NOT NULL,
        created_at timestamptz NOT NULL,
        user_display_name text,
        user_id bigint,
        content_license text,
        source_offset bigint NOT NULL CHECK (source_offset >= 0),
        PRIMARY KEY (site, id, created_at)
    )
PARTITION BY
    RANGE (created_at);

CREATE TABLE
    votes (
        site text NOT NULL,
        id bigint NOT NULL,
        post_id bigint NOT NULL,
        vote_type_id smallint NOT NULL,
        user_id bigint,
        created_at timestamptz NOT NULL,
        bounty_amount integer,
        source_offset bigint NOT NULL CHECK (source_offset >= 0),
        PRIMARY KEY (site, id, created_at)
    )
PARTITION BY
    RANGE (created_at);

CREATE TABLE
    badges (
        site text NOT NULL,
        id bigint NOT NULL,
        user_id bigint NOT NULL,
        name text NOT NULL,
        awarded_at timestamptz NOT NULL,
        class smallint NOT NULL,
        tag_based boolean NOT NULL,
        source_offset bigint NOT NULL CHECK (source_offset >= 0),
        PRIMARY KEY (site, id)
    );

CREATE TABLE
    tags (
        site text NOT NULL,
        id bigint NOT NULL,
        name text NOT NULL,
        usage_count integer NOT NULL,
        excerpt_post_id bigint,
        wiki_post_id bigint,
        moderator_only boolean,
        required boolean,
        source_offset bigint NOT NULL CHECK (source_offset >= 0),
        PRIMARY KEY (site, id)
    );

CREATE TABLE
    post_links (
        site text NOT NULL,
        id bigint NOT NULL,
        created_at timestamptz NOT NULL,
        post_id bigint NOT NULL,
        related_post_id bigint NOT NULL,
        link_type_id smallint NOT NULL,
        source_offset bigint NOT NULL CHECK (source_offset >= 0),
        PRIMARY KEY (site, id)
    );

CREATE TABLE
    post_history (
        site text NOT NULL,
        id bigint NOT NULL,
        history_type_id smallint NOT NULL,
        post_id bigint NOT NULL,
        revision_guid uuid NOT NULL,
        created_at timestamptz NOT NULL,
        user_id bigint,
        user_display_name text,
        edit_comment text,
        text text,
        content_license text,
        source_offset bigint NOT NULL CHECK (source_offset >= 0),
        PRIMARY KEY (site, id)
    );

DO $$
    DECLARE
        year_start date;
    BEGIN
        FOR year_start IN
            SELECT generate_series(date '2008-01-01', date '2026-01-01', interval '1 year')::date
            LOOP
                EXECUTE format(
                        'CREATE TABLE comments_%s PARTITION OF comments FOR VALUES FROM (%L) TO (%L)',
                        to_char(year_start, 'YYYY'),
                        year_start,
                        year_start + interval '1 year'
                        );
            END LOOP;
    END
$$;

CREATE TABLE
    comments_default PARTITION OF comments DEFAULT;

DO $$
    DECLARE
        month_start date;
    BEGIN
        FOR month_start IN
            SELECT generate_series(date '2008-01-01', date '2026-12-01', interval '1 month')::date
            LOOP
                EXECUTE format(
                        'CREATE TABLE votes_%s PARTITION OF votes FOR VALUES FROM (%L) TO (%L)',
                        to_char(month_start, 'YYYY_MM'),
                        month_start,
                        month_start + interval '1 month'
                        );
            END LOOP;
    END
$$;

CREATE TABLE
    votes_default PARTITION OF votes DEFAULT;

COMMIT;