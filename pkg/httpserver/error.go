package httpserver

// Redacted marks an error whose detail must never reach the client, regardless of whether its
// status code is otherwise public. The ErrorHandler substitutes the generic status text but still
// logs the original.
type Redacted interface {
	Redacted()
}

// RedactedError wraps an error, so the ErrorHandler never sends its detail to the client, even for
// a status code that is otherwise public.
type RedactedError struct {
	error
}

// Error returns an empty string so the wrapped detail is never rendered to the client. The original
// remains reachable through Unwrap for logging.
func (r *RedactedError) Error() string {
	return ""
}

// Redacted marks this error as redacted, satisfying the Redacted interface.
func (r *RedactedError) Redacted() {}

// Unwrap exposes the wrapped error so it can still be logged and matched with errors.Is.
func (r *RedactedError) Unwrap() error {
	return r.error
}
