// Command bench-chart reads a `go test -bench` RESULTS.txt and writes SVG bar
// charts into the output directory. Used by `make bench-charts` to refresh
// BENCHMARK.md visuals.
//
// Usage: bench-chart <results.txt> <outdir>
//
// SVGs are hand-rolled (no external charting deps). Outliers (values past the
// per-chart cutoff) are still rendered, but capped to the plot width, drawn
// with a striped overflow pattern, and labelled `> Xµs (actual)` so the
// reader sees both the cutoff and the real number without distorting the
// linear scale used by the in-scope bars.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: bench-chart <results.txt> <outdir>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run is the testable entry point; main() only handles argv and exit
// codes. Iterates the scenario catalogue (see scenarios.go), reduces
// each scenario's metric to median bars, and writes the rendered SVG.
// Scenarios with no matching samples are skipped silently, so a partial
// RESULTS.txt still produces whichever charts it can.
func run(resultsPath, outDir string) error {
	samples, err := ParseResults(resultsPath)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	for _, sc := range scenarios {
		bars := GroupMedians(samples, sc.Pick)[sc.Scenario]
		if len(bars) == 0 {
			continue
		}
		path := filepath.Join(outDir, sc.File)
		svg := Render(sc.chartConfig(bars))
		if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", sc.File, err)
		}
		fmt.Println("wrote", path)
	}
	return nil
}
