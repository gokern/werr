package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Sample is one parsed row from a `go test -bench` RESULTS.txt: a single
// measurement of one library running one scenario. Multiple samples per
// (Scenario, Lib) pair are reduced to medians elsewhere. They stay
// separate at parse time so the aggregator can pick whichever metric it
// wants.
//
// Metrics holds custom metrics emitted via `b.ReportMetric(value, "unit")`,
// for example `live-B/err` from BenchmarkFootprint. Built-in `ns/op` and
// `B/op` get their own fields because every benchmark emits them and the
// chart catalogue addresses them by name far more often than custom ones.
type Sample struct {
	Scenario   string
	Lib        string
	NsPerOp    float64
	BytesPerOp float64
	Metrics    map[string]float64
}

// ParseResults reads `go test -bench` output and returns every Benchmark
// row it can decode. Lines that don't match the expected shape are silently
// skipped so RESULTS.txt headers, blank lines and `PASS` / `ok` footers
// don't need a separate filter pass.
func ParseResults(path string) ([]Sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only file; close error is irrelevant

	var out []Sample
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		s, ok := parseLine(sc.Text())
		if ok {
			out = append(out, s)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return out, nil
}

// parseLine decodes one `go test -bench` result row. The shape is:
//
//	BenchmarkScenario_Lib-N  iters  ns ns/op  [B B/op  allocs allocs/op]  [<v> <unit> ...]
//
// Returns ok=false for any line that isn't a benchmark result.
func parseLine(line string) (Sample, bool) {
	if !strings.HasPrefix(line, "Benchmark") {
		return Sample{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 4 || fields[3] != "ns/op" {
		return Sample{}, false
	}

	scenario, lib, ok := splitName(fields[0])
	if !ok {
		return Sample{}, false
	}
	ns, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return Sample{}, false
	}

	s := Sample{Scenario: scenario, Lib: lib, NsPerOp: ns}

	// Walk the remaining (value, unit) pairs starting after `ns ns/op`.
	// Built-in B/op is hoisted into its own field; everything else lands
	// in s.Metrics so the chart catalogue can address them by unit name.
	for i := 4; i+1 < len(fields); i += 2 {
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			continue
		}
		unit := fields[i+1]
		switch unit {
		case "B/op":
			s.BytesPerOp = v
		case "allocs/op":
			// allocs/op isn't currently charted; skip.
		default:
			if s.Metrics == nil {
				s.Metrics = map[string]float64{}
			}
			s.Metrics[unit] = v
		}
	}
	return s, true
}

// splitName breaks `BenchmarkScenario_Lib-N` into its scenario and lib parts.
// The trailing `-N` is the GOMAXPROCS suffix that `go test -bench` always
// appends; we drop it. Anything else is a parse miss.
func splitName(raw string) (scenario, lib string, ok bool) {
	name := strings.TrimPrefix(raw, "Benchmark")
	if i := strings.LastIndexByte(name, '-'); i >= 0 {
		if _, err := strconv.Atoi(name[i+1:]); err == nil {
			name = name[:i]
		}
	}
	scenario, lib, ok = strings.Cut(name, "_")
	return scenario, lib, ok
}
