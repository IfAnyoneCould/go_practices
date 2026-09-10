package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LogLine struct {
	timeStamp time.Time
	lvl       string
	service   string
	latency   int
	message   string
}

type LogLines []LogLine
type ParseError struct {
	file string
	line int
	err  error
}

func (e ParseError) Error() string {
	return fmt.Sprintf("error in file %s at line %d: %s", e.file, e.line, e.err)
}

func newParseError(file string, line int, err error) ParseError { return ParseError{file, line, err} }

type Stats struct {
	levelCounts  map[string]int
	serviceCount map[string]int
	serviceSum   map[string]int
	serviceMax   map[string]int
	malformed    int
}

func newStats() *Stats {
	return &Stats{
		levelCounts:  make(map[string]int),
		serviceCount: make(map[string]int),
		serviceSum:   make(map[string]int),
		serviceMax:   make(map[string]int),
		malformed:    0,
	}
}

func parseLine(line string, lineNum int) (LogLine, error) {
	vals := strings.SplitN(line, " ", 5)

	if len(vals) < 5 {
		return LogLine{}, fmt.Errorf("expected 5 fields got %d", len(vals))
	}
	for i := range 4 {
		vals[i] = strings.TrimSpace(vals[i])
	}

	t, err := time.Parse(time.RFC3339, vals[0])
	if err != nil {
		return LogLine{}, err
	}

	ping, err := strconv.Atoi(vals[3])
	if err != nil {
		return LogLine{}, err
	}

	return LogLine{t, vals[1], vals[2], ping, vals[4]}, nil
}

func processFile(ctx context.Context, path string) (*Stats, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lineNum := 0
	stats := newStats()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line := scanner.Text()

		log, err := parseLine(line, lineNum)

		if err != nil {
			stats.malformed++
		} else {
			stats.levelCounts[log.lvl]++
			stats.serviceCount[log.service]++
			stats.serviceSum[log.service] += log.latency
			if log.latency > stats.serviceMax[log.service] {
				stats.serviceMax[log.service] = log.latency
			}
		}

		lineNum++
	}

	return stats, nil

}

func merge(stats []*Stats) *Stats {
	stat := newStats()

	for _, s := range stats {
		stat.malformed += s.malformed
		for k, v := range s.serviceSum {
			stat.serviceSum[k] += v
		}
		for k, v := range s.serviceCount {
			stat.serviceCount[k] += v
		}
		for k, v := range s.levelCounts {
			stat.levelCounts[k] += v
		}
		for k := range s.serviceMax {
			if s.serviceMax[k] > stat.serviceMax[k] {
				stat.serviceMax[k] = s.serviceMax[k]
			}
		}
	}

	return stat
}

func analyze(ctx context.Context, paths []string, numWorkers int) (*Stats, error) {

	jobs := make(chan string, len(paths))
	results := make(chan *Stats, len(paths))
	errs := make(chan error, len(paths))

	for _, p := range paths {
		jobs <- p
	}
	close(jobs)

	var wg = sync.WaitGroup{}
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				r, err := processFile(ctx, p)
				if err != nil {
					errs <- err
					continue

				}
				select {
				case results <- r:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
		close(errs)
	}()

	var errList []error
	errDone := make(chan struct{})
	go func() {
		for e := range errs {
			errList = append(errList, e)
		}
		close(errDone)
	}()

	var stats []*Stats
	for s := range results {
		stats = append(stats, s)
	}

	<-errDone

	result := merge(stats)
	if err := errors.Join(errList...); err != nil {
		return result, err
	}

	return result, nil

}

func main() {
	paths := []string{"node-a.log", "node-b.log", "node-c.log", "node-d.log", "node-e.log"}
	ctx := context.Background()
	stats, err := analyze(ctx, paths, 5)
	if err != nil {
		fmt.Printf("Errors: %s\n\n", err)
	}
	for k, v := range stats.serviceMax {
		fmt.Printf("ServiceMax on %s: %d\n", k, v)
	}
	fmt.Println()
	for k, v := range stats.levelCounts {
		fmt.Printf("LevelsCounts on %s: %d\n", k, v)
	}
	fmt.Println()
	for k, v := range stats.serviceCount {
		fmt.Printf("ServiceCount on %s: %d\n", k, v)
	}
	fmt.Println()
	for k, v := range stats.serviceSum {
		fmt.Printf("ServiceSum on %s: %d\n", k, v)
	}
	fmt.Println()
	fmt.Printf("Malformed lines: %d", stats.malformed)
}
