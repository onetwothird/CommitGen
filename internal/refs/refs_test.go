package refs

import "testing"

func TestFromBranch(t *testing.T) {
	cases := map[string]string{
		"fix/123-login-crash":                    "#123",
		"123-login":                              "#123",
		"feature/gh-45":                          "#45",
		"issue-7-crash":                          "#7",
		"user/issue_88":                          "#88",
		"feature/PROJ-42-search":                 "PROJ-42",
		"ENG-7":                                  "ENG-7",
		"main":                                   "",
		"release/1.2":                            "",
		"hotfix-2024-05-01":                      "",
		"feat/oauth2-login":                      "",
		"dependabot/npm_and_yarn/lodash-4.17.21": "",
		"feat/v2-api":                            "",
		"":                                       "",
	}
	for branch, want := range cases {
		if got := FromBranch(branch); got != want {
			t.Errorf("FromBranch(%q) = %q, want %q", branch, got, want)
		}
	}
}

func TestAddFooter(t *testing.T) {
	cases := []struct{ msg, want string }{
		{"feat: add search", "feat: add search\n\nRefs #12"},
		{"feat: add search\n\n- index titles", "feat: add search\n\n- index titles\n\nRefs #12"},
		{"feat!: drop v1\n\nBREAKING CHANGE: v1 is gone", "feat!: drop v1\n\nBREAKING CHANGE: v1 is gone\nRefs #12"},
		{"fix: crash (#12)", "fix: crash (#12)"},
		{"fix: crash\n\nCloses #12", "fix: crash\n\nCloses #12"},
		{"fix: crash in #123", "fix: crash in #123\n\nRefs #12"},
	}
	for _, c := range cases {
		if got := AddFooter(c.msg, "Refs", "#12"); got != c.want {
			t.Errorf("AddFooter(%q)\n got %q\nwant %q", c.msg, got, c.want)
		}
	}
	if got := AddFooter("feat: x", "", "#12"); got != "feat: x" {
		t.Error("empty footer word must disable the footer")
	}
}
