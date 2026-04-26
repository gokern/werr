package main

import (
	"fmt"
	"html"
	"strings"
)

// SVG geometry. All coordinates are derived from these so any tweak (taller
// rows, wider chart) needs only one edit here.
const (
	chartW   = 820
	rowH     = 26
	labelW   = 130
	valueW   = 130
	titleTop = 28
	axisTop  = 50
	plotTop  = 70
	bottomH  = 16
	dividerH = 22 // extra row height for the off-scale separator
	barInset = 4  // top/bottom inset inside a row

	// barAreaW is the bar plot region: what's left after the fixed-width
	// label gutter on the left and the value gutter on the right.
	barAreaW = chartW - labelW - valueW
)

// Palette. Overflow uses near-background gray so capped bars stay quiet
// instead of reading like alerts.
const (
	bgFill          = "#fafafa"
	titleFill       = "#1a1a1a"
	subtitleFill    = "#666"
	valueFill       = "#444"
	dividerStroke   = "#d4d4d8"
	dividerLabel    = "#888"
	overflowFill    = "#e5e7eb"
	overflowStripe  = "#d1d5db"
	overflowText    = "#9ca3af"
	highlightColor  = "#16a34a"
	defaultBarColor = "#94a3b8"
)

// svgBuilder accumulates SVG fragments. It is a thin wrapper over
// strings.Builder that knows how to draw the shapes Render uses; the
// individual methods exist so Render reads as a sequence of operations
// instead of one long printf cascade.
type svgBuilder struct {
	b strings.Builder
}

func newSVG(w, h int) *svgBuilder {
	s := &svgBuilder{}
	fmt.Fprintf(&s.b,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" `+
			`font-family="-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif" `+
			`font-size="13" fill="%s">`,
		w, h, titleFill,
	)
	return s
}

func (s *svgBuilder) close() string {
	s.b.WriteString(`</svg>`)
	return s.b.String()
}

// defOverflowPattern emits a <defs> block with the diagonal-stripe pattern
// used to fill capped bars. The id must be unique across charts that may
// share a host page.
func (s *svgBuilder) defOverflowPattern(id string) {
	fmt.Fprintf(&s.b,
		`<defs><pattern id="%s" patternUnits="userSpaceOnUse" width="8" height="8" patternTransform="rotate(45)">`+
			`<rect width="8" height="8" fill="%s"/>`+
			`<line x1="0" y1="0" x2="0" y2="8" stroke="%s" stroke-width="3"/>`+
			`</pattern></defs>`,
		id, overflowFill, overflowStripe,
	)
}

func (s *svgBuilder) background(w, h int) {
	fmt.Fprintf(&s.b, `<rect width="%d" height="%d" fill="%s"/>`, w, h, bgFill)
}

func (s *svgBuilder) title(text string) {
	fmt.Fprintf(&s.b, `<text x="16" y="%d" font-size="17" font-weight="700">%s</text>`,
		titleTop, html.EscapeString(text))
}

func (s *svgBuilder) subtitle(text string) {
	fmt.Fprintf(&s.b, `<text x="16" y="%d" font-size="12" fill="%s">%s</text>`,
		axisTop, subtitleFill, html.EscapeString(text))
}

// inScopeBar draws one bar that fits within the linear scale.
func (s *svgBuilder) inScopeBar(row int, it Bar, maxIn float64, highlight string, f Formatter, unit string) {
	y := plotTop + row*rowH
	cy := y + rowH/2
	barLen := (it.Value/maxIn)*float64(barAreaW) + 2

	color := defaultBarColor
	if it.Label == highlight {
		color = highlightColor
	}

	fmt.Fprintf(&s.b,
		`<text x="%d" y="%d" text-anchor="end" dominant-baseline="middle">%s</text>`,
		labelW-10, cy, html.EscapeString(it.Label),
	)
	fmt.Fprintf(&s.b,
		`<rect x="%d" y="%d" width="%.1f" height="%d" fill="%s" rx="2"/>`,
		labelW, y+barInset, barLen, rowH-2*barInset, color,
	)
	fmt.Fprintf(&s.b,
		`<text x="%.1f" y="%d" dominant-baseline="middle" fill="%s">%s</text>`,
		float64(labelW)+barLen+6, cy, valueFill, f.Format(it.Value, unit),
	)
}

// divider draws the dashed off-scale separator just above the overflow
// section.
func (s *svgBuilder) divider(inScopeCount int) {
	y := plotTop + inScopeCount*rowH + dividerH/2
	fmt.Fprintf(&s.b,
		`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1" stroke-dasharray="4 4"/>`,
		labelW, y, chartW-16, y, dividerStroke,
	)
	fmt.Fprintf(&s.b,
		`<text x="%d" y="%d" text-anchor="end" font-size="11" fill="%s" dominant-baseline="middle">off-scale ↓</text>`,
		labelW-10, y, dividerLabel,
	)
}

// overflowBar draws one capped bar in the off-scale section. The label
// shows both the cutoff and the actual value, so the reader sees "this
// bar is at least X" plus the real number.
func (s *svgBuilder) overflowBar(row, inScopeCount int, it Bar, cutoff float64, overflowID string, f Formatter, chartUnit string) {
	y := plotTop + (inScopeCount+row)*rowH + dividerH
	cy := y + rowH/2
	barLen := float64(barAreaW)

	fmt.Fprintf(&s.b,
		`<text x="%d" y="%d" text-anchor="end" dominant-baseline="middle" fill="%s">%s</text>`,
		labelW-10, cy, overflowText, html.EscapeString(it.Label),
	)
	fmt.Fprintf(&s.b,
		`<rect x="%d" y="%d" width="%.1f" height="%d" fill="url(#%s)" rx="2"/>`,
		labelW, y+barInset, barLen, rowH-2*barInset, overflowID,
	)
	actualUnit := f.PickUnit(it.Value)
	label := fmt.Sprintf("> %s (%s)",
		f.Format(cutoff, chartUnit),
		f.Format(it.Value, actualUnit),
	)
	fmt.Fprintf(&s.b,
		`<text x="%.1f" y="%d" dominant-baseline="middle" fill="%s">%s</text>`,
		float64(labelW)+barLen+6, cy, overflowText, label,
	)
}
