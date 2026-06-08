package registration

import "testing"

func TestResolveMode_DefaultNormal(t *testing.T) {
	if got := ResolveMode(ModeInput{}); got != ModeNormal {
		t.Fatalf("got %q, want NORMAL", got)
	}
}

func TestResolveMode_EventLevel(t *testing.T) {
	in := ModeInput{EventModeSet: true, EventMode: ModeWarQueue}
	if got := ResolveMode(in); got != ModeWarQueue {
		t.Fatalf("got %q, want WAR_QUEUE", got)
	}
}

func TestResolveMode_CategoryOverride(t *testing.T) {
	in := ModeInput{
		EventModeSet: true, EventMode: ModeWarQueue,
		CategoryOverride: true, CategoryModeSet: true, CategoryMode: ModeBallot,
	}
	if got := ResolveMode(in); got != ModeBallot {
		t.Fatalf("got %q, want BALLOT (category override)", got)
	}
}

func TestResolveMode_CategoryOverrideDisabled_UsesEvent(t *testing.T) {
	in := ModeInput{
		EventModeSet: true, EventMode: ModeWarQueue,
		CategoryOverride: false, CategoryModeSet: true, CategoryMode: ModeBallot,
	}
	if got := ResolveMode(in); got != ModeWarQueue {
		t.Fatalf("got %q, want WAR_QUEUE (override disabled)", got)
	}
}
