BEGIN;

CREATE TABLE ingest_checkpoints (
    site text NOT NULL,
    source_table text NOT NULL,
    archive_id text NOT NULL,
    source_offset bigint NOT NULL CHECK (source_offset >= 0),
    confirmed_count bigint NOT NULL CHECK (confirmed_count >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (site, source_table)
);

COMMIT;
