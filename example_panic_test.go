package werr_test

import (
	"fmt"
	"strings"

	"github.com/gokern/werr"
)

// Recover is a defer helper that catches a panic and stores a
// werr-wrapped representation of it in *target. It must appear directly
// in a defer statement, not inside another closure: recover() only
// catches a panic when called from the deferred function itself.
//
// Pass nil to swallow the panic without recording it.
func ExampleRecover() {
	//nolint:nonamedreturns // werr.Recover writes into the named error return.
	mustParse := func() (n int, err error) {
		defer werr.Recover(&err)

		var s []int

		_ = s[1] // panic: index out of range

		return n, nil
	}

	_, err := mustParse()
	fmt.Println(strings.Contains(err.Error(), "index out of range"))

	// Output:
	// true
}

// PanicToError is the explicit primitive for converting a recovered
// panic into a werr.Error. Use it when you need control over the
// recover() site yourself (Recover handles the defer/recover pattern
// for you).
func ExamplePanicToError() {
	var got error

	func() {
		defer func() {
			if r := recover(); r != nil {
				got = werr.PanicToError(r)
			}
		}()

		panic("boom")
	}()

	fmt.Println(strings.Contains(got.Error(), "boom"))

	// Output:
	// true
}
