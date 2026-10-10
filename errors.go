package miosa

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ─── Error types ─────────────────────────────────────────────────────────────

// MiosaError is the base error type for all API errors returned by this SDK.
type MiosaError struct {
	// StatusCode is the HTTP status code, or 0 for transport-level errors.
	StatusCode int
	// Message is a human-readable description of the error.
	Message string
	// RequestID is the X-Request-ID header from the API response, if present.
	RequestID string
	// Body is the raw response body for further inspection.
	Body []byte
	// Code is the stable machine-readable error code from the
	// {"error": {"code": ...}} envelope (for example "SANDBOX_STARTING"),
	// or "" when the server sent none.
	Code string
	// Details is the decoded "details" member of the error envelope, or nil.
	// It is an object for most codes and a list for validation errors.
	Details interface{}
	// Retryable is true when the server marked the failure as safe to retry
	// ("retryable": true in the envelope).
	Retryable bool
	// RetryDelay is the server's retry hint, taken from the Retry-After
	// header or the envelope's retry_after_ms / retry_after. Zero when the
	// server gave none.
	RetryDelay time.Duration
}

// base lets every typed error expose its embedded MiosaError.
func (e *MiosaError) base() *MiosaError { return e }

// ShouldRetry reports whether the server marked this failure as retryable or
// gave a retry hint.
func (e *MiosaError) ShouldRetry() bool { return e.Retryable || e.RetryDelay > 0 }

func (e *MiosaError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("miosa: %s (status=%d, request_id=%s)", e.Message, e.StatusCode, e.RequestID)
	}
	return fmt.Sprintf("miosa: %s (status=%d)", e.Message, e.StatusCode)
}

// AuthenticationError is returned for 401 responses.
type AuthenticationError struct{ MiosaError }

// PermissionError is returned for 403 responses.
type PermissionError struct{ MiosaError }

// NotFoundError is returned for 404 responses.
type NotFoundError struct{ MiosaError }

// ValidationError is returned for 422 responses.
type ValidationError struct{ MiosaError }

// InsufficientCreditsError is returned for 402 responses.
type InsufficientCreditsError struct{ MiosaError }

// RateLimitError is returned for 429 responses.
// RetryAfter holds the server-suggested delay in seconds (0 if not provided).
type RateLimitError struct {
	MiosaError
	RetryAfter float64
}

// ServerError is returned for 5xx responses.
type ServerError struct{ MiosaError }

// ConnectionError is returned when the SDK cannot reach the API.
type ConnectionError struct {
	Cause error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("miosa: connection error: %v", e.Cause)
}

func (e *ConnectionError) Unwrap() error { return e.Cause }

// ─── Sprint 2026-10 typed errors ──────────────────────────────────────────────
//
// Each embeds MiosaError, so Code, Details, Retryable and RetryDelay are
// available on them. Use AsMiosaError to read those fields from any error this
// SDK returns.

// Error codes the API documents. The list is not exhaustive: Code is whatever
// the server sent.
const (
	CodeSandboxStarting           = "SANDBOX_STARTING"
	CodeComputerStarting          = "COMPUTER_STARTING"
	CodeSnapshotInUse             = "SNAPSHOT_IN_USE"
	CodeEnvironmentMemoryWithheld = "ENVIRONMENT_MEMORY_WITHHELD"
	CodeEnvironmentScrubFailed    = "ENVIRONMENT_SCRUB_FAILED"
	CodeEnvironmentNotFound       = "ENVIRONMENT_NOT_FOUND"
	CodeEnvironmentNameTaken      = "ENVIRONMENT_NAME_TAKEN"
	CodeMachineKeyScope           = "MACHINE_KEY_SCOPE"
	CodeBillToLocked              = "BILL_TO_LOCKED"
	CodeBillToForbidden           = "BILL_TO_FORBIDDEN"
	CodeBillToSuspended           = "BILL_TO_SUSPENDED"
	CodeTeamMemberCapReached      = "TEAM_MEMBER_CAP_REACHED"
	CodeMemberLimitReached        = "MEMBER_LIMIT_REACHED"
	CodeCreditRequired            = "CREDIT_REQUIRED"
	CodeNoAgentRun                = "NO_AGENT_RUN"
	CodeOwnCredentialsRequired    = "OWN_CREDENTIALS_REQUIRED"
	CodeInvalidSetupFile          = "INVALID_SETUP_FILE"
	CodeTimeoutTooLong            = "TIMEOUT_TOO_LONG"
)

// ResourceStartingError is the retryable refusal a sandbox or computer gives
// while it is still provisioning, resuming, or holding on a blocking
// environment step (409 SANDBOX_STARTING or COMPUTER_STARTING). Nothing ran;
// retry after RetryDelay.
type ResourceStartingError struct{ MiosaError }

