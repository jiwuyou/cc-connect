package codex

import "testing"

func TestNormalizeMode_Aliases(t *testing.T) {
	tests := map[string]string{
		"":                   "suggest",
		"suggest":            "suggest",
		"default":            "suggest",
		"plan":               "suggest",
		"dontAsk":            "suggest",
		"acceptEdits":        "auto-edit",
		"accept-edits":       "auto-edit",
		"auto-edit":          "auto-edit",
		"full-auto":          "full-auto",
		"auto":               "full-auto",
		"bypassPermissions":  "yolo",
		"bypass-permissions": "yolo",
		"yolo":               "yolo",
		"unknown":            "suggest",
	}

	for raw, want := range tests {
		if got := normalizeMode(raw); got != want {
			t.Fatalf("normalizeMode(%q) = %q, want %q", raw, got, want)
		}
	}
}
