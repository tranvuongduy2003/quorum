package ingestion

import (
	"context"
	"errors"
	"io"
	domainingestion "quorum/internal/domain/ingestion"
	"time"
)

type Service struct {
	archives   ArchiveFactory
	quarantine QuarantineStore
	now        func() time.Time
	policies   []domainingestion.Policy
}

func NewService(
	archives ArchiveFactory,
	quarantine QuarantineStore,
	now func() time.Time,
	policies ...domainingestion.Policy,
) Service {
	if now == nil {
		now = time.Now
	}

	return Service{
		archives:   archives,
		quarantine: quarantine,
		now:        now,
		policies:   append([]domainingestion.Policy(nil), policies...),
	}
}
func (s Service) Run(ctx context.Context, command Command) (summary RunSummary, runErr error) {
	if !command.DryRun && s.quarantine == nil {
		summary = summary.WithStatus(RunStatusFailed)
		return summary, RunError{Table: "request", Offset: 0, Err: ErrQuarantineStoreRequired}
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
	summary = TableSummary{
		Table:      table,
		Rejections: make(map[domainingestion.ReasonCode]int64),
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

	for {
		record, err := member.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
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

		summary.Processed++
		summary.LastOffset = record.Offset

		finding, rejected := domainingestion.FirstFinding(record, s.policies)
		if rejected {
			summary.Rejected++
			summary.Rejections[finding.Reason]++
		} else {
			summary.Valid++
		}

		if !rejected || command.DryRun {
			continue
		}

		quarantineRecord := domainingestion.NewQuarantineRecord(command.Site, record, finding.Reason, s.now())
		err = s.quarantine.Save(ctx, quarantineRecord)
		if err != nil {
			return summary, RunError{
				Table:  record.Table.String(),
				Offset: record.Offset,
				Err:    err,
			}
		}
	}

	return summary, nil
}

func sourceError(err error) (SourceError, bool) {
	var sourceErr SourceError
	return sourceErr, errors.As(err, &sourceErr)
}
