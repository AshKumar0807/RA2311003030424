// Package logging provides a reusable logging middleware that ships log events
// to the centralised evaluation-service log ingestion API.
//
// Usage:
//
//	import "github.com/AshKumar0807/RA2311003030424/logging_middleware"
//
//	logging.SetAPIKey("your-key")
//	logging.Log("backend", "info", "handler", "Request received for /health")
package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ── Constants ────────────────────────────────────────────────────────────────

const defaultLogEndpoint = "http://20.207.122.201/evaluation-service/logs"

var validStacks = map[string]bool{
	"backend":  true,
	"frontend": true,
}

var validLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
	"fatal": true,
}

var validPackages = map[string]bool{
	// Shared
	"cache":      true,
	"controller": true,
	"cron_job":   true,
	"db":         true,
	"domain":     true,
	"handler":    true,
	"repository": true,
	"route":      true,
	"service":    true,
	// Backend-only
	"auth":       true,
	"config":     true,
	"middleware": true,
	"utils":      true,
}

// ── Types ────────────────────────────────────────────────────────────────────

// LogRequest is the JSON body sent to the log ingestion API.
type LogRequest struct {
	Stack   string `json:"stack"`
	Level   string `json:"level"`
	Package string `json:"package"`
	Message string `json:"message"`
}

// LogResponse is the JSON body returned by the log ingestion API on success.
type LogResponse struct {
	LogID   string `json:"logId"`
	Message string `json:"message"`
}

// Logger holds configuration for the logging client.
type Logger struct {
	HTTPClient  *http.Client
	APIKey      string
	endpointURL string // overridable in tests; defaults to defaultLogEndpoint
}

// NewLogger creates a Logger with a custom API key and default HTTP timeout.
func NewLogger(apiKey string) *Logger {
	return &Logger{
		APIKey:      apiKey,
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
		endpointURL: defaultLogEndpoint,
	}
}

// ── Package-level default logger ─────────────────────────────────────────────

var defaultLogger = &Logger{
	HTTPClient:  &http.Client{Timeout: 10 * time.Second},
	endpointURL: defaultLogEndpoint,
}

// SetAPIKey configures the API key used by the package-level Log function.
func SetAPIKey(key string) {
	defaultLogger.APIKey = key
}

// ── Validation ───────────────────────────────────────────────────────────────

func validate(stack, level, pkg string) error {
	if !validStacks[stack] {
		return fmt.Errorf("invalid stack %q: allowed values are backend, frontend", stack)
	}
	if !validLevels[level] {
		return fmt.Errorf("invalid level %q: allowed values are debug, info, warn, error, fatal", level)
	}
	if !validPackages[pkg] {
		return fmt.Errorf("invalid package %q: see validPackages for allowed values", pkg)
	}
	return nil
}

// ── Core Log function (package-level convenience wrapper) ─────────────────────

// Log ships a structured log event to the central evaluation-service API.
//
//	stack   – "backend" | "frontend"
//	level   – "debug" | "info" | "warn" | "error" | "fatal"
//	pkg     – "handler" | "db" | "service" | "middleware" | … (see validPackages)
//	message – human-readable description of the event
//
// Fields are normalised to lower-case before sending.
// Returns the logId assigned by the server, or an error.
func Log(stack, level, pkg, message string) (string, error) {
	return defaultLogger.Log(stack, level, pkg, message)
}

// Log is the Logger method version — useful for injecting a custom logger.
func (l *Logger) Log(stack, level, pkg, message string) (string, error) {
	endpoint := l.endpointURL
	if endpoint == "" {
		endpoint = defaultLogEndpoint
	}
	return l.logTo(endpoint, stack, level, pkg, message)
}

// logTo is the internal implementation; endpoint is injectable for tests.
func (l *Logger) logTo(endpoint, stack, level, pkg, message string) (string, error) {
	// Normalise to lower-case
	stack = strings.ToLower(strings.TrimSpace(stack))
	level = strings.ToLower(strings.TrimSpace(level))
	pkg = strings.ToLower(strings.TrimSpace(pkg))

	// Validate fields before hitting the network
	if err := validate(stack, level, pkg); err != nil {
		return "", fmt.Errorf("logging.Log validation: %w", err)
	}
	if strings.TrimSpace(message) == "" {
		return "", errors.New("logging.Log: message must not be empty")
	}

	// Serialise request body
	payload := LogRequest{
		Stack:   stack,
		Level:   level,
		Package: pkg,
		Message: message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("logging.Log marshal: %w", err)
	}

	// Build HTTP request
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("logging.Log build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if l.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.APIKey)
	}

	// Execute
	resp, err := l.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("logging.Log http: %w", err)
	}
	defer resp.Body.Close()

	// Decode response
	var logResp LogResponse
	if err := json.NewDecoder(resp.Body).Decode(&logResp); err != nil {
		return "", fmt.Errorf("logging.Log decode response (status %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("logging.Log server error (status %d): %s", resp.StatusCode, logResp.Message)
	}

	return logResp.LogID, nil
}

// Convenience helpers — call Log with "backend" stack pre-filled.
func (l *Logger) Debug(pkg, message string) (string, error) {
	return l.Log("backend", "debug", pkg, message)
}
func (l *Logger) Info(pkg, message string) (string, error) {
	return l.Log("backend", "info", pkg, message)
}
func (l *Logger) Warn(pkg, message string) (string, error) {
	return l.Log("backend", "warn", pkg, message)
}
func (l *Logger) Error(pkg, message string) (string, error) {
	return l.Log("backend", "error", pkg, message)
}
func (l *Logger) Fatal(pkg, message string) (string, error) {
	return l.Log("backend", "fatal", pkg, message)
}

// New is an alias for NewLogger for convenient instantiation.
func New(apiKey string) *Logger { return NewLogger(apiKey) }
