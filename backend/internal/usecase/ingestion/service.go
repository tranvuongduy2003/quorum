package ingestion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	domainingestion "quorum/internal/domain/ingestion"
)

const copyBatchRows = 4096
const copyBatchRawBytes int64 = 32 * 1024 * 1024

type pendingWrites struct {
	accepted   []domainingestion.SourceRecord
	quarantine []domainingestion.QuarantineRecord
	rawBytes   int64
}

type checkpointTracker struct {
	archiveID              domainingestion.ArchiveIdentity
	interval               int64
	confirmedSinceLastSave int64
}

type Service struct {
	archives    ArchiveFactory
	writer      Writer
	checkpoints CheckpointStore
	reporter    CheckpointReporter
	now         func() time.Time
	policies    []domainingestion.Policy
}

func (s Service) WithCheckpointReporter(reporter CheckpointReporter) Service {
	s.reporter = reporter
	return s
}

func NewService(
	archives ArchiveFactory,
	writer Writer,
	checkpoints CheckpointStore,
	now func() time.Time,
	policies ...domainingestion.Policy,
) Service {
	if now == nil {
		now = time.Now
	}

	return Service{
		archives:    archives,
		writer:      writer,
		checkpoints: checkpoints,
		now:         now,
		policies:    append([]domainingestion.Policy(nil), policies...),
	}
}
func (s Service) Run(ctx context.Context, command Command) (summary RunSummary, runErr error) {
	if !command.DryRun && s.writer == nil {
		summary = summary.WithStatus(RunStatusFailed)
		return summary, RunError{Table: "request", Offset: 0, Err: ErrWriterRequired}
	}

	archive, err := s.openValidated(ctx, command)
	if err != nil {
		return RunSummary{}, err
	}
	defer func() {
		if closeErr := archive.Close(); closeErr != nil && runErr == nil {
			summary = summary.WithStatus(RunStatusFailed)
			runErr = RunError{Table: command.Tables[0].String(), Offset: 0, Err: closeErr}
		}
	}()

	summary = RunSummary{
		DryRun:           command.DryRun,
		ThresholdPercent: command.RejectThresholdPercent,
		Status:           RunStatusOK,
		Tables:           make([]TableSummary, 0, len(command.Tables)),
	}

	for _, table := range command.Tables {
		tSum, tableErr := s.processTable(ctx, archive, table, command)

		summary.Tables = append(summary.Tables, tSum)
		summary.ObservedPercent = summary.MaxRejectedPercent()

		if tableErr != nil {
			return summary.WithStatus(RunStatusFailed), tableErr
		}
	}

	failingTable, exceeded := summary.FirstTableAbove(command.RejectThresholdPercent)
	if exceeded {
		summary.Status = RunStatusFailed
		return summary, RunError{
			Table:  failingTable.Table.String(),
			Offset: failingTable.LastOffset,
			Err: RejectThresholdError{
				ObservedPercent:  failingTable.RejectedPercent(),
				ThresholdPercent: command.RejectThresholdPercent,
			},
		}
	}

	return summary, nil
}

func (s Service) openValidated(ctx context.Context, command Command) (Archive, error) {
	if err := ctx.Err(); err != nil {
		return nil, RunError{Table: command.Tables[0].String(), Offset: 0, Err: err}
	}

	archive, err := s.archives.Open(command.ArchivePath)
	if err != nil {
		return nil, RunError{Table: command.Tables[0].String(), Offset: 0, Err: err}
	}

	if err := archive.ValidateTables(command.Tables); err != nil {
		_ = archive.Close()
		if sourceErr, ok := sourceError(err); ok {
			return nil, RunError{Table: sourceErr.Table.String(), Offset: sourceErr.Offset, Err: sourceErr.Err}
		}
		return nil, RunError{Table: command.Tables[0].String(), Offset: 0, Err: err}
	}

	return archive, nil
}

