package main

import "testing"

// TestGroupMedians covers the contract: per (scenario, lib) median across
// repeated samples, ascending sort by value within each scenario, and
// scenario isolation.
func TestGroupMedians(t *testing.T) {
	samples := []Sample{
		// Two scenarios, two libs each, three samples per pair.
		{Scenario: "A", Lib: "fast", NsPerOp: 100},
		{Scenario: "A", Lib: "fast", NsPerOp: 110},
		{Scenario: "A", Lib: "fast", NsPerOp: 120}, // median 110
		{Scenario: "A", Lib: "slow", NsPerOp: 500},
		{Scenario: "A", Lib: "slow", NsPerOp: 510},
		{Scenario: "A", Lib: "slow", NsPerOp: 520}, // median 510
		{Scenario: "B", Lib: "only", NsPerOp: 1000},
		{Scenario: "B", Lib: "only", NsPerOp: 2000}, // median 1500 (even count)
	}
	got := GroupMedians(samples, func(s Sample) float64 { return s.NsPerOp })

	if len(got) != 2 {
		t.Fatalf("got %d scenarios, want 2", len(got))
	}

	a := got["A"]
	if len(a) != 2 || a[0].Label != "fast" || a[1].Label != "slow" {
		t.Errorf("scenario A wrong order or count: %+v", a)
	}
	if a[0].Value != 110 || a[1].Value != 510 {
		t.Errorf("scenario A wrong medians: %+v", a)
	}

	b := got["B"]
	if len(b) != 1 || b[0].Value != 1500 {
		t.Errorf("scenario B median wrong (expected mean of two for even count): %+v", b)
	}
}

func TestGroupMedians_Empty(t *testing.T) {
	got := GroupMedians(nil, func(s Sample) float64 { return s.NsPerOp })
	if len(got) != 0 {
		t.Errorf("empty input should produce empty map, got %+v", got)
	}
}
