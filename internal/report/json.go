package report

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON writes exactly one JSON document followed by a newline. Nothing
// else is written to the stream; progress and warnings go to stderr.
func WriteJSON(w io.Writer, document Document) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("write json report: %w", err)
	}
	return nil
}
