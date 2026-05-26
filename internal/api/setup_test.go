package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	body := `{"tenant_name": "Acme Corp", "admin_email": "admin@acme.com", "admin_password": "securepass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp setupResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.TenantID)
	assert.NotEmpty(t, resp.UserID)
	assert.NotEmpty(t, resp.AccessToken)

	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie)
	assert.True(t, refreshCookie.HttpOnly)
	assert.Equal(t, "/api/v1/auth", refreshCookie.Path)
}

func TestSetup_AlreadyInitialized(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	body := `{"tenant_name": "First Corp", "admin_email": "first@acme.com", "admin_password": "password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	body2 := `{"tenant_name": "Second Corp", "admin_email": "second@acme.com", "admin_password": "password456"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusConflict, rec2.Code)

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	err := json.NewDecoder(rec2.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "ALREADY_INITIALIZED", errResp.Error.Code)
}

func TestSetup_ValidationErrors(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "missing tenant_name",
			body:     `{"admin_email": "admin@acme.com", "admin_password": "securepass123"}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "invalid email",
			body:     `{"tenant_name": "Acme", "admin_email": "not-an-email", "admin_password": "securepass123"}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "short password",
			body:     `{"tenant_name": "Acme", "admin_email": "admin@acme.com", "admin_password": "short"}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "empty JSON",
			body:     `{}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "password too long",
			body:     `{"tenant_name": "Acme", "admin_email": "admin@acme.com", "admin_password": "` + strings.Repeat("a", 73) + `"}`,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			testServer.Routes().ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)

			var errResp struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			err := json.NewDecoder(rec.Body).Decode(&errResp)
			require.NoError(t, err)
			assert.Equal(t, "VALIDATION_ERROR", errResp.Error.Code)
		})
	}
}

func TestSetup_WrongContentType(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	body := `{"tenant_name": "Acme", "admin_email": "admin@acme.com", "admin_password": "securepass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "UNSUPPORTED_MEDIA_TYPE", errResp.Error.Code)
}

func TestSetup_GET_returnsMethodNotAllowed(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestSetup_largeBody(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	largePayload := strings.Repeat("a", 2*1024*1024)
	body := `{"tenant_name": "` + largePayload + `", "admin_email": "admin@acme.com", "admin_password": "securepass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}
