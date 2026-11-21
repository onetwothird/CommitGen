package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/onetwothird/CommitGen/internal/heuristic"
)

type Offline struct{}

func (Offline) Generate(_ context.Context, r Request, on OnToken) (string, error) {
	var msg string
	if r.Offline != nil {
		msg = r.Offline()
	} else {
		msg = heuristic.Generate(r.Diff, heuristic.Options{
			Style: r.Style, Scopes: r.Scopes, Hint: r.Hint, MaxSubject: r.MaxHeader, NoBreaking: r.NoBreaking,
		})
	}
	if msg == "" {
		return "", errors.New("offline: the diff has no files")
	}
	if on != nil {
		on(msg)
	}
	return msg, nil
}

type Link struct {
	Name     string // display name, e.g. "groq"
	Model    string // model used with this provider
	Provider Provider
}

// Defaults for how the chain treats rate limits.
const (
	// DefaultMaxWait is the longest the chain waits for a rate-limited
	// provider when the only thing left after it is offline mode. Free tiers
	// mostly limit requests/tokens per minute, so a short wait usually beats
	// a rule-based message.
	DefaultMaxWait = 30 * time.Second
	// defaultCooldown is used when a rate limit doesn't say how long to wait.
	defaultCooldown = 60 * time.Second
	maxRetries      = 3
)

// Chain tries each link in order and moves on when one fails.
//
//   - Rate limits: if the next links are only offline mode and the provider
//     asks to wait at most MaxWait, the chain waits and retries. Otherwise it
//     moves on and puts the provider on a cooldown, so later calls (e.g.
//     `--each` over 50 files) go back to it once the limit resets.
//   - Bad key, unknown model, no credit: the provider is skipped for the rest
//     of the run.
//   - Network / server errors: a short cooldown.
type Chain struct {
	Links   []Link
	Notify  func(msg string) // called when waiting or falling back; may be nil
	MaxWait time.Duration    // 0 = DefaultMaxWait; negative = never wait

	// For tests.
	Now   func() time.Time
	Sleep func(ctx context.Context, d time.Duration) error

	mu       sync.Mutex
	until    map[int]time.Time // link index -> skip until
	disabled map[int]bool
	used     string
}

func (c *Chain) Used() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.used
}

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

func (e *ChainError) Unwrap() []error { return e.Errs }

func (c *Chain) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Chain) sleep(ctx context.Context, d time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *Chain) notify(format string, a ...any) {
	if c.Notify != nil {
		c.Notify(fmt.Sprintf(format, a...))
	}
}

func (c *Chain) maxWait() time.Duration {
	if c.MaxWait == 0 {
		return DefaultMaxWait
	}
	return c.MaxWait
}

// usable reports whether link i may be tried now.
func (c *Chain) usable(i int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.disabled[i] && !c.now().Before(c.until[i])
}

func (c *Chain) penalize(i int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.until == nil {
		c.until, c.disabled = map[int]time.Time{}, map[int]bool{}
	}
	switch KindOf(err) {
	case ErrAuth, ErrNotFound, ErrNoCredit:
		c.disabled[i] = true
	case ErrRateLimit:
		d := defaultCooldown
		var e *Error
		if errors.As(err, &e) && e.RetryAfter > 0 {
			d = e.RetryAfter
		}
		c.until[i] = c.now().Add(d)
	default:
		c.until[i] = c.now().Add(15 * time.Second)
	}
}

func (c *Chain) isDisabled(i int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disabled[i]
}

// onlyOfflineAfter reports whether every usable link after i is offline mode.
func (c *Chain) onlyOfflineAfter(i int) bool {
	for j := i + 1; j < len(c.Links); j++ {
		if _, off := c.Links[j].Provider.(Offline); !off && c.usable(j) {
			return false
		}
	}
	return true
}

// next returns the name of the first usable link after i.
func (c *Chain) next(i int) string {
	for j := i + 1; j < len(c.Links); j++ {
		if c.usable(j) {
			return c.Links[j].Name
		}
	}
	return ""
}

func (c *Chain) Generate(ctx context.Context, r Request, on OnToken) (string, error) {
	failed := &ChainError{}
	tried := false
	for pass := 0; pass < 2; pass++ {
		for i, l := range c.Links {
			// First pass skips cooling-down links; if that left nothing to
			// try, the second pass tries them anyway.
			if pass == 0 && !c.usable(i) || pass == 1 && (tried || c.isDisabled(i)) {
				continue
			}
			tried = true
			out, err := c.try(ctx, i, l, r, on)
			if err == nil {
				return out, nil
			}
			if !ShouldFallback(err) {
				return "", err
			}
			failed.Names = append(failed.Names, l.Name)
			failed.Errs = append(failed.Errs, err)
			c.penalize(i, err)
			if n := c.next(i); n != "" {
				c.notify("%s failed (%s), trying %s", l.Name, shortReason(err), n)
			}
		}
	}
	if len(failed.Errs) == 0 {
		return "", errors.New("no provider is available")
	}
	return "", failed
}

// try calls one link, waiting out short rate limits when the alternative is
// only offline mode.
func (c *Chain) try(ctx context.Context, i int, l Link, r Request, on OnToken) (string, error) {
	req := r
	if l.Model != "" {
		req.Model = l.Model
	}
	for attempt := 0; ; attempt++ {
		out, err := l.Provider.Generate(ctx, req, on)
		if err == nil && strings.TrimSpace(out) == "" {
			err = errors.New("empty reply")
		}
		if err == nil {
			c.mu.Lock()
			c.used = l.Name
			c.mu.Unlock()
			return out, nil
		}
		var e *Error
		if attempt >= maxRetries || KindOf(err) != ErrRateLimit || !errors.As(err, &e) ||
			e.RetryAfter <= 0 || e.RetryAfter > c.maxWait() || !c.onlyOfflineAfter(i) {
			return "", err
		}
		wait := e.RetryAfter + 250*time.Millisecond
		c.notify("%s rate limit reached, waiting %s and retrying", l.Name, wait.Round(100*time.Millisecond))
		if err := c.sleep(ctx, wait); err != nil {
			return "", err
		}
	}
}

func shortReason(err error) string {
	switch KindOf(err) {
	case ErrAuth:
		return "API key rejected or missing"
	case ErrRateLimit:
		var e *Error
		if errors.As(err, &e) && e.RetryAfter > 0 {
			return "rate limit / quota reached, resets in " + e.RetryAfter.Round(time.Second).String()
		}
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
