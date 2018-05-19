package main

import (
	"fmt"
	"os"
)

func _error(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func assert(expr bool) {
	if !expr {
		_error("Assertion failed.s")
	}
}

func warn(format string, args ...interface{}) {
	fmt.Fprint(os.Stderr, "warning: ")
	fmt.Fprintf(os.Stderr, format, args...)
}

func quote_cstring(sval Cstring) string {
	var s string
	for _, c := range sval {
		if c == byte(0) {
			break
		}
		if c == '"' || c == '\\' {
			s += "\\"
		}
		s += fmt.Sprintf("%c", c)
	}
	return s
}
