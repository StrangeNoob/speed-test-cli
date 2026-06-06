package output

import (
	"encoding/json"
	"io"

	"speed-test-cli/internal/speedtest"
)

// JSON writes the result as a single-line JSON object to w.
func JSON(w io.Writer, res speedtest.Result) error {
	enc := json.NewEncoder(w)
	return enc.Encode(res)
}
