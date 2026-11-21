package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrKind classifies provider failures so the CLI can explain them and the
// fallback chain can decide what to do next.
type ErrKind int

const (
	ErrOther       ErrKind = iota
	ErrAuth                // bad or missing API key
	ErrRateLimit           // too many requests / quota used up
	ErrNoCredit            // account has no credit left
	ErrNotFound            // wrong URL or unknown model
	ErrUnreachable         // network error, server not running, timeout
	ErrServer              // 5xx from the provider
)

// Error is a provider failure with a human-friendly explanation.
type Error struct {
	Kind       ErrKind
	Status     int
	Base       string
	Detail     string
	RetryAfter time.Duration
	Err        error
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Error() string {
	var why string
	switch e.Kind {
	case ErrAuth:
		why = "the API key was rejected or is missing"
	case ErrRateLimit:
		why = "rate limit or free-tier quota reached"
		if e.RetryAfter > 0 {
			why += fmt.Sprintf(" (try again in %s)", e.RetryAfter.Round(time.Second))
		}
	case ErrNoCredit:
		why = "the account is out of credits"
	case ErrNotFound:
		why = "not found — check base_url and the model name"
	case ErrUnreachable:
		why = "could not connect"
		if isLocal(e.Base) {
			why += " — is the local model server running?"
		}
	case ErrServer:
		why = "the provider had a server error"
	default:
		why = "request failed"
	}
	s := fmt.Sprintf("%s: %s", e.Base, why)
	if e.Status != 0 {
		s += fmt.Sprintf(" [HTTP %d]", e.Status)
	}
	if d := strings.TrimSpace(e.Detail); d != "" {
		if len(d) > 300 {
			d = d[:300] + "…"
		}
		s += ": " + d
	} else if e.Err != nil {
		s += ": " + e.Err.Error()
	}
	return s
}

// KindOf returns the error kind, or ErrOther.
func KindOf(err error) ErrKind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return ErrOther
}

// ShouldFallback reports whether the next provider in a chain should be tried.
// Everything except a user cancel (Ctrl-C) falls through: a bad key on one
// provider shouldn't stop the others from working.
func ShouldFallback(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled)
}

func isLocal(base string) bool {
	return strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1") || strings.Contains(base, "[::1]")
}

func httpError(resp *http.Response, base string) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
	e := &Error{Status: resp.StatusCode, Base: base, Detail: strings.TrimSpace(string(b))}
	low := strings.ToLower(e.Detail)
	switch {
	case resp.StatusCode == 429:
		e.Kind = ErrRateLimit
	case resp.StatusCode == 402:
		e.Kind = ErrNoCredit
	case resp.StatusCode == 401:
		e.Kind = ErrAuth
	case resp.StatusCode == 403:
		if strings.Contains(low, "quota") || strings.Contains(low, "limit") {
			e.Kind = ErrRateLimit
		} else {
			e.Kind = ErrAuth
		}
	case resp.StatusCode == 400 && (strings.Contains(low, "api key") || strings.Contains(low, "api_key")):
		e.Kind = ErrAuth // Gemini returns 400 for an invalid key
	case resp.StatusCode == 404:
		e.Kind = ErrNotFound
	case resp.StatusCode >= 500:
		e.Kind = ErrServer
	}
	if e.Kind == ErrOther && (strings.Contains(low, "quota") || strings.Contains(low, "rate limit")) {
		e.Kind = ErrRateLimit
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.ParseFloat(ra, 64); err == nil {
			e.RetryAfter = time.Duration(secs * float64(time.Second))
		}
	}
	// Groq and others put the exact wait in the message, e.g.
	// "Please try again in 7.5s" or "... in 12m30.2s"; it's more precise than
	// the rounded header.
	if m := tryAgainRe.FindStringSubmatch(e.Detail); m != nil {
		if d, err := time.ParseDuration(m[1]); err == nil && d > 0 {
			e.RetryAfter = d
		}
	}
	return e
}

var tryAgainRe = regexp.MustCompile(`(?i)try again in ((?:\d+(?:\.\d+)?(?:h|ms|m|s))+)`)

func connError(err error, base string) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	return &Error{Kind: ErrUnreachable, Base: base, Err: err}
}
