package main

// ChartConfig is the input to Render. Everything is plain data; Render
// itself is a pure function of (config) → SVG string, so it is easy to
// unit-test and main.go can describe charts declaratively in one place
// (see scenarios.go).
type ChartConfig struct {
	Title      string
	Subtitle   string
	Items      []Bar // pre-sorted ascending by Value
	Highlight  string
	Cutoff     float64
	Formatter  Formatter
	OverflowID string // unique <pattern> id; required if multiple charts may share a host page
}

// Render produces the SVG string for one chart. Items are split at Cutoff
// into in-scope (drawn on the linear scale) and overflow (drawn at full
// width with a hatched fill below an "off-scale" divider).
func Render(cfg ChartConfig) string {
	cutIdx := splitAtCutoff(cfg.Items, cfg.Cutoff)
	inScope, overflow := cfg.Items[:cutIdx], cfg.Items[cutIdx:]

	maxIn := maxValue(inScope)
	if maxIn == 0 { // degenerate: every item overflowed
		maxIn = cfg.Cutoff
	}
	unit := cfg.Formatter.PickUnit(maxIn)

	height := plotTop + rowH*len(cfg.Items) + bottomH
	if len(overflow) > 0 {
		height += dividerH
	}

	sb := newSVG(chartW, height)
	sb.defOverflowPattern(cfg.OverflowID)
	sb.background(chartW, height)
	sb.title(cfg.Title)
	sb.subtitle(cfg.Subtitle)

	for i, it := range inScope {
		sb.inScopeBar(i, it, maxIn, cfg.Highlight, cfg.Formatter, unit)
	}
	if len(overflow) > 0 {
		sb.divider(len(inScope))
		for i, it := range overflow {
			sb.overflowBar(i, len(inScope), it, cfg.Cutoff, cfg.OverflowID, cfg.Formatter, unit)
		}
	}

	return sb.close()
}

// splitAtCutoff returns the index of the first item whose value exceeds
// cutoff. Items must be sorted ascending; if no item is over the cutoff,
// returns len(items).
func splitAtCutoff(items []Bar, cutoff float64) int {
	for i, it := range items {
		if it.Value > cutoff {
			return i
		}
	}
	return len(items)
}

func maxValue(bars []Bar) float64 {
	var m float64
	for _, b := range bars {
		if b.Value > m {
			m = b.Value
		}
	}
	return m
}
