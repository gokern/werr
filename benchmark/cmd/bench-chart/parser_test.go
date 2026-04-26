package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseLine covers every shape RESULTS.txt can contain: benchmark
// rows (with and without B/op), header lines, the goos/cpu preamble,
// and the PASS / ok footers. Silent-skip on garbage is part of the
// parser's contract.
func TestParseLine(t *testing.T) {
	cases := []struct {
		name string
		line string
		want Sample
		ok   bool
	}{
		{
			name: "ns_and_bytes",
			line: "BenchmarkRealistic_werr-8        	 1871028	       637.1 ns/op	     627 B/op	       0 allocs/op",
			want: Sample{Scenario: "Realistic", Lib: "werr", NsPerOp: 637.1, BytesPerOp: 627},
			ok:   true,
		},
		{
			name: "ns_only",
			line: "BenchmarkSlogJSON_stdlib-8       	 2494015	       478.3 ns/op",
			want: Sample{Scenario: "SlogJSON", Lib: "stdlib", NsPerOp: 478.3},
			ok:   true,
		},
		{
			name: "no_cpu_suffix",
			line: "BenchmarkRealistic_oops    	    5374	    222436 ns/op	   73200 B/op	    1140 allocs/op",
			want: Sample{Scenario: "Realistic", Lib: "oops", NsPerOp: 222436, BytesPerOp: 73200},
			ok:   true,
		},
		{
			name: "custom_metrics",
			line: "BenchmarkFootprint_werr-8	1380819	      92.11 ns/op	      40.00 header-B	      57.0 live-B/err	      36 B/op	       2 allocs/op",
			want: Sample{Scenario: "Footprint", Lib: "werr", NsPerOp: 92.11, BytesPerOp: 36},
			ok:   true,
		},
		{name: "header_pkg", line: "pkg: github.com/gokern/werr/benchmark"},
		{name: "header_cpu", line: "cpu: Apple M1"},
		{name: "footer_pass", line: "PASS"},
		{name: "footer_ok", line: "ok  	github.com/gokern/werr/benchmark	68.966s"},
		{name: "blank", line: ""},
		{
			name: "missing_underscore",
			line: "BenchmarkNoUnderscore-8        	 1000	       100 ns/op",
		},
		{
			name: "garbled_ns",
			line: "BenchmarkX_y-8        	 100	       abc ns/op",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseLine(c.line)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if !ok {
				return
			}
			if got.Scenario != c.want.Scenario || got.Lib != c.want.Lib ||
				got.NsPerOp != c.want.NsPerOp || got.BytesPerOp != c.want.BytesPerOp {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestParseResults_File checks the file-level integration: temp file with a
// realistic mix of valid + skipped lines, the parser returns only the valid
// rows in order.
func TestParseResults_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "results.txt")
	contents := `goos: darwin
goarch: arm64
pkg: example
cpu: Apple M1
BenchmarkA_x-8	1000	100 ns/op	50 B/op	0 allocs/op
BenchmarkA_y-8	1000	200 ns/op	75 B/op	1 allocs/op
PASS
ok  	example	1.0s
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ParseResults(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d samples, want 2: %+v", len(got), got)
	}
	if got[0].Lib != "x" || got[1].Lib != "y" {
		t.Errorf("ordering broken: %+v", got)
	}
}

func TestParseResults_MissingFile(t *testing.T) {
	if _, err := ParseResults("/nonexistent/results.txt"); err == nil {
		t.Error("expected error for missing file")
	}
}

// TestParseLine_CustomMetric verifies that BenchmarkFootprint's custom
// metric columns (`header-B`, `live-B/err`) are captured in the Metrics
// map. Multiple custom metrics can appear in any position in `go test
// -bench` output as `<value> <unit>` pairs, intermixed with the standard
// ns/op, B/op, allocs/op columns.
func TestParseLine_CustomMetric(t *testing.T) {
	line := "BenchmarkFootprint_werr-8	1380819	      92.11 ns/op	      40.0 header-B	      57.0 live-B/err	      36 B/op	       2 allocs/op"
	got, ok := parseLine(line)
	if !ok {
		t.Fatal("parseLine returned ok=false")
	}
	if v := got.Metrics["live-B/err"]; v != 57.0 {
		t.Errorf("Metrics[\"live-B/err\"] = %v, want 57.0", v)
	}
	if v := got.Metrics["header-B"]; v != 40.0 {
		t.Errorf("Metrics[\"header-B\"] = %v, want 40.0", v)
	}
}
