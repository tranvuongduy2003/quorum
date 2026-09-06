BEGIN;

CREATE TABLE ingest_quarantine (
    id bigserial PRIMARY KEY,
    site text NOT NULL,
    source_table text NOT NULL,
    source_offset bigint NOT NULL CHECK (source_offset >= 0),
    raw_row text NOT NULL,
    reason_code text NOT NULL,
    found_at timestamptz NOT NULL
);

CREATE INDEX ingest_quarantine_review_idx
ON ingest_quarantine (site, source_table, reason_code, found_at DESC);

COMMIT;
