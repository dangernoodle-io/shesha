package httpx

import (
	"fmt"
	"net/http"

	"github.com/dangernoodle-io/shesha/jsonutil"
)

// WriteJSON writes v as an application/json response with the given status
// code, encoded via jsonutil.Marshal for shesha's canonical, error-wrapped
// encoding. v is marshaled to a buffer first, before anything is written to
// w: if marshaling fails, WriteJSON returns the wrapped error and writes
// nothing at all — no status, no headers, no partial body — so a marshal
// failure never yields an incomplete or misleadingly-200 response. Only
// once encoding succeeds does WriteJSON set the Content-Type header, write
// the status, and write the body; any error from that final Write is
// returned.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	b, err := jsonutil.Marshal(v)
	if err != nil {
		return fmt.Errorf("httpx: write json: %w", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("httpx: write json: %w", err)
	}

	return nil
}
