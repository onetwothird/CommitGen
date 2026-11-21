package cache

import (
	"testing"
	"time"
)

func TestPutGet(t *testing.T) {
	c := &Cache{Dir: t.TempDir(), TTL: time.Hour}
	k := Key("system", "user")
	if _, ok := c.Get(k); ok {
		t.Fatal("empty cache should miss")
	}
	if err := c.Put(k, "feat: add cache"); err != nil {
		t.Fatal(err)
	}
	if got, ok := c.Get(k); !ok || got != "feat: add cache" {
		t.Fatalf("got %q, %v", got, ok)
	}
	if err := c.Put(k, "fix: replace entry"); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Get(k); got != "fix: replace entry" {
		t.Errorf("overwrite failed: %q", got)
	}
}

func TestKeyDependsOnEveryPart(t *testing.T) {
	if Key("a", "bc") == Key("ab", "c") {
		t.Error("parts must be separated")
	}
	if Key("x") != Key("x") {
		t.Error("key must be stable")
	}
}

func TestExpiry(t *testing.T) {
	now := time.Now()
	c := &Cache{Dir: t.TempDir(), TTL: time.Hour, Now: func() time.Time { return now }}
	c.Put("k", "feat: old")
	now = now.Add(2 * time.Hour)
	if _, ok := c.Get("k"); ok {
		t.Error("expired entry should miss")
	}
	c.Put("other", "feat: new") // prunes the expired one
	if n, _ := c.Clear(); n != 1 {
		t.Errorf("expected 1 entry left after prune, cleared %d", n)
	}
}

func TestNilCache(t *testing.T) {
	var c *Cache
	if _, ok := c.Get("k"); ok {
		t.Error("nil cache must miss")
	}
	if err := c.Put("k", "v"); err != nil {
		t.Error(err)
	}
}