// SnapshotInUseError is returned when a snapshot cannot be deleted because
// something still depends on it (409 SNAPSHOT_IN_USE).
type SnapshotInUseError struct {
	MiosaError
	// Dependents lists every dependent that blocks the delete.
	Dependents []SnapshotDependent
}

// EnvironmentMemoryWithheldError is returned when a sandbox copy would need to
// restore memory the target environment must not receive (409
// ENVIRONMENT_MEMORY_WITHHELD). It is not retryable as is.
type EnvironmentMemoryWithheldError struct {
	MiosaError
	// Reasons explains why memory is withheld (details.reasons).
	Reasons []string
}

// EnvironmentScrubFailedError is returned when a snapshot or fork was refused
// because the guest's delivered secrets could not be removed first (409
// ENVIRONMENT_SCRUB_FAILED). Retryable; nothing was saved.
type EnvironmentScrubFailedError struct{ MiosaError }

// AsMiosaError returns the MiosaError carried by any error this SDK returns
// (including the typed ones, and errors wrapped with %w), or nil.
func AsMiosaError(err error) *MiosaError {
	var b interface{ base() *MiosaError }
	if errors.As(err, &b) {
		return b.base()
	}
	return nil
}

// ErrorCode returns the API error code carried by err, or "".
func ErrorCode(err error) string {
	if m := AsMiosaError(err); m != nil {
		return m.Code
	}
	return ""
}

// IsCode reports whether err carries the given API error code.
func IsCode(err error, code string) bool {
	return code != "" && ErrorCode(err) == code
}

// IsStarting reports whether err is the retryable "machine is still starting"
// refusal (SANDBOX_STARTING or COMPUTER_STARTING).
func IsStarting(err error) bool {
	var s *ResourceStartingError
	return errors.As(err, &s)
}

// RetryAfter returns the server's retry hint for err and whether the server
// gave one.
func RetryAfter(err error) (time.Duration, bool) {
	m := AsMiosaError(err)
	if m == nil || m.RetryDelay <= 0 {
		return 0, false
	}
	return m.RetryDelay, true
}

// ─── Error construction ───────────────────────────────────────────────────────

// errorFromResponse builds the appropriate typed error from an HTTP response.
// It reads and closes the body.
func errorFromResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB cap
	resp.Body.Close()

	requestID := resp.Header.Get("X-Request-ID")
	env := parseErrorEnvelope(body)
	message := extractMessage(body, resp.StatusCode)

	retryDelay := retryAfterHeader(resp.Header.Get("Retry-After"))
	if retryDelay == 0 {
		retryDelay = env.retryDelay
	}

	base := MiosaError{
		StatusCode: resp.StatusCode,
		Message:    message,
		RequestID:  requestID,
		Body:       body,
		Code:       env.code,
		Details:    env.details,
		Retryable:  env.retryable,
		RetryDelay: retryDelay,
	}

	switch base.Code {
	case CodeSandboxStarting, CodeComputerStarting:
		base.Retryable = true
		return &ResourceStartingError{base}
	case CodeSnapshotInUse:
		return &SnapshotInUseError{MiosaError: base, Dependents: dependentsFromDetails(env.details)}
	case CodeEnvironmentMemoryWithheld:
		return &EnvironmentMemoryWithheldError{MiosaError: base, Reasons: reasonsFromDetails(env.details)}
	case CodeEnvironmentScrubFailed:
		base.Retryable = true
		return &EnvironmentScrubFailedError{base}
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &AuthenticationError{base}
	case http.StatusPaymentRequired:
		return &InsufficientCreditsError{base}
	case http.StatusForbidden:
		return &PermissionError{base}
	case http.StatusNotFound:
		return &NotFoundError{base}
	case http.StatusUnprocessableEntity:
		return &ValidationError{base}
	case http.StatusTooManyRequests:
		ra := parseRetryAfter(body)
		if ra == 0 && base.RetryDelay > 0 {
			ra = base.RetryDelay.Seconds()
		}
		return &RateLimitError{MiosaError: base, RetryAfter: ra}
	default:
		if resp.StatusCode >= 500 {
			return &ServerError{base}
		}
		return &base
	}
}

// errorEnvelope is what parseErrorEnvelope reads out of an error body. The API
// uses {"error": {"code", "message", "details", "retryable", "retry_after_ms"}};
// older routes use {"error": "text", "code": "X"}.
type errorEnvelope struct {
	code       string
	details    interface{}
	retryable  bool
	retryDelay time.Duration
}

