package tickets

import (
	"encoding/json"
	"io"
)

// jsonDecode is a tiny indirection so handlers don't import encoding/json directly.
// Keeping it private keeps the import footprint in bib_handler.go small.
func jsonDecode(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}