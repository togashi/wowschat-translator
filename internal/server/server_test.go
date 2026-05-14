package server

import (
	"testing"
	"time"
)

func TestNormalizeWhitespace(t *testing.T) {
	got := normalizeWhitespace("  Why\t the   hell\r\nare you shooting so fast?  ")
	want := "Why the hell are you shooting so fast?"
	if got != want {
		t.Fatalf("normalizeWhitespace() = %q, want %q", got, want)
	}
}

func TestDuplicateBurstGuard(t *testing.T) {
	s := &Server{
		duplicateBurstSkipEnabled:    true,
		duplicateBurstWindow:         80 * time.Millisecond,
		duplicateNormalizeWhitespace: true,
		duplicateLastSeen:            make(map[string]time.Time),
	}

	if s.shouldSkipDuplicateBurst("Why the hell are you shooting so fast?") {
		t.Fatal("first message should not be skipped")
	}
	if !s.shouldSkipDuplicateBurst("Why   the hell are you shooting so fast?") {
		t.Fatal("duplicate in window should be skipped")
	}

	time.Sleep(100 * time.Millisecond)
	if s.shouldSkipDuplicateBurst("Why the hell are you shooting so fast?") {
		t.Fatal("message after window should not be skipped")
	}
}