func parseErrorEnvelope(body []byte) errorEnvelope {
	var out errorEnvelope
	var top map[string]json.RawMessage
	if json.Unmarshal(body, &top) != nil {
		return out
	}
	str := func(raw json.RawMessage) string {
		var v string
		_ = json.Unmarshal(raw, &v)
		return v
	}
	if raw, ok := top["code"]; ok {
		out.code = str(raw)
	}
	fill := func(m map[string]json.RawMessage) {
		if raw, ok := m["code"]; ok && str(raw) != "" {
			out.code = str(raw)
		}
		if raw, ok := m["details"]; ok {
			var d interface{}
			if json.Unmarshal(raw, &d) == nil {
				out.details = d
			}
		}
		if raw, ok := m["retryable"]; ok {
			_ = json.Unmarshal(raw, &out.retryable)
		}
		if raw, ok := m["retry_after_ms"]; ok {
			var ms float64
			if json.Unmarshal(raw, &ms) == nil && ms > 0 {
				out.retryDelay = time.Duration(ms * float64(time.Millisecond))
			}
		}
		if out.retryDelay == 0 {
			if raw, ok := m["retry_after"]; ok {
				var sec float64
				if json.Unmarshal(raw, &sec) == nil && sec > 0 {
					out.retryDelay = time.Duration(sec * float64(time.Second))
				}
			}
		}
	}
	fill(top)
	if raw, ok := top["error"]; ok {
		var nested map[string]json.RawMessage
		if json.Unmarshal(raw, &nested) == nil {
			fill(nested)
		}
	}
	return out
}

// retryAfterHeader parses a Retry-After header: delta-seconds or an HTTP date.
func retryAfterHeader(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if sec, err := strconv.ParseFloat(v, 64); err == nil {
		if sec <= 0 {
			return 0
		}
		return time.Duration(sec * float64(time.Second))
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func dependentsFromDetails(details interface{}) []SnapshotDependent {
	m, ok := details.(map[string]interface{})
	if !ok {
		return nil
	}
	raw, err := json.Marshal(m["dependents"])
	if err != nil {
		return nil
	}
	var out []SnapshotDependent
	_ = json.Unmarshal(raw, &out)
	return out
}

func reasonsFromDetails(details interface{}) []string {
	m, ok := details.(map[string]interface{})
	if !ok {
		return nil
	}
	list, _ := m["reasons"].([]interface{})
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// extractMessage tries to pull a human-readable message from a JSON body.
func extractMessage(body []byte, statusCode int) string {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return fmt.Sprintf("request failed with status %d", statusCode)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err == nil {
		for _, key := range []string{"message", "error", "detail"} {
			if raw, ok := obj[key]; ok {
				var s string
				if err := json.Unmarshal(raw, &s); err == nil && s != "" {
					return s
				}
			}
		}
		// Nested envelope: {"error": {"code": "...", "message": "..."}}.
		if raw, ok := obj["error"]; ok {
			var nested struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(raw, &nested) == nil && nested.Message != "" {
				return nested.Message
			}
		}
		// "errors" key may be a list
		if raw, ok := obj["errors"]; ok {
			var list []string
			if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
				return list[0]
			}
		}
	}
	// A JSON object with no readable message: do not surface the raw object.
	if body[0] == '{' {
		return fmt.Sprintf("request failed with status %d", statusCode)
	}
	// Fall back to the raw body if it's a plain string.
	if s := strings.Trim(string(body), `"`); s != "" {
		return s
	}
	return fmt.Sprintf("request failed with status %d", statusCode)
}

// parseRetryAfter extracts retry_after from a JSON body, returning 0 if absent.
func parseRetryAfter(body []byte) float64 {
	var obj struct {
		RetryAfter float64 `json:"retry_after"`
	}
	_ = json.Unmarshal(body, &obj)
	return obj.RetryAfter
}

// isRetryable reports whether the error is of a kind that may be retried
// (a rate limit or a server error). shouldRetry applies the method rules.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	switch err.(type) {
	case *RateLimitError, *ServerError:
		return true
	}
	return false
}

// retrySafe reports whether repeating a request cannot do its work twice:
// idempotent methods, or any request carrying an Idempotency-Key.
func retrySafe(method string, headers map[string]string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	}
	return headers["Idempotency-Key"] != ""
}

// shouldRetry decides whether the client repeats a request that failed with
// err:
//   - 429 is always safe to repeat: the request was refused before it ran.
//   - The server marked it retryable (retryable: true): its word that nothing
//     was done.
//   - Other 5xx responses are repeated only when the request is idempotent or
//     carries an Idempotency-Key, so a create is never silently run twice.
func shouldRetry(method string, headers map[string]string, err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*RateLimitError); ok {
		return true
	}
	if m := AsMiosaError(err); m != nil && m.Retryable {
		return true
	}
	if isRetryable(err) {
		return retrySafe(method, headers)
	}
	return false
}
