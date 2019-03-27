package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var (
	log = &logrus.Logger{
		Out:       os.Stderr,
		Formatter: &logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "2006-01-02T15:04:05"},
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}

	commonLogFormat = regexp.MustCompile(`(?P<client_ip>[^ ]*) (?P<user_identifier>[^ ]*) (?P<user_id>[^ ]*) \[(?P<date>[^]]*)\] "(?P<http_method>[A-Z]*) (?P<http_url>[^"]*) (?P<http_version>HTTP/\d.\d)" (?P<status_code>[^ ]*) (?P<bytes_sent>[^ ]*)`)
)

func parseHitLines(lines <-chan string, hits chan<- HTTPHit) error {
	defer close(hits)

	for line := range lines {
		submatch := map[string]string{}
		for i, name := range commonLogFormat.FindStringSubmatch(line) {
			submatch[commonLogFormat.SubexpNames()[i]] = name
		}
		hit := NewHTTPHit(
			submatch["date"], submatch["http_method"], submatch["http_url"], submatch["status_code"],
			submatch["bytes_sent"],
		)
		hits <- hit
	}
	return nil
}

// aggregateHits aggregates HTTP hits by a time window of 1 sec and add the aggregated values to the timeseries
// We aggregate so that we don't have to store each and every data point in the DB. It's a memory/cpu vs granularity
// tradeoff
func aggregateHits(hits <-chan HTTPHit, timeseries *Timeseries) error {
	row := NewRow(time.Time{})
	ticker := time.NewTicker(time.Second)

	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Row is empty, don't insert into the timeseries so the timeseries stays sparse
			if row.time.IsZero() {
				continue
			}
			// Insert/Flush the previous row
			if time.Now().Truncate(time.Second) != row.time {
				timeseries.insert(row)
				// Allocate a new row
				row = NewRow(time.Time{})
			}
		case hit, ok := <-hits:
			if !ok {
				return nil
			}
			truncatedTime := hit.time.Truncate(time.Second)
			// We only enter this if statement when we received our very first hit or if no hit for more than 1sec
			if row.time.IsZero() {
				row.time = truncatedTime
			}
			if truncatedTime != row.time {
				timeseries.insert(row)
				// Allocate a new row
				row = NewRow(truncatedTime)
			}
			row.totalHits++
			row.totalBits += hit.bytesSent
			row.perSectionHits[hit.section]++
		}
	}
}

func printStats(ctx context.Context, period time.Duration, ts *Timeseries) error {
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			log.Printf("Top 3 sections in the last %s: %v", period, ts.TopNSectionsSince(3, period))
			req, bw := ts.GetAvgRate(period)
			log.Printf("Average req/s: %.1f, Average throughput: %.1f B/s", req, bw)
		}
	}
}

func main() {
	logPath := flag.String("logpath", os.Getenv("LOGPATH"), "Path to the access log file")
	alertThreshold := flag.Float64("alertThreshold", 10.0, "Alert threshold")
	flag.Parse()

	if *logPath == "" {
		*logPath = "/tmp/access.log"
	}
	if *alertThreshold == 0.0 {
		*alertThreshold = 10.0
	}

	timeseries, err := NewTimeseries(120 * time.Second)
	if err != nil {
		log.Fatal(err)
	}

	ctx, done := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		signalChannel := make(chan os.Signal, 1)
		signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)

		select {
		case sig := <-signalChannel:
			log.Info("Got signal ", sig.String())
			done()
		case <-gctx.Done():
			// A goroutine in the group returned an error, exit this goroutine
			log.Info("closing signal goroutine")
			return gctx.Err()
		}

		return nil
	})

	lines := make(chan string)
	g.Go(func() error {
		return readlines(gctx, *logPath, lines)
	})

	hits := make(chan HTTPHit)
	g.Go(func() error {
		return parseHitLines(lines, hits)
	})

	g.Go(func() error {
		return aggregateHits(hits, timeseries)
	})

	g.Go(func() error {
		return printStats(gctx, 10*time.Second, timeseries)
	})

	alerter := Alerter{
		tsdb: timeseries, threshold: *alertThreshold, period: 120 * time.Second,
		evaluateEvery: time.Second, out: log.Writer(),
	}
	g.Go(func() error {
		return alerter.PrintAlertStateChange(gctx)
	})

	log.Infof("Monitor started: file=%q,threshold=%1.f", *logPath, *alertThreshold)

	// wait for all errgroup goroutines
	if err := g.Wait(); err == nil || err == context.Canceled {
		log.Info("finished clean")
	} else {
		log.Errorf("received error: %v", err)
	}

}
