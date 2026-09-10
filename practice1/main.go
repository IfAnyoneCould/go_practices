package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	date        string
	category    string
	description string
	amount      int
}

type Entries []Entry

type Error struct {
	line   int
	reason string
}

func (e Error) Error() string {
	return fmt.Sprintf("Error %s at line %d\n", e.reason, e.line+1)
}

func NewError(line int, reason string) *Error {
	return &Error{line: line, reason: reason}
}

func load(ctx context.Context, path string) (Entries, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	file, err := os.Open(path)

	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer file.Close()

	var entries []Entry

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		err := ctx.Err()
		if err != nil {
			return nil, err
		}

		line := scanner.Text()

		entry, err := parseLine(line, lineNum)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
		lineNum++
	}

	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	return entries, nil
}

type loadInfo struct {
	index   int
	entries Entries
	err     error
}

func loadAll(ctx context.Context, paths []string) ([]Entries, error) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var ch = make(chan loadInfo, len(paths))

	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()

			result, err := load(ctx, p)

			select {
			case ch <- loadInfo{i, result, err}:
			case <-ctx.Done():
			}
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	entries := make([]Entries, len(paths))
	var errs []error

	for l := range ch {
		if l.err != nil {
			errs = append(errs, l.err)
			cancel()
			continue
		}
		entries[l.index] = l.entries
	}

	err := errors.Join(errs...)
	if err != nil {
		return nil, err
	}

	return entries, nil

}

func parseLine(line string, lineNum int) (Entry, error) {
	vals := strings.Split(line, ",")

	for i := range vals {
		vals[i] = strings.TrimSpace(vals[i])
	}

	if len(vals) != 4 {
		return Entry{}, NewError(lineNum, "incorrect number of arguments")
	}

	_, err := time.Parse("2006-01-02", vals[0])
	if err != nil {
		return Entry{}, NewError(lineNum, "malformed date. expected format: YYYY-MM-DD")
	}

	dollars, err := strconv.ParseFloat(vals[3], 64)
	if err != nil {
		return Entry{}, NewError(lineNum, "malformed amount integer")
	}
	amount := int(math.Round(dollars * 100))

	return Entry{vals[0], vals[1], vals[2], amount}, nil

}

func (e Entries) byCategory(cat string) (Entries, error) {

	var inCat []Entry

	for _, entry := range e {
		if entry.category == cat {
			inCat = append(inCat, entry)
		}
	}

	if len(inCat) == 0 {
		err := fmt.Errorf("category error: %s not found", cat)
		return nil, err
	}

	return inCat, nil
}

func (e Entries) totals() map[string]int {
	m := make(map[string]int)
	for _, entry := range e {
		m[entry.category] += entry.amount
	}
	return m
}

type Reporter interface {
	Report(e Entries)
}

type SummaryReporter struct{}

func (r SummaryReporter) Report(e Entries) {
	m := e.totals()

	var keys []string
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	sum := 0
	for _, k := range keys {
		sum += m[k]
		fmt.Printf("Category: %-14s Total expense: %d.%02d\n", k, m[k]/100, m[k]%100)
	}
	fmt.Printf("Total: %d.%02d\n", sum/100, sum%100)
}

type DetailedReporter struct{}

func (r DetailedReporter) Report(e Entries) {
	for _, entry := range e {
		fmt.Printf("Category: %s Expense: %d.%02d\n", entry.category, entry.amount/100, entry.amount%100)
	}
}

func main() {

	ctx := context.Background()

	all, err := loadAll(ctx, []string{"testdata.txt", "more.txt"})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var merged Entries
	for _, e := range all {
		merged = append(merged, e...)
	}

	for _, r := range []Reporter{SummaryReporter{}, DetailedReporter{}} {
		r.Report(merged)
	}
}
