package copybenchmark

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

var ErrInvalidRowCount = errors.New("benchmark row count must be positive")
var ErrInvalidRepetitions = errors.New("benchmark requires exactly three repetitions")
var ErrIneligibleTrial = errors.New("benchmark trial is not eligible")

type Service struct {
	runner Runner
}

func NewService(runner Runner) Service {
	return Service{runner: runner}
}

func (s Service) Run(ctx context.Context, rows int64, repetitions int) (Report, error) {
	if rows <= 0 {
		return Report{}, ErrInvalidRowCount
	}
	if repetitions != 3 {
		return Report{}, ErrInvalidRepetitions
	}
	if err := s.runner.Prepare(ctx); err != nil {
		return Report{}, fmt.Errorf("prepare benchmark: %w", err)
	}
	environment, err := s.runner.Environment(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("capture benchmark environment: %w", err)
	}
	if err := s.runner.WarmUp(ctx, 10_000); err != nil {
		return Report{}, fmt.Errorf("warm up benchmark: %w", err)
	}

	strategies := []Strategy{
		StrategySingleInsert,
		StrategyBatchedInsert,
		StrategyTextCopy,
		StrategyBinaryCopy,
	}
	report := Report{
		Environment: environment,
		Rows:        rows,
		Repetitions: repetitions,
		Trials:      make([]Trial, 0, len(strategies)*repetitions),
		Results:     make([]StrategyResult, 0, len(strategies)),
	}
	expectedChecksum := ""

	for _, strategy := range strategies {
		for run := 1; run <= repetitions; run++ {
			trial, runErr := s.runner.Run(ctx, strategy, rows)
			if runErr != nil {
				return Report{}, fmt.Errorf("run benchmark strategy=%q run=%d: %w", strategy, run, runErr)
			}
			trial.Run = run
			if trial.Strategy != strategy {
				return Report{}, fmt.Errorf("%w: strategy=%q run=%d returned_strategy=%q", ErrIneligibleTrial, strategy, run, trial.Strategy)
			}
			if trial.Rows != rows {
				return Report{}, fmt.Errorf("%w: strategy=%q run=%d expected_rows=%d actual_rows=%d", ErrIneligibleTrial, strategy, run, rows, trial.Rows)
			}
			if trial.SourceChecksum == "" || trial.SourceChecksum != trial.StoredChecksum {
				return Report{}, fmt.Errorf("%w: strategy=%q run=%d source_checksum=%q stored_checksum=%q", ErrIneligibleTrial, strategy, run, trial.SourceChecksum, trial.StoredChecksum)
			}
			if expectedChecksum == "" {
				expectedChecksum = trial.SourceChecksum
			} else if trial.SourceChecksum != expectedChecksum {
				return Report{}, fmt.Errorf("%w: strategy=%q run=%d expected_checksum=%q actual_checksum=%q", ErrIneligibleTrial, strategy, run, expectedChecksum, trial.SourceChecksum)
			}
			if trial.Elapsed <= 0 {
				return Report{}, fmt.Errorf("%w: strategy=%q run=%d elapsed=%s", ErrIneligibleTrial, strategy, run, trial.Elapsed)
			}
			trial.RowsPerSecond = float64(rows) / trial.Elapsed.Seconds()
			trial.Eligible = true
			report.Trials = append(report.Trials, trial)
		}
	}

	for _, strategy := range strategies {
		elapsed := make([]time.Duration, 0, repetitions)
		checksum := ""
		for _, trial := range report.Trials {
			if trial.Strategy == strategy {
				elapsed = append(elapsed, trial.Elapsed)
				checksum = trial.StoredChecksum
			}
		}
		sort.Slice(elapsed, func(i, j int) bool {
			return elapsed[i] < elapsed[j]
		})
		median := elapsed[1]
		report.Results = append(report.Results, StrategyResult{
			Strategy:      strategy,
			Rows:          rows,
			MedianElapsed: median,
			MinElapsed:    elapsed[0],
			MaxElapsed:    elapsed[2],
			RowsPerSecond: float64(rows) / median.Seconds(),
			Checksum:      checksum,
		})
	}

	baseline := report.Results[0].MedianElapsed.Seconds()
	for index := range report.Results {
		report.Results[index].RelativeSpeed = baseline / report.Results[index].MedianElapsed.Seconds()
	}
	report.Publishable = rows == PublishedRows && len(report.Trials) == len(strategies)*repetitions
	for _, result := range report.Results {
		if result.Strategy == StrategyBinaryCopy {
			report.ThresholdMet = result.RowsPerSecond >= MinimumBinaryRowsPerSecond
		}
	}

	return report, nil
}
