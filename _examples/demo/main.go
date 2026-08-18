// Demo program for github.com/gokern/werr. Run from the repo root:
//
//	go run ./_examples/demo
//
// Walks the public API and prints the real output for each capability —
// godoc Examples can't show file paths and line numbers because they are
// runtime-dependent.
package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gokern/panics"

	"github.com/gokern/werr/v2"
)

// User stands in for a domain type; the chain is what's on display.
type User struct{ ID int }

// A layered call chain mimicking a real application. Each layer carries
// //go:noinline so the rendered call stack matches the declared functions
// one-to-one — otherwise the compiler may inline repository() into
// service() and the runtime reports them sharing a frame.

//go:noinline
func repository() (User, error) {
	return User{}, werr.Wrap(io.EOF)
}

//go:noinline
func service() (User, error) {
	return werr.Wrap2(repository()) // Wrap2: forward (T, error) in one line
}

//go:noinline
func usecase() (User, error) {
	u, err := service()

	return u, werr.Wrapf(err, "load user profile")
}

//go:noinline
func handler() error {
	_, err := usecase()
	if err != nil {
		return werr.Wrapf(err, "GET /users/%d", 42)
	}

	return nil
}

//nolint:nonamedreturns // divide-by-zero relies on the zero value of the named return.
func divideByZero() (n int) { return 1 / n }

// werr does not recover panics; panics.Catch does, and werr wraps the result
// with the call site that was handling the request when it blew up.
func mustParse() (int, error) {
	var out int

	if p := panics.Catch(func() { out = divideByZero() }); p != nil {
		return 0, werr.Wrapf(p, "parsing user input")
	}

	return out, nil
}

func main() {
	demoPretty()
	demoOneLine()
	demoAsWrap()
	demoWalk()
	demoErrorsIs()
	demoRecoveredPanic()
	demoCustomFormatter()
}

// --- 1. Default formatter (Pretty) ----------------------------------------

func demoPretty() {
	header("Pretty (default formatter, dev-friendly)")

	err := handler()
	fmt.Println(err)
}

// --- 2. OneLine for grep/Loki ---------------------------------------------

func demoOneLine() {
	header("OneLine (grep / Loki / ELK)")

	werr.SetOneLineFormatter()

	defer werr.SetPrettyFormatter()

	fmt.Println(handler())
}

// --- 3. AsWrap to read frame metadata -------------------------------------

func demoAsWrap() {
	header("AsWrap — extract frame metadata")

	err := handler()

	if w, ok := werr.AsWrap(err); ok {
		fmt.Printf("File:     %s\n", w.File())
		fmt.Printf("Line:     %d\n", w.Line())
		fmt.Printf("FuncName: %s\n", w.FuncName())
		fmt.Printf("Message:  %s\n", w.Message())
		fmt.Printf("PC:       %#x\n", w.PC())
	}
}

// --- 4. Walk to iterate every werr layer ----------------------------------

func demoWalk() {
	header("Walk — visit every werr layer outermost to innermost")

	err := handler()

	_ = werr.Walk(err, func(f werr.Frame) bool {
		msg := f.Msg
		if msg == "" {
			msg = "(no message)"
		}

		fmt.Printf("  • %s @ %s:%d — %s\n", f.FuncName, f.File, f.Line, msg)

		return true
	})
}

// --- 5. Standard library interop (errors.Is) ------------------------------

func demoErrorsIs() {
	header("errors.Is — sentinel comparison through the chain")

	err := handler()
	fmt.Printf("errors.Is(err, io.EOF) = %v\n", errors.Is(err, io.EOF))
}

// --- 6. Panic recovery ----------------------------------------------------

func demoRecoveredPanic() {
	header("Recovered panic — panics.Catch contains it, werr renders it")

	_, err := mustParse()
	fmt.Println(err)
}

// --- 7. Custom formatter --------------------------------------------------

func demoCustomFormatter() {
	header("Custom FormatFn — your own log shape")

	werr.SetFormatter(func(frames []werr.Frame, leaf error) string {
		var (
			out      string
			outSb123 strings.Builder
		)

		for i, f := range frames {
			fmt.Fprintf(&outSb123, "[%d] %s", i+1, f.FuncName)

			if f.Msg != "" {
				outSb123.WriteString(": " + f.Msg)
			}

			outSb123.WriteString("\n")
		}

		out += outSb123.String()

		if leaf != nil {
			out += "↳ " + leaf.Error()
		}

		return out
	})

	defer werr.SetPrettyFormatter()

	fmt.Println(handler())
}

// --- helper ---------------------------------------------------------------

func header(title string) {
	fmt.Println()
	fmt.Println("=== " + title + " ===")
}
