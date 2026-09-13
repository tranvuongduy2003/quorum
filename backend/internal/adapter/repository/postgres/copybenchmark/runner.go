package copybenchmark

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	usecasecopybenchmark "quorum/internal/usecase/copybenchmark"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const voteColumns = "site, id, post_id, vote_type_id, user_id, created_at, bounty_amount, source_offset"

var _ usecasecopybenchmark.Runner = Runner{}

type Runner struct {
	pool *pgxpool.Pool
}

type voteRow struct {
	Site         string
	ID           int64
	PostID       int64
	VoteTypeID   int16
	UserID       *int64
	CreatedAt    time.Time
	BountyAmount *int32
	SourceOffset int64
}

type producerResult struct {
	Err       error
	Intrinsic bool
}

func NewRunner(pool *pgxpool.Pool) Runner {
	return Runner{pool: pool}
}

func (r Runner) Prepare(ctx context.Context) error {
	if _, err := r.pool.Exec(ctx, "TRUNCATE copy_benchmark.votes"); err != nil {
		return fmt.Errorf("truncate benchmark votes: %w", err)
	}
	rows, err := r.pool.Query(ctx, "SELECT "+voteColumns+" FROM copy_benchmark.votes LIMIT 0")
	if err != nil {
		return fmt.Errorf("inspect benchmark votes: %w", err)
	}
	fields := rows.FieldDescriptions()
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect benchmark votes rows: %w", err)
	}
	expectedNames := []string{"site", "id", "post_id", "vote_type_id", "user_id", "created_at", "bounty_amount", "source_offset"}
	expectedOIDs := []uint32{pgtype.TextOID, pgtype.Int8OID, pgtype.Int8OID, pgtype.Int2OID, pgtype.Int8OID, pgtype.TimestamptzOID, pgtype.Int4OID, pgtype.Int8OID}
	if len(fields) != len(expectedNames) {
		return fmt.Errorf("benchmark votes schema has %d fields, expected %d", len(fields), len(expectedNames))
	}
	for index, field := range fields {
		if field.Name != expectedNames[index] || field.DataTypeOID != expectedOIDs[index] {
			return fmt.Errorf("benchmark votes field %d is %s/%d, expected %s/%d", index, field.Name, field.DataTypeOID, expectedNames[index], expectedOIDs[index])
		}
	}
	return nil
}

func (r Runner) WarmUp(ctx context.Context, rows int64) error {
	if _, err := r.pool.Exec(ctx, "TRUNCATE copy_benchmark.votes"); err != nil {
		return fmt.Errorf("truncate before warm-up: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin warm-up: %w", err)
	}
	defer tx.Rollback(ctx)
	count, err := r.runBinaryCopy(ctx, tx, rows)
	if err != nil {
		return fmt.Errorf("write warm-up: %w", err)
	}
	if count != rows {
		return fmt.Errorf("warm-up wrote %d rows, expected %d", count, rows)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit warm-up: %w", err)
	}
	if _, err := r.pool.Exec(ctx, "TRUNCATE copy_benchmark.votes"); err != nil {
		return fmt.Errorf("truncate after warm-up: %w", err)
	}
	return nil
}

