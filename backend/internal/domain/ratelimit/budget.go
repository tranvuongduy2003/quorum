package ratelimit

import "time"

type Budget struct {
	Limit      int
	Used       int
	ResetAfter time.Duration
}

func (b Budget) Allowed() bool {
	return b.Used <= b.Limit
}

func (b Budget) Remaining() int {
	return max(0, b.Limit-b.Used)
}
