package copybenchmark

import "time"

type Strategy string

const PublishedRows int64 = 1_000_000
const MinimumBinaryRowsPerSecond = 50_000

const StrategySingleInsert Strategy = "single-row INSERT"
const StrategyBatchedInsert Strategy = "1,000-row batched INSERT"
const StrategyTextCopy Strategy = "text COPY"
const StrategyBinaryCopy Strategy = "binary COPY"

type Trial struct {
	Strategy       Strategy
	Run            int
	Rows           int64
	Elapsed        time.Duration
	RowsPerSecond  float64
	SourceChecksum string
	StoredChecksum string
	Eligible       bool
}

type StrategyResult struct {
	Strategy      Strategy
	Rows          int64
	MedianElapsed time.Duration
	MinElapsed    time.Duration
	MaxElapsed    time.Duration
	RowsPerSecond float64
	RelativeSpeed float64
	Checksum      string
}

type Setting struct {
	Name  string
	Value string
}

type Environment struct {
	GitSHA             string
	GitDirty           bool
	Hardware           string
	ResourceLimits     string
	GoVersion          string
	OSArch             string
	LogicalCPUs        int
	PostgreSQL         string
	PostgreSQLSettings []Setting
	CapturedAt         time.Time
}

type Report struct {
	Environment  Environment
	Rows         int64
	Repetitions  int
	Trials       []Trial
	Results      []StrategyResult
	Publishable  bool
	ThresholdMet bool
}