func (r Runner) Run(ctx context.Context, strategy usecasecopybenchmark.Strategy, rows int64) (usecasecopybenchmark.Trial, error) {
	if _, err := r.pool.Exec(ctx, "TRUNCATE copy_benchmark.votes"); err != nil {
		return usecasecopybenchmark.Trial{}, fmt.Errorf("truncate benchmark votes: %w", err)
	}
	source, err := sourceChecksum(rows)
	if err != nil {
		return usecasecopybenchmark.Trial{}, fmt.Errorf("checksum benchmark source: %w", err)
	}
	started := time.Now()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return usecasecopybenchmark.Trial{}, fmt.Errorf("begin benchmark transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	var confirmed int64
	switch strategy {
	case usecasecopybenchmark.StrategySingleInsert:
		confirmed, err = r.runSingleInsert(ctx, tx, rows)
	case usecasecopybenchmark.StrategyBatchedInsert:
		confirmed, err = r.runBatchedInsert(ctx, tx, rows)
	case usecasecopybenchmark.StrategyTextCopy:
		confirmed, err = r.runTextCopy(ctx, tx, rows)
	case usecasecopybenchmark.StrategyBinaryCopy:
		confirmed, err = r.runBinaryCopy(ctx, tx, rows)
	default:
		return usecasecopybenchmark.Trial{}, fmt.Errorf("unsupported benchmark strategy %q", strategy)
	}
	if err != nil {
		return usecasecopybenchmark.Trial{}, fmt.Errorf("write strategy %q: %w", strategy, err)
	}
	if confirmed != rows {
		return usecasecopybenchmark.Trial{}, fmt.Errorf("strategy %q confirmed %d rows, expected %d", strategy, confirmed, rows)
	}
	if err := tx.Commit(ctx); err != nil {
		return usecasecopybenchmark.Trial{}, fmt.Errorf("commit strategy %q: %w", strategy, err)
	}
	elapsed := time.Since(started)
	storedRows, stored, err := storedChecksum(ctx, r.pool)
	if err != nil {
		return usecasecopybenchmark.Trial{}, fmt.Errorf("checksum stored strategy %q: %w", strategy, err)
	}
	return usecasecopybenchmark.Trial{
		Strategy:       strategy,
		Rows:           storedRows,
		Elapsed:        elapsed,
		SourceChecksum: source,
		StoredChecksum: stored,
	}, nil
}

func (r Runner) Environment(ctx context.Context) (usecasecopybenchmark.Environment, error) {
	var version string
	if err := r.pool.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return usecasecopybenchmark.Environment{}, fmt.Errorf("query PostgreSQL version: %w", err)
	}
	names := []string{"server_version", "synchronous_commit", "fsync", "full_page_writes", "wal_level", "max_wal_size"}
	settings := make([]usecasecopybenchmark.Setting, 0, len(names))
	for _, name := range names {
		var value string
		if err := r.pool.QueryRow(ctx, "SELECT current_setting($1)", name).Scan(&value); err != nil {
			return usecasecopybenchmark.Environment{}, fmt.Errorf("query PostgreSQL setting %s: %w", name, err)
		}
		settings = append(settings, usecasecopybenchmark.Setting{Name: name, Value: value})
	}
	return usecasecopybenchmark.Environment{PostgreSQL: version, PostgreSQLSettings: settings}, nil
}

func voteAt(index int64) voteRow {
	row := voteRow{
		Site:         "benchmark.example",
		ID:           index + 1,
		PostID:       index%100_000 + 1,
		VoteTypeID:   int16(2 + index%2),
		CreatedAt:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(index) * time.Millisecond),
		SourceOffset: index * 64,
	}
	if index%10 != 0 {
		value := index%500_000 + 1
		row.UserID = &value
	}
	if index%1000 == 0 {
		value := int32(50)
		row.BountyAmount = &value
	}
	return row
}

func (r voteRow) values() []any {
	var userID any
	if r.UserID != nil {
		userID = *r.UserID
	}
	var bountyAmount any
	if r.BountyAmount != nil {
		bountyAmount = *r.BountyAmount
	}
	return []any{r.Site, r.ID, r.PostID, r.VoteTypeID, userID, r.CreatedAt, bountyAmount, r.SourceOffset}
}