func (s Service) processTable(ctx context.Context, archive Archive, table domainingestion.Table, command Command) (summary TableSummary, runErr error) {
	pending := pendingWrites{
		accepted:   make([]domainingestion.SourceRecord, 0, copyBatchRows),
		quarantine: make([]domainingestion.QuarantineRecord, 0, copyBatchRows),
		rawBytes:   0,
	}

	summary = TableSummary{
		Table:      table,
		Rejections: make(map[domainingestion.ReasonCode]int64),
	}

	var tracker *checkpointTracker
	if !command.DryRun && s.checkpoints != nil {
		tracker = &checkpointTracker{
			archiveID: archive.Identity(),
			interval:  command.CheckpointInterval,
		}
	}

	var checkpointOffset int64
	if tracker != nil {
		checkpoint, found, err := s.checkpoints.Load(ctx, command.Site, table)
		if err != nil {
			return summary, RunError{Table: table.String(), Offset: 0, Err: err}
		}
		if found {
			if checkpoint.ArchiveID != tracker.archiveID {
				return summary, RunError{
					Table:  table.String(),
					Offset: 0,
					Err: fmt.Errorf(
						"%w: stored=%s current=%s",
						domainingestion.ErrIncompatibleArchive,
						checkpoint.ArchiveID,
						tracker.archiveID,
					),
				}
			}

			checkpointOffset = checkpoint.SourceOffset
			summary.Confirmed = checkpoint.ConfirmedCount
			summary.LastOffset = checkpoint.SourceOffset
			summary.Resumed = true
			summary.ResumedOffset = checkpoint.SourceOffset

			if err := s.checkpoints.CleanAfterCheckpoint(ctx, command.Site, table, checkpointOffset); err != nil {
				return summary, RunError{Table: table.String(), Offset: checkpointOffset, Err: err}
			}
		}
	}

	member, err := archive.OpenTable(ctx, table, command.MaxRecordBytes)
	if err != nil {
		var sourceErr SourceError
		if errors.As(err, &sourceErr) {
			return summary, RunError{
				Table:  sourceErr.Table.String(),
				Offset: sourceErr.Offset,
				Err:    sourceErr.Err,
			}
		}
		return summary, RunError{
			Table:  table.String(),
			Offset: 0,
			Err:    err,
		}
	}
	defer func() {
		if closeErr := member.Close(); closeErr != nil && runErr == nil {
			runErr = RunError{Table: table.String(), Offset: summary.LastOffset, Err: closeErr}
		}
	}()

	skipping := summary.Resumed
	for {
		record, err := member.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = s.flushAndCheckpoint(ctx, command.Site, table, &pending, &summary, tracker)
				if err != nil {
					return summary, err
				}
				break
			}

			var sourceErr SourceError
			if errors.As(err, &sourceErr) {
				summary.Processed++
				summary.Malformed++
				summary.LastOffset = sourceErr.Offset
				return summary, RunError{
					Table:  sourceErr.Table.String(),
					Offset: sourceErr.Offset,
					Err:    sourceErr.Err,
				}
			}

			return summary, RunError{
				Table:  table.String(),
				Offset: summary.LastOffset,
				Err:    err,
			}
		}

		if skipping && record.Offset <= checkpointOffset {
			continue
		}
		skipping = false

		summary.Processed++
		summary.LastOffset = record.Offset

		finding, rejected := domainingestion.FirstFinding(record, s.policies)
		if rejected {
			summary.Rejected++
			summary.Rejections[finding.Reason]++
		} else {
			summary.Valid++
		}

		if command.DryRun {
			continue
		}

		if pending.shouldFlush(record) {
			err = s.flushAndCheckpoint(ctx, command.Site, table, &pending, &summary, tracker)
			if err != nil {
				return summary, err
			}
		}

		if !rejected {
			pending.accepted = append(pending.accepted, record)
		} else {
			quarantineRecord := domainingestion.NewQuarantineRecord(command.Site, record, finding.Reason, s.now())
			pending.quarantine = append(pending.quarantine, quarantineRecord)
		}

		pending.rawBytes += int64(len(record.Raw))

		checkpointBoundary := tracker != nil && int64(pending.count()) >= tracker.remaining()
		if pending.count() >= copyBatchRows || pending.rawBytes >= copyBatchRawBytes || checkpointBoundary {
			err = s.flushAndCheckpoint(ctx, command.Site, table, &pending, &summary, tracker)
			if err != nil {
				return summary, err
			}
		}
	}

	return summary, nil
}

