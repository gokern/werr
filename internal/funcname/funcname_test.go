package funcname

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// testPkg is the sample module path shared by every module-scoped case below.
const testPkg = "github.com/foo/bar"

func TestSplit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      string
		wantPkg string
		wantFn  string
	}{
		{name: "stdlib top-level", in: "main.main", wantPkg: "main", wantFn: "main"},
		{
			name:    "module func",
			in:      testPkg + ".Func",
			wantPkg: testPkg,
			wantFn:  "Func",
		},
		{
			name:    "pointer receiver method",
			in:      testPkg + ".(*Type).Method",
			wantPkg: testPkg,
			wantFn:  "(*Type).Method",
		},
		{
			name:    "value receiver method",
			in:      testPkg + ".Type.Method",
			wantPkg: testPkg,
			wantFn:  "Type.Method",
		},
		{name: "no dot", in: "noPackage", wantPkg: "", wantFn: "noPackage"},
		{name: "empty input", in: "", wantPkg: "", wantFn: ""},
		{name: "trailing slash", in: "trailing/", wantPkg: "", wantFn: "trailing/"},
		{name: "stdlib runtime", in: "runtime.gopanic", wantPkg: "runtime", wantFn: "gopanic"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pkg, fn := Split(tc.in)
			require.Equal(t, tc.wantPkg, pkg)
			require.Equal(t, tc.wantFn, fn)
		})
	}
}

// FuzzSplit must never panic for any byte sequence and must satisfy the
// invariant that joining (pkg, fn) reproduces the input when pkg is non-empty.
func FuzzSplit(f *testing.F) {
	f.Add("github.com/foo/bar.(*T).M")
	f.Add("")
	f.Add(".")
	f.Add("a.b.c.d")
	f.Add("/leading/slash.fn")
	f.Add("trailing/")
	f.Add("\x00")

	f.Fuzz(func(t *testing.T, str string) {
		pkg, fn := Split(str)

		require.True(t, strings.Contains(str, fn) || str == fn,
			"fn must be a substring of input: in=%q pkg=%q fn=%q", str, pkg, fn)

		if pkg != "" {
			require.Equal(t, str, pkg+"."+fn,
				"join must reproduce input when pkg is non-empty: in=%q pkg=%q fn=%q", str, pkg, fn)
		}
	})
}
