package testkit_test

import (
	"testing"
	"time"

	"github.com/dangernoodle-io/shesha/testkit"
)

// TestEventuallyContains_MissThenMatch deterministically exercises both
// branches of EventuallyContains' polling closure: the first poll returns a
// non-matching slice (covering the miss/"return false" path), and a later
// poll returns a matching slice (covering the "return true" path). The
// counter-driven fn makes this deterministic across runs, unlike a
// wall-clock race between the closure and the poll interval.
func TestEventuallyContains_MissThenMatch(t *testing.T) {
	const want = "target"

	var calls int
	fn := func() []string {
		calls++
		if calls < 3 {
			return []string{"other"}
		}
		return []string{want}
	}

	testkit.EventuallyContains(t, 200*time.Millisecond, 1*time.Millisecond, fn, want)

	if calls < 3 {
		t.Fatalf("expected fn to be polled at least 3 times before matching, got %d", calls)
	}
}

// TestEventuallyContains_ImmediateMatch covers the case where the very
// first poll already contains want.
func TestEventuallyContains_ImmediateMatch(t *testing.T) {
	const want = "target"

	fn := func() []string {
		return []string{want}
	}

	testkit.EventuallyContains(t, 200*time.Millisecond, 1*time.Millisecond, fn, want)
}
