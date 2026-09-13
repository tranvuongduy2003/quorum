package copybenchmark

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

type runnerStub struct {
	prepareErr     error
	warmUpErr      error
	environmentErr error
	environment    Environment
	trials         map[Strategy][]Trial
	calls          []string
	indexes        map[Strategy]int
}

func (r *runnerStub) Prepare(context.Context) error {
	r.calls = append(r.calls, "prepare")
	return r.prepareErr
}

func (r *runnerStub) WarmUp(_ context.Context, rows int64) error {
	r.calls = append(r.calls, "warmup")
	if rows != 10_000 {
		return errors.New("unexpected warm-up rows")
	}
	return r.warmUpErr
}

func (r *runnerStub) Run(_ context.Context, strategy Strategy, _ int64) (Trial, error) {
	r.calls = append(r.calls, string(strategy))
	index := r.indexes[strategy]
	r.indexes[strategy] = index + 1
	return r.trials[strategy][index], nil
}

func (r *runnerStub) Environment(context.Context) (Environment, error) {
	r.calls = append(r.calls, "environment")
	return r.environment, r.environmentErr
}

func TestServiceRunAggregatesEligibleTrials(t *testing.T) {
	rows := int64(1_000_000)
	runner := successfulRunner(rows)
	report, err := NewService(runner).Run(context.Background(), rows, 3)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Join(runner.calls[:3], ",") != "prepare,environment,warmup" {
		t.Fatalf("calls = %v", runner.calls)
	}
	if len(report.Trials) != 12 || len(report.Results) != 4 {
		t.Fatalf("trial/result count = %d/%d", len(report.Trials), len(report.Results))
	}
	if !report.Publishable || !report.ThresholdMet {
		t.Fatalf("publication gates = %t/%t", report.Publishable, report.ThresholdMet)
	}
	baseline := report.Results[0]
	if baseline.MedianElapsed != 200*time.Second || baseline.MinElapsed != 100*time.Second || baseline.MaxElapsed != 300*time.Second {
		t.Fatalf("baseline durations = %s/%s/%s", baseline.MinElapsed, baseline.MedianElapsed, baseline.MaxElapsed)
	}
	if baseline.RelativeSpeed != 1 {
		t.Fatalf("baseline relative speed = %f", baseline.RelativeSpeed)
	}
	binary := report.Results[3]
	if binary.RowsPerSecond != 50_000 || math.Abs(binary.RelativeSpeed-10) > 0.000001 {
		t.Fatalf("binary throughput/relative = %f/%f", binary.RowsPerSecond, binary.RelativeSpeed)
	}
	for _, trial := range report.Trials {
		if !trial.Eligible || trial.RowsPerSecond <= 0 || trial.Run < 1 || trial.Run > 3 {
			t.Fatalf("invalid trial = %+v", trial)
		}
	}
}

func TestServiceRunSmokeIsNotPublishable(t *testing.T) {
	runner := successfulRunner(10_000)
	report, err := NewService(runner).Run(context.Background(), 10_000, 3)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Publishable {
		t.Fatal("smoke report is publishable")
	}
}

func TestServiceRunBinaryThresholdBoundary(t *testing.T) {
	passing := successfulRunner(PublishedRows)
	passReport, err := NewService(passing).Run(context.Background(), PublishedRows, 3)
	if err != nil {
		t.Fatalf("Run() passing error = %v", err)
	}
	if !passReport.ThresholdMet {
		t.Fatal("exact threshold did not pass")
	}

	failing := successfulRunner(PublishedRows)
	for index := range failing.trials[StrategyBinaryCopy] {
		trial := failing.trials[StrategyBinaryCopy][index]
		trial.Elapsed = 20*time.Second + time.Nanosecond
		failing.trials[StrategyBinaryCopy][index] = trial
	}
	failReport, err := NewService(failing).Run(context.Background(), PublishedRows, 3)
	if err != nil {
		t.Fatalf("Run() failing error = %v", err)
	}
	if failReport.ThresholdMet {
		t.Fatal("below-threshold throughput passed")
	}
}

