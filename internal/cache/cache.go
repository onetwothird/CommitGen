// Package cache remembers generated text by a hash of its inputs, so asking
// again for the same staged diff (a second run, the git hook after a preview,
// a commit you aborted) costs no API call and no rate limit.
//
// Only the generated messages are stored, never the diff.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultTTL is how long an entry stays valid.
const DefaultTTL = 7 * 24 * time.Hour

type Cache struct {
	Dir string
	TTL time.Duration
	Now func() time.Time // for tests
}

// Default returns a cache in the user's cache directory, or nil when there
// isn't one. A nil *Cache is valid and never hits.
func Default() *Cache {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return nil
	}
	return &Cache{Dir: filepath.Join(dir, "CommitGen", "messages"), TTL: DefaultTTL}
}

// Key hashes the parts that decide the output (prompt, provider chain, ...).
func Key(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (c *Cache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Cache) path(key string) string { return filepath.Join(c.Dir, key+".txt") }

// Get returns the stored text for key if it exists and hasn't expired.
func (c *Cache) Get(key string) (string, bool) {
	if c == nil || key == "" {
		return "", false
	}
	p := c.path(key)
	st, err := os.Stat(p)
	if err != nil || c.now().Sub(st.ModTime()) > c.TTL {
		return "", false
	}
	b, err := os.ReadFile(p)
	if err != nil || len(strings.TrimSpace(string(b))) == 0 {
		return "", false
	}
	return string(b), true
}

// Put stores text under key and drops expired entries. Errors are returned
// but callers can ignore them: a cache that can't be written just misses.
func (c *Cache) Put(key, text string) error {
	if c == nil || key == "" || strings.TrimSpace(text) == "" {
		return nil
	}
	if err := os.MkdirAll(c.Dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(c.Dir, "tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.WriteString(text)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		if werr != nil {
			return werr
		}
		return cerr
	}
	p := c.path(key)
	os.Remove(p) // Windows can't rename over an existing file
	if err := os.Rename(tmp.Name(), p); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if c.Now != nil {
		t := c.now()
		os.Chtimes(p, t, t)
	}
	c.prune()
	return nil
}

func (c *Cache) prune() {
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && c.now().Sub(info.ModTime()) > c.TTL {
			os.Remove(filepath.Join(c.Dir, e.Name()))
		}
	}
}

// Clear removes every entry and returns how many were removed.
func (c *Cache) Clear() (int, error) {
	if c == nil {
		return 0, nil
	}
	entries, err := os.ReadDir(c.Dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if os.Remove(filepath.Join(c.Dir, e.Name())) == nil {
			n++
		}
	}
	return n, nil
}
