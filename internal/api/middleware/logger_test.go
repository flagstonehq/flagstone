package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func fieldsMap(fields []zapcore.Field) map[string]interface{} {
	m := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		switch f.Type {
		case zapcore.StringType:
			m[f.Key] = f.String
		case zapcore.Int64Type, zapcore.Int32Type:
			m[f.Key] = f.Integer
		case zapcore.DurationType:
			m[f.Key] = time.Duration(f.Integer)
		default:
			if f.Interface != nil {
				m[f.Key] = f.Interface
			}
		}
	}
	return m
}

func TestLogger_logsRequest(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	assert.Equal(t, "request completed", entry.Message)
	assert.Equal(t, zapcore.InfoLevel, entry.Level)

	fields := fieldsMap(entry.Context)
	assert.Equal(t, "GET", fields["method"])
	assert.True(t, strings.HasSuffix(fields["path"].(string), "/test"))
	assert.Equal(t, int64(http.StatusOK), fields["status"])
	assert.Equal(t, int64(5), fields["bytes"])
	assert.Contains(t, fields, "duration")
	assert.Contains(t, fields, "request_id")
}

func TestLogger_logsErrorStatus(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodPost, "/missing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	entry := logs.All()[0]
	fields := fieldsMap(entry.Context)
	assert.Equal(t, int64(http.StatusNotFound), fields["status"])
}

func TestLogger_passthrough(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 1, logs.Len())
}
