package main

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// Row holds one second worth of HTTP traffic
type Row struct {
	time           time.Time
	totalHits      int
	totalBits      int
	perSectionHits map[sectionName]int
}

func NewRow(time time.Time) Row {
	return Row{time: time, perSectionHits: make(map[sectionName]int)}
}

// Timeseries is a timeseries with round second precision: time is truncated to a multiple of second.
// It also has a retention policy: old data points are discarded.
// TODO(jordanP): profile and see whether a RingBuffer would make sense
type Timeseries struct {
	sync.RWMutex
	retention time.Duration
	s         []Row
}

func NewTimeseries(retention time.Duration) (*Timeseries, error) {
	if retention.Truncate(time.Second) != retention {
		return nil, errors.New("retention period must be a multiple of time.Second")
	}
	return &Timeseries{
		retention: retention,
		s:         make([]Row, 0, retention/time.Second),
	}, nil
}

// insert inserts a row in the TSDB and delete expired rows
func (ts *Timeseries) insert(row Row) {
	ts.Lock()
	ts.s = append(ts.s, row)
	for time.Since(ts.s[0].time) > ts.retention {
		ts.s = ts.s[1:]
	}
	ts.Unlock()
}

func (ts *Timeseries) GetRowsSince(since time.Duration) []Row {
	var rows []Row
	ts.RLock()
	defer ts.RUnlock()
	// Time since `since` could possibly not be a round number of seconds. We need to decide whether to count
	// all or none of the hits that happened during that split second. Here we decide to include them all, hence
	// we Truncate(...)
	for i := len(ts.s) - 1; i >= 0 && time.Since(ts.s[i].time).Truncate(time.Second) <= since; i-- {
		rows = append(rows, ts.s[i])
	}
	return rows
}

// GetAvgRate uses named return values so the method's signature makes it clear which is which
func (ts *Timeseries) GetAvgRate(since time.Duration) (hits float64, bytes float64) {
	for _, row := range ts.GetRowsSince(since) {
		hits += float64(row.totalHits)
		bytes += float64(row.totalBits)
	}
	secs := float64(since / time.Second)
	return hits / secs, bytes / secs
}

// TopNSectionsSince gets the top N sections in term of hits count over the given duration
func (ts *Timeseries) TopNSectionsSince(n int, since time.Duration) []SectionStats {
	// Aggregate
	mergedStats := make(map[sectionName]int)
	for _, row := range ts.GetRowsSince(since) {
		for sectionName, hitCount := range row.perSectionHits {
			mergedStats[sectionName] += hitCount
		}
	}

	// Sort
	allSections := make([]SectionStats, 0, len(mergedStats))
	for section, hit := range mergedStats {
		allSections = append(allSections, SectionStats{sectionName: sectionName(section), hits: hit})
	}
	sort.Slice(allSections, func(i, j int) bool {
		return allSections[i].hits > allSections[j].hits
	})

	// Filter
	if len(allSections) <= n {
		return allSections
	}
	return allSections[:n]
}

type SectionStats struct {
	sectionName sectionName
	hits        int
}

func (s SectionStats) String() string {
	return fmt.Sprintf("'Section %s got %d hits'", s.sectionName, s.hits)
}
