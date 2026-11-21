package redact

import (
	"strings"
	"testing"
)

func TestScan(t *testing.T) {
	// Keys are assembled at run time so this file never contains a real-looking key.
	gh := fake("gh", "p_", strings.Repeat("a1B2", 9))
	diff := strings.Join([]string{
		"diff --git a/app/config.go b/app/config.go",
		"--- a/app/config.go",
		"+++ b/app/config.go",
		"@@ -10,3 +10,4 @@ func load() {",
		" \tx := 1",
		"-\told := \"" + gh + "\"",
		"+\ttoken := \"" + gh + "\"",
		"+\tdbPassword := \"${DB_PASSWORD}\"",
		"+\tpassword = \"hunter2hunter2\"",
	}, "\n")
	got := Scan(diff)
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %+v", got)
	}
	if got[0].File != "app/config.go" || got[0].Line != 11 || got[0].Kind != "GitHub token" || !got[0].Strong {
		t.Errorf("first finding wrong: %+v", got[0])
	}
	if got[1].Line != 13 || got[1].Strong {
		t.Errorf("name-based finding should be weak and on line 13: %+v", got[1])
	}
	if !AnyStrong(got) || AnyStrong(got[1:]) {
		t.Error("AnyStrong wrong")
	}
}

func TestScanIgnoresRemovedLines(t *testing.T) {
	diff := "+++ b/x\n@@ -1 +1 @@\n-" + fake("AKIA", "ABCDEFGHIJKLMNOP") + "\n+clean\n"
	if got := Scan(diff); len(got) != 0 {
		t.Errorf("removed secrets must not count: %+v", got)
	}
}
