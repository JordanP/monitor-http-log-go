package main

import (
	"context"
	"fmt"
	"io"
	"time"
)

type alertState int

const (
	AlertLowState alertState = iota
	AlertHighState
)

// averager has a single method that returns the average Hits and Bandwidth since the given duration
type averager interface {
	GetAvgRate(since time.Duration) (float64, float64)
}

type Alerter struct {
	tsdb averager

	threshold     float64
	period        time.Duration
	evaluateEvery time.Duration

	out io.Writer
}

func (a *Alerter) PrintAlertStateChange(ctx context.Context) error {
	ticker := time.NewTicker(a.evaluateEvery)
	defer ticker.Stop()

	var currentState alertState
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			reqs, _ := a.tsdb.GetAvgRate(a.period)

			var state alertState
			if reqs >= a.threshold {
				state = AlertHighState
			} else {
				state = AlertLowState
			}

			if state != currentState {
				if state == AlertHighState {
					// We don't include a timestamp as it's already done by the logger
					msg := "\033[93mHigh traffic generated an alert - hits = %.1f req/s in the last %s\033[0m\n"
					fmt.Fprintf(a.out, msg, reqs, a.period)
				} else {
					msg := "\033[92mAlert recovered\033[0m\n"
					fmt.Fprint(a.out, msg)
				}

				currentState = state
			}
		}
	}
}
