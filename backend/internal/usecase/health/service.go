package health

import (
	"context"
	"sync"
	"time"
)

type DependencyStatus struct {
	Name    string
	Healthy bool
	Err     error
}

type ReadinessReport struct {
	Ready        bool
	Dependencies []DependencyStatus
	CheckedAt    time.Time
}

type Service interface {
	Readiness(ctx context.Context) ReadinessReport
}

func NewService(now func() time.Time, probes ...DependencyProbe) Service {
	return &healthUseCase{now: now, probes: probes}
}

type healthUseCase struct {
	now    func() time.Time
	probes []DependencyProbe
}

func (hu *healthUseCase) Readiness(ctx context.Context) ReadinessReport {
	report := ReadinessReport{
		Dependencies: make([]DependencyStatus, len(hu.probes)),
	}

	var wg sync.WaitGroup
	for i, probe := range hu.probes {
		wg.Go(func() {
			err := probe.Check(ctx)

			report.Dependencies[i] = DependencyStatus{
				Name:    probe.Name(),
				Healthy: err == nil,
				Err:     err,
			}
		})
	}
	wg.Wait()

	report.Ready = true
	for _, dependency := range report.Dependencies {
		if !dependency.Healthy {
			report.Ready = false
			break
		}
	}

	report.CheckedAt = hu.now()

	return report
}
