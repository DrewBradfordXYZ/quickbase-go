package core

import (
	"fmt"
	"log"
	"time"
)

// Logger provides debug logging for the QuickBase SDK.
type Logger struct {
	enabled bool
	prefix  string
}

// NewLogger creates a new logger.
func NewLogger(enabled bool) *Logger {
	return &Logger{
		enabled: enabled,
		prefix:  "quickbase-go",
	}
}

func (l *Logger) formatMessage(level, message string) string {
	return fmt.Sprintf("[%s] [%s] [%s] %s",
		time.Now().Format(time.RFC3339),
		l.prefix,
		level,
		message,
	)
}

// Debug logs a debug message (only if debug is enabled).
func (l *Logger) Debug(message string, args ...any) {
	if l.enabled {
		if len(args) > 0 {
			message = fmt.Sprintf(message, args...)
		}
		log.Println(l.formatMessage("DEBUG", message))
	}
}

// Info logs an info message (only if debug is enabled).
func (l *Logger) Info(message string, args ...any) {
	if l.enabled {
		if len(args) > 0 {
			message = fmt.Sprintf(message, args...)
		}
		log.Println(l.formatMessage("INFO", message))
	}
}

// Warn logs a warning message (always logged).
func (l *Logger) Warn(message string, args ...any) {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	log.Println(l.formatMessage("WARN", message))
}

// Error logs an error message (always logged).
func (l *Logger) Error(message string, args ...any) {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	log.Println(l.formatMessage("ERROR", message))
}

// RateLimit logs rate limit information.
func (l *Logger) RateLimit(info RateLimitInfo) {
	if l.enabled {
		rayID := info.QBAPIRay
		if rayID == "" {
			rayID = info.CFRay
		}
		msg := fmt.Sprintf("Rate limited (attempt %d): %s - Status %d, Retry-After: %ds",
			info.Attempt, info.RequestURL, info.HTTPStatus, info.RetryAfter)
		if rayID != "" {
			msg += fmt.Sprintf(", Ray: %s", rayID)
		}
		l.Debug(msg)
	}
}

// Timing logs request timing information.
func (l *Logger) Timing(method, url string, duration time.Duration) {
	if l.enabled {
		l.Debug("%s %s completed in %dms", method, url, duration.Milliseconds())
	}
}

// Retry logs retry attempt information.
func (l *Logger) Retry(attempt, maxAttempts int, delay time.Duration, reason string) {
	if l.enabled {
		l.Debug("Retry %d/%d in %dms: %s", attempt, maxAttempts, delay.Milliseconds(), reason)
	}
}

// Token logs token operations (without exposing the actual token).
func (l *Logger) Token(operation string, dbid string) {
	if l.enabled {
		if dbid != "" {
			l.Debug("Token %s for dbid: %s", operation, dbid)
		} else {
			l.Debug("Token %s", operation)
		}
	}
}

// Enabled returns whether debug logging is enabled.
func (l *Logger) Enabled() bool {
	return l.enabled
}
