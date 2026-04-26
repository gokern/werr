//go:build !race

package werr_test

// raceEnabled reports whether the test binary was built with -race.
// Lets call-site tests detect the case where coverage instrumentation
// is active but race is off — that combination shifts line numbers in
// PC-resolved frames, breaking the //go:embed @trace assertions in
// TestWrap_callSites.
const raceEnabled = false
