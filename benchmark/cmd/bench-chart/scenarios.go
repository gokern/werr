package main

// Scenario is one chart definition: which scenario to read from the parsed
// samples, which metric to plot, and the visual config Render needs. The
// dispatch loop in run() iterates a slice of these so adding a new chart is
// one entry, not a new branch in main.
type Scenario struct {
	File       string                  // output filename in outDir
	Scenario   string                  // sample.Scenario key
	Pick       func(Sample) float64    // metric extractor
	Title      string
	Subtitle   string
	Cutoff     float64
	Formatter  Formatter
	OverflowID string // unique <pattern> id per chart so SVGs can be inlined together
}

// scenarios is the full catalogue of charts the binary knows how to draw.
// Cutoffs bound the linear scale so that one pathological library cannot
// squash every other bar into invisibility; anything above a cutoff is drawn
// off-scale below a divider. They are upper guards, not promises: when no
// library exceeds one, Render simply scales to the largest real value and
// draws no divider at all. As of the current results only Footprint still
// overflows — oops used to blow out the other three and stopped doing so in
// v1.23, so those cutoffs now sit unused above the data.
//
//nolint:gochecknoglobals
var scenarios = []Scenario{
	{
		File:       "realistic.svg",
		Scenario:   "Realistic",
		Pick:       func(s Sample) float64 { return s.NsPerOp },
		Title:      "Realistic — full request lifecycle (time)",
		Subtitle:   "ns/op (linear, lower is better) — werr highlighted",
		Cutoff:     25_000, // 25 µs; nothing overflows today (slowest is oops at ~12 µs)
		Formatter:  TimeFormatter,
		OverflowID: "ovf-time",
	},
	{
		File:       "realistic_bytes.svg",
		Scenario:   "Realistic",
		Pick:       func(s Sample) float64 { return s.BytesPerOp },
		Title:      "Realistic — memory per iteration",
		Subtitle:   "B/op (linear, lower is better) — werr highlighted",
		Cutoff:     20_000, // 20 KB; nothing overflows today (heaviest is mdobak at ~6.5 KB)
		Formatter:  BytesFormatter,
		OverflowID: "ovf-bytes",
	},
	{
		File:       "slog.svg",
		Scenario:   "SlogJSON",
		Pick:       func(s Sample) float64 { return s.NsPerOp },
		Title:      "slog.JSONHandler — structured logging cost",
		Subtitle:   "ns/op (linear, lower is better) — werr highlighted",
		Cutoff:     20_000, // 20 µs; nothing overflows today (slowest is oops at ~9 µs)
		Formatter:  TimeFormatter,
		OverflowID: "ovf-slog",
	},
	{
		File:       "footprint.svg",
		Scenario:   "Footprint",
		Pick:       func(s Sample) float64 { return s.Metrics["live-B/err"] },
		Title:      "Footprint — live bytes per error (steady-state)",
		Subtitle:   "live-B/err (linear, lower is better) — werr highlighted",
		Cutoff:     600, // keeps werr/errtrace/stdlib visible; pushes the 5 heaviest libs to overflow
		Formatter:  BytesFormatter,
		OverflowID: "ovf-foot",
	},
}

// chartConfig converts a Scenario plus its already-grouped bars into the
// visual config Render needs. Splitting this from Scenario keeps Scenario
// fully data-only (no Bar slice) so the catalogue is readable.
func (sc Scenario) chartConfig(items []Bar) ChartConfig {
	return ChartConfig{
		Title:      sc.Title,
		Subtitle:   sc.Subtitle,
		Items:      items,
		Highlight:  "werr",
		Cutoff:     sc.Cutoff,
		Formatter:  sc.Formatter,
		OverflowID: sc.OverflowID,
	}
}
