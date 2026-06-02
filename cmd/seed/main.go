package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBackend = "http://localhost:8080"
	adminEmail     = "admin@acme.com"
	adminPassword  = "password123"
	tenantName     = "Acme"
	tenantSlug     = "acme"
	projectSlug    = "my-app"
	projectName    = "My App"
	requestTimeout = 15 * time.Second
)

// httpClient is reused across calls to keep connections alive.
var httpClient = &http.Client{Timeout: requestTimeout}

type setupStatus struct {
	Initialized bool `json:"initialized"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type environment struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
}

type apiKey struct {
	ID            string `json:"id"`
	EnvironmentID string `json:"environment_id"`
	Name          string `json:"name"`
	KeyPrefix     string `json:"key_prefix"`
	Key           string `json:"key"`
}

type setupResponse struct {
	TenantID    string `json:"tenant_id"`
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
}

func main() {
	backend := os.Getenv("BACKEND_URL")
	if backend == "" {
		backend = defaultBackend
	}

	if err := run(backend); err != nil {
		fmt.Fprintf(os.Stderr, "[seed] FAILED: %v\n", err)
		os.Exit(1)
	}
}

func run(backend string) error {
	fmt.Printf("[seed] backend: %s\n", backend)

	initialized, err := getSetupStatus(backend)
	if err != nil {
		return fmt.Errorf("setup status: %w", err)
	}
	if initialized {
		fmt.Println("[seed] already initialized, skipping seed")
		printSummary(nil)
		return nil
	}

	token, err := postSetup(backend)
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	fmt.Println("[seed] tenant + admin user created")

	if err := createProject(backend, token); err != nil && !isConflict(err) {
		return fmt.Errorf("create project: %w", err)
	}
	fmt.Printf("[seed] project '%s' ready\n", projectSlug)

	envs, err := listEnvironments(backend, token)
	if err != nil {
		return fmt.Errorf("list environments: %w", err)
	}
	if len(envs) == 0 {
		return fmt.Errorf("project has no environments (auto-create failed?)")
	}
	envSlugs := make([]string, 0, len(envs))
	for _, e := range envs {
		envSlugs = append(envSlugs, e.Slug)
	}
	fmt.Printf("[seed] environments: %s\n", strings.Join(envSlugs, ", "))

	keys := map[string]string{}
	for _, e := range envs {
		k, err := createAPIKey(backend, token, e.Slug, fmt.Sprintf("Seed key for %s", e.Slug))
		if err != nil {
			return fmt.Errorf("create api key for %s: %w", e.Slug, err)
		}
		keys[e.Slug] = k
	}
	for slug, k := range keys {
		fmt.Printf("[seed] api key (%s): %s\n", slug, k)
	}

	flags := []struct {
		key, name, typ, defVal string
	}{
		{"new-checkout", "New Checkout Flow", "boolean", "false"},
		{"dark-mode", "Dark Mode", "boolean", "false"},
		{"max-items", "Max items per cart", "number", "10"},
	}
	for _, f := range flags {
		if err := createFlag(backend, token, f.key, f.name, f.typ, f.defVal); err != nil && !isConflict(err) {
			return fmt.Errorf("create flag %s: %w", f.key, err)
		}
	}
	fmt.Println("[seed] flags: new-checkout, dark-mode, max-items")

	if err := toggleFlagEnv(backend, token, "dark-mode", "production"); err != nil && !isConflict(err) {
		return fmt.Errorf("toggle dark-mode in production: %w", err)
	}
	fmt.Println("[seed] dark-mode enabled in production")

	segments := []struct {
		key, name, rules string
	}{
		{
			key:  "beta-users",
			name: "Beta Users",
			rules: `{
				"all": [
					{"attribute": "plan", "op": "eq", "value": "beta"}
				]
			}`,
		},
		{
			key:  "premium-customers",
			name: "Premium Customers",
			rules: `{
				"all": [
					{"attribute": "plan", "op": "in", "value": ["premium", "enterprise"]}
				]
			}`,
		},
		{
			key:  "power-shoppers",
			name: "Power Shoppers",
			rules: `{
				"all": [
					{"attribute": "country", "op": "in", "value": ["AR", "BR", "MX"]},
					{"attribute": "lifetime_orders", "op": "gte", "value": 5}
				]
			}`,
		},
	}
	for _, s := range segments {
		if err := createSegment(backend, token, s.key, s.name, s.rules); err != nil && !isConflict(err) {
			return fmt.Errorf("create segment %s: %w", s.key, err)
		}
	}
	fmt.Println("[seed] segments: beta-users, premium-customers, power-shoppers")

	printSummary(keys)
	return nil
}

func printSummary(keys map[string]string) {
	fmt.Println()
	fmt.Println("=== Demo ready ===")
	fmt.Println("Dashboard:  http://localhost:3000")
	fmt.Println("API:        http://localhost:8080")
	fmt.Printf("Login:      %s / %s\n", adminEmail, adminPassword)
	fmt.Printf("Tenant:     %s\n", tenantSlug)
	fmt.Printf("Project:    %s (envs: development, staging, production)\n", projectSlug)
	if len(keys) > 0 {
		fmt.Println("API keys:")
		for _, env := range []string{"production", "staging", "development"} {
			if k, ok := keys[env]; ok {
				fmt.Printf("  %-12s %s\n", env+":", k)
			}
		}
	}
}

// --- HTTP helpers -----------------------------------------------------------

// doJSON performs an HTTP request with optional JSON body and decodes the
// response. 2xx is success. Non-2xx returns the parsed error body (or the
// raw response) so callers can inspect status codes.
//
// The url parameter is operator-controlled (BACKEND_URL env var or default
// localhost), not user input, so G304/G404 do not apply.
func doJSON(method, url string, body, out any, token string) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	// #nosec G704 -- URL is operator-controlled (BACKEND_URL), not untrusted input.
	req, err := http.NewRequestWithContext(context.Background(), method, url, reqBody)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// #nosec G704 -- see above.
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpError{
			status: resp.StatusCode,
			body:   string(respBody),
			url:    url,
		}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response (status %d): %w (body: %s)", resp.StatusCode, err, truncate(string(respBody), 200))
		}
	}
	return nil
}

type httpError struct {
	status int
	body   string
	url    string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d from %s: %s", e.status, e.url, truncate(e.body, 300))
}

func (e *httpError) StatusCode() int { return e.status }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// isConflict reports whether the error is a 409 (resource already exists).
func isConflict(err error) bool {
	var he *httpError
	if errors.As(err, &he) {
		return he.status == http.StatusConflict
	}
	return false
}

// --- Endpoint wrappers -----------------------------------------------------

func getSetupStatus(base string) (bool, error) {
	var s setupStatus
	if err := doJSON(http.MethodGet, base+"/api/v1/setup/status", nil, &s, ""); err != nil {
		return false, err
	}
	return s.Initialized, nil
}

func postSetup(base string) (string, error) {
	body := map[string]string{
		"tenant_name":    tenantName,
		"admin_email":    adminEmail,
		"admin_password": adminPassword,
	}
	var resp setupResponse
	if err := doJSON(http.MethodPost, base+"/api/v1/setup", body, &resp, ""); err != nil {
		return "", err
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("setup response missing access_token")
	}
	return resp.AccessToken, nil
}

func login(base string) (string, error) {
	body := map[string]string{
		"email":       adminEmail,
		"password":    adminPassword,
		"tenant_slug": tenantSlug,
	}
	var resp loginResponse
	if err := doJSON(http.MethodPost, base+"/api/v1/auth/login", body, &resp, ""); err != nil {
		return "", err
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("login response missing access_token")
	}
	return resp.AccessToken, nil
}

func createProject(base, token string) error {
	body := map[string]string{
		"slug": projectSlug,
		"name": projectName,
	}
	return doJSON(http.MethodPost, base+"/api/v1/projects", body, nil, token)
}

func listEnvironments(base, token string) ([]environment, error) {
	var envs []environment
	url := fmt.Sprintf("%s/api/v1/projects/%s/environments", base, projectSlug)
	if err := doJSON(http.MethodGet, url, nil, &envs, token); err != nil {
		return nil, err
	}
	return envs, nil
}

func createAPIKey(base, token, envSlug, name string) (string, error) {
	body := map[string]string{"name": name}
	url := fmt.Sprintf("%s/api/v1/projects/%s/environments/%s/apikeys", base, projectSlug, envSlug)
	var resp apiKey
	if err := doJSON(http.MethodPost, url, body, &resp, token); err != nil {
		return "", err
	}
	if resp.Key == "" {
		return "", fmt.Errorf("api key response missing 'key' field")
	}
	return resp.Key, nil
}

func createFlag(base, token, key, name, typ, defaultValue string) error {
	body := map[string]any{
		"key":           key,
		"name":          name,
		"type":          typ,
		"default_value": json.RawMessage(defaultValue),
	}
	url := fmt.Sprintf("%s/api/v1/projects/%s/flags", base, projectSlug)
	return doJSON(http.MethodPost, url, body, nil, token)
}

func toggleFlagEnv(base, token, flagKey, envSlug string) error {
	body := map[string]bool{"enabled": true}
	url := fmt.Sprintf("%s/api/v1/projects/%s/flags/%s/environments/%s", base, projectSlug, flagKey, envSlug)
	return doJSON(http.MethodPatch, url, body, nil, token)
}

func createSegment(base, token, key, name, rulesJSON string) error {
	body := map[string]any{
		"key":   key,
		"name":  name,
		"rules": json.RawMessage(rulesJSON),
	}
	url := fmt.Sprintf("%s/api/v1/projects/%s/segments", base, projectSlug)
	return doJSON(http.MethodPost, url, body, nil, token)
}
