package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Sentinel errors returned by DecodeJSON for common failure modes.
var (
	ErrUnsupportedContentType = errors.New("unsupported media type")
	ErrBodyTooLarge           = errors.New("request body too large")
	ErrInvalidJSON            = errors.New("invalid json")
	ErrEmptyBody              = errors.New("empty request body")
)

// DecodeJSON decodes a JSON request body into the target value.
// It returns descriptive errors for common failure modes.
func DecodeJSON(r *http.Request, dst any) error {
	if r.Body == nil || r.Body == http.NoBody {
		return ErrEmptyBody
	}

	ct := r.Header.Get("Content-Type")
	if ct != "" && !hasJSONContentType(ct) {
		return ErrUnsupportedContentType
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return ErrBodyTooLarge
		}

		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
		}

		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return fmt.Errorf("%w: field %q expects %s", ErrInvalidJSON, typeErr.Field, typeErr.Type)
		}

		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	if dec.More() {
		return fmt.Errorf("%w: multiple JSON objects", ErrInvalidJSON)
	}

	return nil
}

// DrainAndCloseBody reads and closes the request body to enable connection reuse.
func DrainAndCloseBody(r *http.Request) {
	if r.Body != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
	}
}

func hasJSONContentType(ct string) bool {
	ct = strings.TrimSpace(ct)
	ct, _, _ = strings.Cut(ct, ";")
	ct = strings.TrimSpace(ct)
	return ct == "application/json"
}

const (
	slugPattern    = `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	flagKeyPattern = `^[a-z0-9][a-z0-9_-]*[a-z0-9]$`
	timeFormat     = time.RFC3339Nano
)

var (
	slugRegex    = regexp.MustCompile(slugPattern)
	flagKeyRegex = regexp.MustCompile(flagKeyPattern)
)
