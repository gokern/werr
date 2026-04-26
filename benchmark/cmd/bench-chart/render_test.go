package main

import (
	"strings"
	"testing"
)

// TestRender_inScopeAndOverflow pins Render's high-level invariants:
// in-scope bars get the highlight color when the label matches, overflow
// bars sit below the divider with the muted palette, and the off-scale
// label encodes both the cutoff and the actual value.
//
// The test inspects substrings, not full markup, so palette tuning or
// geometry tweaks don't force a corpus rewrite. Only contract changes
// trip the assertions.
func TestRender_inScopeAndOverflow(t *testing.T) {
	cfg := ChartConfig{
		Title:    "test",
		Subtitle: "ns/op",
		Items: []Bar{
			{Label: "fast", Value: 100},
			{Label: "werr", Value: 200},
			{Label: "slow", Value: 50_000}, // overflows
		},
		Highlight:  "werr",
		Cutoff:     1_000,
		Formatter:  TimeFormatter,
		OverflowID: "ovf-test",
	}
	got := Render(cfg)

	for _, want := range []string{
		`<svg`,
		`</svg>`,
		`>fast<`,
		`>werr<`,
		`>slow<`,
		highlightColor,    // applied to the werr bar
		defaultBarColor,   // applied to the fast bar
		`url(#ovf-test)`,  // overflow bar uses the pattern
		`off-scale ↓`,     // divider label is present
		`> 1000 ns`,  // cutoff is shown in the chart's in-scope unit
		`(50.0 µs)`,  // actual overflow value uses its own most-readable unit
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output", want)
		}
	}
}

// TestRender_allInScope checks that a chart with no overflow does not
// emit the divider. Capping geometry must not leak through when nothing
// was capped.
func TestRender_allInScope(t *testing.T) {
	cfg := ChartConfig{
		Title:      "test",
		Subtitle:   "ns/op",
		Items:      []Bar{{Label: "a", Value: 100}, {Label: "b", Value: 200}},
		Cutoff:     1_000,
		Formatter:  TimeFormatter,
		OverflowID: "ovf-test",
	}
	got := Render(cfg)
	if strings.Contains(got, "off-scale") {
		t.Error("expected no off-scale divider when nothing overflowed")
	}
}

// TestSplitAtCutoff verifies the partition is ascending-aware: items must be
// pre-sorted, and the index returned is the first one strictly greater than
// the cutoff (cutoff itself stays in-scope).
func TestSplitAtCutoff(t *testing.T) {
	items := []Bar{{Value: 1}, {Value: 5}, {Value: 10}, {Value: 100}}
	cases := []struct {
		cutoff float64
		want   int
	}{
		{cutoff: 0, want: 0},   // everything overflows
		{cutoff: 5, want: 2},   // 1 and 5 stay in-scope
		{cutoff: 10, want: 3},  // 1, 5, 10 stay in-scope
		{cutoff: 200, want: 4}, // nothing overflows
	}
	for _, c := range cases {
		if got := splitAtCutoff(items, c.cutoff); got != c.want {
			t.Errorf("splitAtCutoff(_, %v) = %d, want %d", c.cutoff, got, c.want)
		}
	}
}

func TestMedian(t *testing.T) {
	cases := []struct {
		in   []float64
		want float64
	}{
		{nil, 0},
		{[]float64{5}, 5},
		{[]float64{1, 3, 2}, 2},      // odd, returns middle after sort
		{[]float64{4, 1, 3, 2}, 2.5}, // even, mean of middle two
	}
	for _, c := range cases {
		if got := median(c.in); got != c.want {
			t.Errorf("median(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