func writeCanonical(h hash.Hash, row voteRow) error {
	if err := binary.Write(h, binary.BigEndian, uint32(len(row.Site))); err != nil {
		return err
	}
	written, err := h.Write([]byte(row.Site))
	if err != nil {
		return err
	}
	if written != len(row.Site) {
		return io.ErrShortWrite
	}
	for _, value := range []any{row.ID, row.PostID, row.VoteTypeID} {
		if err := binary.Write(h, binary.BigEndian, value); err != nil {
			return err
		}
	}
	if row.UserID == nil {
		if err := writePresence(h, false); err != nil {
			return err
		}
	} else {
		if err := writePresence(h, true); err != nil {
			return err
		}
		if err := binary.Write(h, binary.BigEndian, *row.UserID); err != nil {
			return err
		}
	}
	if err := binary.Write(h, binary.BigEndian, row.CreatedAt.UTC().UnixNano()); err != nil {
		return err
	}
	if row.BountyAmount == nil {
		if err := writePresence(h, false); err != nil {
			return err
		}
	} else {
		if err := writePresence(h, true); err != nil {
			return err
		}
		if err := binary.Write(h, binary.BigEndian, *row.BountyAmount); err != nil {
			return err
		}
	}
	return binary.Write(h, binary.BigEndian, row.SourceOffset)
}

func writePresence(h hash.Hash, present bool) error {
	value := byte(0)
	if present {
		value = 1
	}
	written, err := h.Write([]byte{value})
	if err != nil {
		return err
	}
	if written != 1 {
		return io.ErrShortWrite
	}
	return nil
}

