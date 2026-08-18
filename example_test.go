package werr_test

import (
	"errors"
	"fmt"

	"github.com/gokern/werr/v2"
)

// Wrap captures the call site of every layer that touches the error and
// stays interoperable with the standard errors package: errors.Is and
// errors.As traverse a werr chain transparently.
func Example() {
	errNotFound := errors.New("not found")

	loadConfig := func() error {
		return werr.Wrapf(errNotFound, "loading config")
	}

	err := loadConfig()
	if err != nil {
		// Sentinel comparison still works through werr layers.
		fmt.Println("not found:", errors.Is(err, errNotFound))
	}

	// Output:
	// not found: true
}
