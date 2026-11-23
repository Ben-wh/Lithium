package store

import "testing"

func TestExcerpt(t *testing.T) {
	body := "## Before you start\n\nYou'll need the **dock** and a [cable](https://example.com).\n\n![photo](/api/kb/images/x.png)\n\n- [x] Dock powered\n\n```\ncode\n```"
	want := "Before you start You'll need the dock and a cable. Dock powered code"
	if got := excerpt(body, 300); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if got := excerpt("word ", 3); got != "wor…" {
		t.Fatalf("truncation: %q", got)
	}
}