func sourceChecksum(rows int64) (string, error) {
	h := sha256.New()
	for index := int64(0); index < rows; index++ {
		if err := writeCanonical(h, voteAt(index)); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (r Runner) runSingleInsert(ctx context.Context, tx pgx.Tx, rows int64) (int64, error) {
	query := "INSERT INTO copy_benchmark.votes (" + voteColumns + ") VALUES ($1, $2, $3, $4, $5, $6, $7, $8)"
	if _, err := tx.Prepare(ctx, "copy_benchmark_single", query); err != nil {
		return 0, err
	}
	var confirmed int64
	for index := int64(0); index < rows; index++ {
		tag, err := tx.Exec(ctx, "copy_benchmark_single", voteAt(index).values()...)
		if err != nil {
			return confirmed, err
		}
		if tag.RowsAffected() != 1 {
			return confirmed, fmt.Errorf("single-row INSERT affected %d rows", tag.RowsAffected())
		}
		confirmed++
	}
	return confirmed, nil
}

func (r Runner) runBatchedInsert(ctx context.Context, tx pgx.Tx, rows int64) (int64, error) {
	const batchSize int64 = 1000
	var confirmed int64
	for confirmed < rows {
		current := min(batchSize, rows-confirmed)
		query := batchedInsertSQL(current)
		values := make([]any, 0, current*8)
		for offset := int64(0); offset < current; offset++ {
			values = append(values, voteAt(confirmed+offset).values()...)
		}
		tag, err := tx.Exec(ctx, query, values...)
		if err != nil {
			return confirmed, err
		}
		if tag.RowsAffected() != current {
			return confirmed, fmt.Errorf("batched INSERT affected %d rows, expected %d", tag.RowsAffected(), current)
		}
		confirmed += current
	}
	return confirmed, nil
}

func batchedInsertSQL(rows int64) string {
	var builder strings.Builder
	builder.WriteString("INSERT INTO copy_benchmark.votes (")
	builder.WriteString(voteColumns)
	builder.WriteString(") VALUES ")
	for row := int64(0); row < rows; row++ {
		if row > 0 {
			builder.WriteByte(',')
		}
		builder.WriteByte('(')
		for column := int64(0); column < 8; column++ {
			if column > 0 {
				builder.WriteByte(',')
			}
			builder.WriteByte('$')
			builder.WriteString(strconv.FormatInt(row*8+column+1, 10))
		}
		builder.WriteByte(')')
	}
	return builder.String()
}

func (r Runner) runTextCopy(ctx context.Context, tx pgx.Tx, rows int64) (int64, error) {
	pipeReader, pipeWriter := io.Pipe()
	result := make(chan producerResult, 1)
	go func() {
		producer := produceTextRows(pipeWriter, rows)
		closeErr := pipeWriter.CloseWithError(producer.Err)
		if producer.Err == nil && closeErr != nil {
			producer.Err = closeErr
		}
		result <- producer
	}()
	query := "COPY copy_benchmark.votes (" + voteColumns + ") FROM STDIN WITH (FORMAT text)"
	tag, copyErr := tx.Conn().PgConn().CopyFrom(ctx, pipeReader, query)
	closeErr := pipeReader.Close()
	producer := <-result
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if producer.Intrinsic && producer.Err != nil {
		return 0, producer.Err
	}
	if copyErr != nil {
		return 0, copyErr
	}
	if producer.Err != nil {
		return 0, producer.Err
	}
	if closeErr != nil && !errors.Is(closeErr, io.ErrClosedPipe) {
		return 0, closeErr
	}
	if tag.RowsAffected() != rows {
		return tag.RowsAffected(), fmt.Errorf("text COPY affected %d rows, expected %d", tag.RowsAffected(), rows)
	}
	return tag.RowsAffected(), nil
}

func produceTextRows(w io.Writer, rows int64) producerResult {
	buffered := bufio.NewWriterSize(w, 64*1024)
	for index := int64(0); index < rows; index++ {
		if _, err := io.WriteString(buffered, textRow(voteAt(index))); err != nil {
			return producerResult{Err: err}
		}
	}
	if err := buffered.Flush(); err != nil {
		return producerResult{Err: err}
	}
	return producerResult{}
}

func textRow(row voteRow) string {
	fields := []string{
		escapeText(row.Site),
		strconv.FormatInt(row.ID, 10),
		strconv.FormatInt(row.PostID, 10),
		strconv.FormatInt(int64(row.VoteTypeID), 10),
		`\N`,
		row.CreatedAt.UTC().Format(time.RFC3339Nano),
		`\N`,
		strconv.FormatInt(row.SourceOffset, 10),
	}
	if row.UserID != nil {
		fields[4] = strconv.FormatInt(*row.UserID, 10)
	}
	if row.BountyAmount != nil {
		fields[6] = strconv.FormatInt(int64(*row.BountyAmount), 10)
	}
	return strings.Join(fields, "\t") + "\n"
}

func escapeText(value string) string {
	return strings.NewReplacer("\\", "\\\\", "\t", "\\t", "\r", "\\r", "\n", "\\n").Replace(value)
}

func (r Runner) runBinaryCopy(ctx context.Context, tx pgx.Tx, rows int64) (int64, error) {
	index := int64(0)
	count, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"copy_benchmark", "votes"},
		[]string{"site", "id", "post_id", "vote_type_id", "user_id", "created_at", "bounty_amount", "source_offset"},
		pgx.CopyFromFunc(func() ([]any, error) {
			if index >= rows {
				return nil, nil
			}
			values := voteAt(index).values()
			index++
			return values, nil
		}),
	)
	if err != nil {
		return count, err
	}
	if count != rows {
		return count, fmt.Errorf("binary COPY affected %d rows, expected %d", count, rows)
	}
	return count, nil
}

func storedChecksum(ctx context.Context, pool *pgxpool.Pool) (int64, string, error) {
	rows, err := pool.Query(ctx, "SELECT "+voteColumns+" FROM copy_benchmark.votes ORDER BY id")
	if err != nil {
		return 0, "", err
	}
	defer rows.Close()
	h := sha256.New()
	var count int64
	for rows.Next() {
		var row voteRow
		if err := rows.Scan(&row.Site, &row.ID, &row.PostID, &row.VoteTypeID, &row.UserID, &row.CreatedAt, &row.BountyAmount, &row.SourceOffset); err != nil {
			return count, "", err
		}
		if err := writeCanonical(h, row); err != nil {
			return count, "", err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, "", err
	}
	return count, hex.EncodeToString(h.Sum(nil)), nil
}