func TestServiceRunRejectsInvalidInputsBeforeRunnerCalls(t *testing.T) {
	for _, test := range []struct {
		name        string
		rows        int64
		repetitions int
		want        error
	}{
		{name: "rows", rows: 0, repetitions: 3, want: ErrInvalidRowCount},
		{name: "repetitions", rows: 1, repetitions: 2, want: ErrInvalidRepetitions},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := successfulRunner(1)
			_, err := NewService(runner).Run(context.Background(), test.rows, test.repetitions)
			if !errors.Is(err, test.want) {
				t.Fatalf("Run() error = %v, want %v", err, test.want)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("runner calls = %v", runner.calls)
			}
		})
	}
}

func TestServiceRunRejectsIneligibleTrial(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Trial)
		match  string
	}{
		{name: "count", mutate: func(trial *Trial) { trial.Rows-- }, match: "expected_rows=10000 actual_rows=9999"},
		{name: "empty checksum", mutate: func(trial *Trial) { trial.SourceChecksum = "" }, match: "source_checksum=\"\""},
		{name: "checksum mismatch", mutate: func(trial *Trial) { trial.StoredChecksum = "different" }, match: "stored_checksum=\"different\""},
		{name: "elapsed", mutate: func(trial *Trial) { trial.Elapsed = 0 }, match: "elapsed=0s"},
		{name: "strategy", mutate: func(trial *Trial) { trial.Strategy = StrategyBinaryCopy }, match: "returned_strategy=\"binary COPY\""},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := successfulRunner(10_000)
			trial := runner.trials[StrategySingleInsert][0]
			test.mutate(&trial)
			runner.trials[StrategySingleInsert][0] = trial
			_, err := NewService(runner).Run(context.Background(), 10_000, 3)
			if !errors.Is(err, ErrIneligibleTrial) || !strings.Contains(err.Error(), "strategy=\"single-row INSERT\" run=1") || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("Run() error = %v", err)
			}
		})
	}
}

func TestServiceRunRejectsChecksumVarianceAcrossTrials(t *testing.T) {
	runner := successfulRunner(10_000)
	trial := runner.trials[StrategySingleInsert][1]
	trial.SourceChecksum = "different"
	trial.StoredChecksum = "different"
	runner.trials[StrategySingleInsert][1] = trial
	_, err := NewService(runner).Run(context.Background(), 10_000, 3)
	if !errors.Is(err, ErrIneligibleTrial) || !strings.Contains(err.Error(), "strategy=\"single-row INSERT\" run=2") || !strings.Contains(err.Error(), "expected_checksum=\"checksum\" actual_checksum=\"different\"") {
		t.Fatalf("Run() error = %v", err)
	}
}

func successfulRunner(rows int64) *runnerStub {
	checksum := "checksum"
	durations := map[Strategy][]time.Duration{
		StrategySingleInsert:  {300 * time.Second, 100 * time.Second, 200 * time.Second},
		StrategyBatchedInsert: {150 * time.Second, 120 * time.Second, 130 * time.Second},
		StrategyTextCopy:      {40 * time.Millisecond, 30 * time.Millisecond, 35 * time.Millisecond},
		StrategyBinaryCopy:    {20 * time.Second, 19 * time.Second, 21 * time.Second},
	}
	trials := make(map[Strategy][]Trial, len(durations))
	for strategy, values := range durations {
		for _, elapsed := range values {
			trials[strategy] = append(trials[strategy], Trial{
				Strategy:       strategy,
				Rows:           rows,
				Elapsed:        elapsed,
				SourceChecksum: checksum,
				StoredChecksum: checksum,
			})
		}
	}
	return &runnerStub{trials: trials, indexes: make(map[Strategy]int)}
}
