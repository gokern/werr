package main

import (
	"slices"
	"sort"
)

// Bar is one row of a horizontal-bar chart, the input shape Render expects.
type Bar struct {
	Label string
	Value float64
}

// GroupMedians reduces a flat sample list into chart-ready bars: for every
// (scenario, lib) pair it computes the median of `pick(sample)` across all
// samples for that pair, then sorts each scenario's bars ascending by value.
// This is the single hop between parser output and Render input.
func GroupMedians(samples []Sample, pick func(Sample) float64) map[string][]Bar {
	type key struct{ scenario, lib string }
	buckets := map[key][]float64{}
	for _, s := range samples {
		k := key{s.Scenario, s.Lib}
		buckets[k] = append(buckets[k], pick(s))
	}
	out := map[string][]Bar{}
	for k, vs := range buckets {
		out[k.scenario] = append(out[k.scenario], Bar{Label: k.lib, Value: median(vs)})
	}
	for sc := range out {
		sort.Slice(out[sc], func(i, j int) bool { return out[sc][i].Value < out[sc][j].Value })
	}
	return out
}

// median returns the middle value of `s` (mean of the two middle values for
// even-length input). Does not mutate `s`. Returns 0 for an empty slice.
func median(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	cp := slices.Clone(s)
	slices.Sort(cp)
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}
