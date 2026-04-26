package main

import "fmt"

// Formatter pairs a unit picker with a value renderer. A chart picks one
// unit from the largest in-scope value and uses it for every label, so
// "0.6 µs" sits next to "2.6 µs" instead of "570 ns" next to "2.6 µs"
// (where the larger numeric reads as the bigger value).
type Formatter struct {
	// PickUnit returns the unit (e.g. "µs") to use for the whole chart given
	// the largest value to be displayed.
	PickUnit func(maxV float64) string
	// Format renders v in the chosen unit (e.g. "0.6 µs").
	Format func(v float64, unit string) string
}

// TimeFormatter renders nanosecond values with the smallest unit that keeps
// the largest number under 1000.
var TimeFormatter = Formatter{
	PickUnit: func(maxNs float64) string {
		switch {
		case maxNs < 1_000:
			return "ns"
		case maxNs < 1_000_000:
			return "µs"
		default:
			return "ms"
		}
	},
	Format: func(ns float64, unit string) string {
		switch unit {
		case "ns":
			return fmt.Sprintf("%.0f ns", ns)
		case "µs":
			return fmt.Sprintf("%.1f µs", ns/1_000)
		case "ms":
			return fmt.Sprintf("%.1f ms", ns/1_000_000)
		}
		return fmt.Sprintf("%.0f ns", ns)
	},
}

// BytesFormatter renders byte values using binary (KB = 1024) units, the
// convention `go test -bench` uses for B/op output.
var BytesFormatter = Formatter{
	PickUnit: func(maxB float64) string {
		switch {
		case maxB < 1_024:
			return "B"
		case maxB < 1_024*1_024:
			return "KB"
		default:
			return "MB"
		}
	},
	Format: func(b float64, unit string) string {
		switch unit {
		case "B":
			return fmt.Sprintf("%.0f B", b)
		case "KB":
			return fmt.Sprintf("%.1f KB", b/1_024)
		case "MB":
			return fmt.Sprintf("%.2f MB", b/(1_024*1_024))
		}
		return fmt.Sprintf("%.0f B", b)
	},
}
