package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJSON_success(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	body := `{"name": "Alice", "email": "alice@test.com"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst payload
	err := DecodeJSON(req, &dst)
	require.NoError(t, err)
	assert.Equal(t, "Alice", dst.Name)
	assert.Equal(t, "alice@test.com", dst.Email)
}

func TestDecodeJSON_emptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set("Content-Type", "application/json")

	var dst struct{}
	err := DecodeJSON(req, &dst)
	assert.ErrorIs(t, err, ErrEmptyBody)
}

func TestDecodeJSON_nilBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")

	var dst struct{}
	err := DecodeJSON(req, &dst)
	assert.ErrorIs(t, err, ErrEmptyBody)
}

func TestDecodeJSON_wrongContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "text/plain")

	var dst struct{}
	err := DecodeJSON(req, &dst)
	assert.ErrorIs(t, err, ErrUnsupportedContentType)
}

func TestDecodeJSON_emptyContentType(t *testing.T) {
	body := `{"name": "Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(req, &dst)
	require.NoError(t, err)
	assert.Equal(t, "Alice", dst.Name)
}

func TestDecodeJSON_invalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")

	var dst struct{}
	err := DecodeJSON(req, &dst)
	assert.True(t, errors.Is(err, ErrInvalidJSON), "expected ErrInvalidJSON, got %v", err)
}

func TestDecodeJSON_unknownFields(t *testing.T) {
	body := `{"name": "Alice", "extra": "forbidden"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(req, &dst)
	assert.True(t, errors.Is(err, ErrInvalidJSON), "expected ErrInvalidJSON, got %v", err)
}

func TestDecodeJSON_typeMismatch(t *testing.T) {
	body := `{"name": 42}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(req, &dst)
	assert.True(t, errors.Is(err, ErrInvalidJSON), "expected ErrInvalidJSON, got %v", err)
}

func TestDecodeJSON_multipleObjects(t *testing.T) {
	body := `{"a": 1}{"b": 2}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst struct{}
	err := DecodeJSON(req, &dst)
	assert.True(t, errors.Is(err, ErrInvalidJSON), "expected ErrInvalidJSON, got %v", err)
}

func TestDecodeJSON_GET_noContentType(t *testing.T) {
	body := `{"name": "Alice"}`
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(body))

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(req, &dst)
	require.NoError(t, err)
	assert.Equal(t, "Alice", dst.Name)
}

func TestDrainAndCloseBody_closesBody(t *testing.T) {
	body := strings.NewReader("hello")
	req := httptest.NewRequest(http.MethodPost, "/", body)

	DrainAndCloseBody(req)

	n, _ := io.ReadAll(req.Body)
	assert.Empty(t, n)
}

func TestDrainAndCloseBody_nilBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	DrainAndCloseBody(req)

	assert.NotNil(t, req.Body)
}

func TestDrainAndCloseBody_noBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	DrainAndCloseBody(req)

	assert.NotNil(t, req.Body)
}
