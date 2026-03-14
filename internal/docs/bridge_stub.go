//go:build !darwin || !cgo

package docs

import "fmt"

func available() error {
	return fmt.Errorf("%w: semantic docs search requires macOS with cgo enabled", ErrUnavailable)
}

func searchJSON(string, SearchOptions) ([]byte, error) {
	return nil, available()
}

func getJSON(string) ([]byte, error) {
	return nil, available()
}
