package ingestion

import (
	"context"
	"errors"
	"io"
	"quorum/internal/domain/ingestion"
)

type Service struct {
	archives ArchiveFactory
	policies []ingestion.Policy
}

func NewService(archives ArchiveFactory, policies ...ingestion.Policy) Service {
	return Service{
		archives: archives,
		policies: append([]ingestion.Policy(nil), policies...),
	}
}
func (s Service) Run(ctx context.Context, command Command) (summary RunSummary, runErr error) {
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
		tSum, tableErr := s.processTable(ctx, archive, table, command.MaxRecordBytes)

		summary.Tables = append(summary.Tables, tSum)

		if tableErr != nil {
			return summary.WithStatus(RunStatusFailed), tableErr
		}
	}

	summary.ObservedPercent = summary.MaxRejectedPercent()
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

func (s Service) processTable(ctx context.Context, archive Archive, table ingestion.Table, maxRecordBytes int) (summary TableSummary, runErr error) {
	summary = TableSummary{
		Table:      table,
		Rejections: make(map[ingestion.ReasonCode]int64),
	}

	member, err := archive.OpenTable(ctx, table, maxRecordBytes)
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

		finding, rejected := ingestion.FirstFinding(record, s.policies)
		if rejected {
			summary.Rejected++
			summary.Rejections[finding.Reason]++
		} else {
			summary.Valid++
		}
	}

	return summary, nil
}

func sourceError(err error) (SourceError, bool) {
	var sourceErr SourceError
	return sourceErr, errors.As(err, &sourceErr)
}
