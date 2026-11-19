package report

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON writes exactly one JSON document followed by a newline. Nothing
// else is written to the stream; progress and warnings go to stderr.
func WriteJSON(w io.Writer, document Document) error {
	encoder := json.NewEncoder(checkedWriter{output: w})
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("write json report: %w", err)
	}
	return nil
}

type checkedWriter struct{ output io.Writer }

func (w checkedWriter) Write(p []byte) (int, error) {
	n, err := w.output.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
func writeString(w io.Writer, s string) error {
	_, err := checkedWriter{output: w}.Write([]byte(s))
	return err
}
