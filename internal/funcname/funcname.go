// Package funcname splits a runtime function name into its package path
// and function identifier.
package funcname

import "strings"

// Split separates a fully-qualified runtime function name into its package
// path and function identifier. Unlike a naive last-dot split, it handles
// methods on pointer receivers correctly:
//
//	"main.main"                          -> "main", "main"
//	"github.com/foo/bar.Func"            -> "github.com/foo/bar", "Func"
//	"github.com/foo/bar.(*Type).Method"  -> "github.com/foo/bar", "(*Type).Method"
//	"github.com/foo/bar.Type.Method"     -> "github.com/foo/bar", "Type.Method"
//	"runtime.gopanic"                    -> "runtime", "gopanic"
//	"noPackage"                          -> "", "noPackage"
//	""                                   -> "", ""
//
//nolint:nonamedreturns // documents the two-value contract in godoc.
func Split(full string) (pkg, fn string) {
	slash := strings.LastIndexByte(full, '/')
	rel := full[slash+1:]

	dot := strings.IndexByte(rel, '.')
	if dot < 0 {
		return "", full
	}

	cut := slash + 1 + dot

	return full[:cut], full[cut+1:]
}
