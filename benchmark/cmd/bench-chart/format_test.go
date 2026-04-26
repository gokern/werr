package main

import "testing"

// TestTimeFormatter pins the unit-transition thresholds (999 → ns,
// 1000 → µs, 999_999 → µs, 1_000_000 → ms). Picking the wrong unit is
// one of the easiest ways to ship a misleading chart, and these
// boundaries are the part most likely to drift on a refactor.
func TestTimeFormatter(t *testing.T) {
	cases := []struct {
		max  float64
		unit string
	}{
		{500, "ns"},
		{999, "ns"},
		{1_000, "µs"},
		{999_999, "µs"},
		{1_000_000, "ms"},
	}
	for _, c := range cases {
		if u := TimeFormatter.PickUnit(c.max); u != c.unit {
			t.Errorf("PickUnit(%v) = %q, want %q", c.max, u, c.unit)
		}
	}

	if got := TimeFormatter.Format(1_500, "µs"); got != "1.5 µs" {
		t.Errorf("Format(1500, µs) = %q, want %q", got, "1.5 µs")
	}
	if got := TimeFormatter.Format(2_500_000, "ms"); got != "2.5 ms" {
		t.Errorf("Format(2_500_000, ms) = %q, want %q", got, "2.5 ms")
	}
}

func TestBytesFormatter(t *testing.T) {
	cases := []struct {
		max  float64
		unit string
	}{
		{500, "B"},
		{1_023, "B"},
		{1_024, "KB"},
		{1_048_575, "KB"},
		{1_048_576, "MB"},
	}
	for _, c := range cases {
		if u := BytesFormatter.PickUnit(c.max); u != c.unit {
			t.Errorf("PickUnit(%v) = %q, want %q", c.max, u, c.unit)
		}
	}

	if got := BytesFormatter.Format(2_048, "KB"); got != "2.0 KB" {
		t.Errorf("Format(2048, KB) = %q, want %q", got, "2.0 KB")
	}
}
