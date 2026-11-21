package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/onetwothird/CommitGen/internal/heuristic"
)

// Offline writes the message from the diff with rules instead of a model.
// It never fails as long as something is staged.
type Offline struct{}

func (Offline) Generate(_ context.Context, r Request, on OnToken) (string, error) {
	msg := heuristic.Generate(r.Diff, heuristic.Options{Style: r.Style, Scopes: r.Scopes, Hint: r.Hint})
	if msg == "" {
		return "", errors.New("offline: the diff has no files")
	}
	if on != nil {
		on(msg)
	}
	return msg, nil
}

// Link is one provider in a fallback chain.
type Link struct {
	Name     string // display name, e.g. "groq"
	Model    string // model used with this provider
	Provider Provider
}

// Chain tries each link in order and moves on when one fails (rate limit,
// quota, bad key, server down...). Once a link works, later calls in the same
// run start from it so retries and extra candidates don't hit the failed
// provider again.
type Chain struct {
	Links  []Link
	Notify func(msg string) // called when falling back; may be nil

	mu    sync.Mutex
	start int
	used  string
}

// Used returns the name of the last provider that produced a message.
func (c *Chain) Used() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.used
}

// ChainError lists why every provider failed.
type ChainError struct {
	Names []string
	Errs  []error
}

func (e *ChainError) Error() string {
	if len(e.Errs) == 1 {
		return e.Names[0] + ": " + e.Errs[0].Error()
	}
	var b strings.Builder
	b.WriteString("every provider failed:")
	for i, err := range e.Errs {
		fmt.Fprintf(&b, "\n  - %s: %v", e.Names[i], err)
	}
	return b.String()
}

// Unwrap exposes the individual errors to errors.Is/As.
func (e *ChainError) Unwrap() []error { return e.Errs }

func (c *Chain) Generate(ctx context.Context, r Request, on OnToken) (string, error) {
	c.mu.Lock()
	start := c.start
	c.mu.Unlock()

	failed := &ChainError{}
	for i := start; i < len(c.Links); i++ {
		l := c.Links[i]
		req := r
		if l.Model != "" {
			req.Model = l.Model
		}
		out, err := l.Provider.Generate(ctx, req, on)
		if err == nil && strings.TrimSpace(out) != "" {
			c.mu.Lock()
			if i > c.start {
				c.start = i
			}
			c.used = l.Name
			c.mu.Unlock()
			return out, nil
		}
		if err == nil {
			err = errors.New("empty reply")
		}
		if !ShouldFallback(err) {
			return "", err
		}
		failed.Names = append(failed.Names, l.Name)
		failed.Errs = append(failed.Errs, err)
		if i+1 < len(c.Links) && c.Notify != nil {
			c.Notify(fmt.Sprintf("%s failed (%s), trying %s", l.Name, shortReason(err), c.Links[i+1].Name))
		}
	}
	return "", failed
}

func shortReason(err error) string {
	switch KindOf(err) {
	case ErrAuth:
		return "API key rejected or missing"
	case ErrRateLimit:
		return "rate limit / quota reached"
	case ErrNoCredit:
		return "out of credits"
	case ErrNotFound:
		return "model or URL not found"
	case ErrUnreachable:
		return "could not connect"
	case ErrServer:
		return "server error"
	}
	s := err.Error()
	if len(s) > 80 {
		s = s[:80] + "…"
	}
	return s
}