func (s Service) flushAndCheckpoint(
	ctx context.Context,
	site domainingestion.Site,
	table domainingestion.Table,
	pending *pendingWrites,
	summary *TableSummary,
	tracker *checkpointTracker,
) error {
	batchOffset := pending.maxOffset()
	previousConfirmed := summary.Confirmed
	if err := s.flushPending(ctx, site, table, pending, summary); err != nil {
		return err
	}
	if tracker == nil || !tracker.advance(summary.Confirmed-previousConfirmed) {
		return nil
	}

	checkpoint := domainingestion.NewCheckpoint(
		site,
		table,
		tracker.archiveID,
		batchOffset,
		summary.Confirmed,
		s.now(),
	)
	if err := s.checkpoints.Save(ctx, checkpoint); err != nil {
		return RunError{Table: table.String(), Offset: batchOffset, Err: err}
	}
	if s.reporter != nil {
		s.reporter.CheckpointSaved(checkpoint)
	}

	tracker.confirmedSinceLastSave = 0
	return nil
}

func (s Service) flushPending(
	ctx context.Context,
	site domainingestion.Site,
	table domainingestion.Table,
	pending *pendingWrites,
	summary *TableSummary,
) error {
	if len(pending.accepted) > 0 {
		offset := pending.accepted[len(pending.accepted)-1].Offset
		count, err := s.writer.WriteAccepted(ctx, site, table, pending.accepted)

		if err != nil {
			failureOffset, failureErr := writeFailure(err, offset)
			return RunError{
				Table:  table.String(),
				Offset: failureOffset,
				Err:    failureErr,
			}
		}

		if count != int64(len(pending.accepted)) {
			failureOffset, failureErr := writeFailure(ErrWriteCountMismatch, offset)
			return RunError{
				Table:  table.String(),
				Offset: failureOffset,
				Err:    failureErr,
			}
		}

		summary.Confirmed += count
	}

	if len(pending.quarantine) > 0 {
		offset := pending.quarantine[len(pending.quarantine)-1].Source.Offset
		count, err := s.writer.WriteQuarantine(ctx, pending.quarantine)

		if err != nil {
			failureOffset, failureErr := writeFailure(err, offset)
			return RunError{
				Table:  table.String(),
				Offset: failureOffset,
				Err:    failureErr,
			}
		}

		if count != int64(len(pending.quarantine)) {
			failureOffset, failureErr := writeFailure(ErrWriteCountMismatch, offset)
			return RunError{
				Table:  table.String(),
				Offset: failureOffset,
				Err:    failureErr,
			}
		}

		summary.Confirmed += count
	}

	pending.reset()

	return nil
}

func sourceError(err error) (SourceError, bool) {
	var sourceErr SourceError
	return sourceErr, errors.As(err, &sourceErr)
}

func (p pendingWrites) count() int {
	return len(p.accepted) + len(p.quarantine)
}

func (p pendingWrites) maxOffset() int64 {
	result := int64(-1)
	if len(p.accepted) > 0 {
		result = p.accepted[len(p.accepted)-1].Offset
	}
	if len(p.quarantine) > 0 {
		offset := p.quarantine[len(p.quarantine)-1].Source.Offset
		if offset > result {
			result = offset
		}
	}
	return result
}

func (p pendingWrites) shouldFlush(next domainingestion.SourceRecord) bool {
	currentCount := p.count()
	if currentCount == 0 {
		return false
	}

	if currentCount+1 > copyBatchRows {
		return true
	}

	if p.rawBytes+int64(len(next.Raw)) > copyBatchRawBytes {
		return true
	}

	return false
}

func (p *pendingWrites) reset() {
	p.accepted = p.accepted[:0]
	p.quarantine = p.quarantine[:0]
	p.rawBytes = 0
}

func (t *checkpointTracker) advance(confirmed int64) bool {
	t.confirmedSinceLastSave += confirmed
	return t.confirmedSinceLastSave >= t.interval
}

func (t *checkpointTracker) remaining() int64 {
	return t.interval - t.confirmedSinceLastSave
}

func writeFailure(err error, fallback int64) (int64, error) {
	var writeErr WriteFailure
	if errors.As(err, &writeErr) {
		return writeErr.Offset, writeErr.Err
	}

	return fallback, err
}
