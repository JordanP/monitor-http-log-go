package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type mockTSDB struct {
	i          int
	avgHitRate []float64
}

func (m *mockTSDB) GetAvgRate(_ time.Duration) (float64, float64) {
	defer func() { m.i++ }()
	// While we still have values in the sequence, return these
	if m.i < len(m.avgHitRate) {
		return m.avgHitRate[m.i], 0
	}
	// When we've ran out of values, always return the last value
	return m.avgHitRate[len(m.avgHitRate)-1], 0
}

func TestAlerterWhenAvgBelowThreshold(t *testing.T) {
	threshold := 42.0
	tsdb := &mockTSDB{avgHitRate: []float64{threshold - 1}}

	b := &strings.Builder{}
	alerter := &Alerter{tsdb: tsdb, threshold: threshold, period: 2 * time.Second, evaluateEvery: time.Millisecond, out: b}
	// Let the alerter run for a while
	ctx, done := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer done()
	alerter.PrintAlertStateChange(ctx)

	if out := b.String(); out != "" {
		t.Fatal("alert state change should not have triggered")
	}
}

func TestAlerterStateTransitions(t *testing.T) {
	threshold := 42.0
	tsdb := &mockTSDB{avgHitRate: []float64{threshold + 1, threshold - 1}}

	b := &strings.Builder{}
	alerter := &Alerter{tsdb: tsdb, threshold: threshold, period: 2 * time.Second, evaluateEvery: time.Millisecond, out: b}
	// Let the alerter run for a while
	ctx, done := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer done()
	alerter.PrintAlertStateChange(ctx)

	lines := strings.Split(b.String(), "\n")
	if len(lines) != 3 {
		t.Fatalf("unexpected log lines: got %q", b.String())
	}

	if !strings.Contains(lines[0], fmt.Sprintf("High traffic generated an alert - hits = %.1f", threshold+1)) {
		t.Fatal("alert above threshold did not trigger")
	}
	if !strings.Contains(lines[1], "Alert recovered") {
		t.Fatal("alert recovery did not trigger")
	}
	if lines[2] != "" {
		t.Fatalf("got an unexpected log line %q", lines[2])
	}
}
