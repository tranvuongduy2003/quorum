BEGIN;

CREATE SCHEMA copy_benchmark;

CREATE TABLE copy_benchmark.votes (
    site text NOT NULL,
    id bigint NOT NULL,
    post_id bigint NOT NULL,
    vote_type_id smallint NOT NULL,
    user_id bigint,
    created_at timestamptz NOT NULL,
    bounty_amount integer,
    source_offset bigint NOT NULL CHECK (source_offset >= 0),
    PRIMARY KEY (id)
);

COMMIT;
