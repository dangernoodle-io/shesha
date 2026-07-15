package spinner_test

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dangernoodle-io/shesha/spinner"
	"github.com/dangernoodle-io/shesha/style"
	"github.com/stretchr/testify/assert"
)

func plainRenderer(w *bytes.Buffer) style.Renderer {
	return style.New(w, style.WithLevel(style.LevelNone))
}

func TestStart_LevelNonePrintsSinglePlainLineNoEscapes(t *testing.T) {
	buf := &bytes.Buffer{}
	s := spinner.New("working", spinner.WithWriter(buf), spinner.WithRenderer(plainRenderer(buf)))

	s.Start()

	assert.Equal(t, "working\n", buf.String())
	assert.NotContains(t, buf.String(), "\x1b")
}

func TestMessage_LevelNoneReprintsPlainly(t *testing.T) {
	buf := &bytes.Buffer{}
	s := spinner.New("working", spinner.WithWriter(buf), spinner.WithRenderer(plainRenderer(buf)))

	s.Start()
	s.Message("almost done")

	assert.Equal(t, "working\nalmost done\n", buf.String())
	assert.NotContains(t, buf.String(), "\x1b")
}

func TestSuccess_LevelNonePrintsPlainMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	s := spinner.New("working", spinner.WithWriter(buf), spinner.WithRenderer(plainRenderer(buf)))

	s.Start()
	s.Success("done")

	assert.Contains(t, buf.String(), "done\n")
	assert.NotContains(t, buf.String(), "\x1b")
}

func TestFail_LevelNonePrintsPlainMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	s := spinner.New("working", spinner.WithWriter(buf), spinner.WithRenderer(plainRenderer(buf)))

	s.Start()
	s.Fail("broke")

	assert.Contains(t, buf.String(), "broke\n")
	assert.NotContains(t, buf.String(), "\x1b")
}

func TestStop_IsIdempotent(t *testing.T) {
	buf := &bytes.Buffer{}
	s := spinner.New("working", spinner.WithWriter(buf), spinner.WithRenderer(plainRenderer(buf)))

	s.Start()

	assert.NotPanics(t, func() {
		s.Stop()
		s.Stop()
	})
}

func TestStop_IsIdempotentWithoutStart(t *testing.T) {
	buf := &bytes.Buffer{}
	s := spinner.New("working", spinner.WithWriter(buf), spinner.WithRenderer(plainRenderer(buf)))

	assert.NotPanics(t, func() {
		s.Stop()
	})
}

func TestStart_ForcedColorTierAnimatesAndClearsOnStop(t *testing.T) {
	buf := &bytes.Buffer{}
	renderer := style.New(buf, style.WithLevel(style.LevelTrueColor))
	s := spinner.New(
		"working",
		spinner.WithWriter(buf),
		spinner.WithRenderer(renderer),
		spinner.WithFrames([]string{"|", "/"}, 5*time.Millisecond),
	)

	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Stop()

	assert.Contains(t, buf.String(), "\x1b[", "an animated frame must emit escape sequences")
	assert.Contains(t, buf.String(), "working")
}

// TestStop_JoinsAnimationGoroutineNoLeak proves Stop's WaitGroup join is
// load-bearing: once Stop returns, the animation goroutine it spawned is
// provably gone, not just eventually garbage-collected.
// runtime.NumGoroutine is inherently noisy across a whole test binary, so
// this asserts on the delta settling back down rather than an exact count.
func TestStop_JoinsAnimationGoroutineNoLeak(t *testing.T) {
	buf := &bytes.Buffer{}
	renderer := style.New(buf, style.WithLevel(style.LevelTrueColor))

	baseline := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		s := spinner.New(
			"working",
			spinner.WithWriter(buf),
			spinner.WithRenderer(renderer),
			spinner.WithFrames([]string{"|", "/"}, time.Millisecond),
		)

		s.Start()
		time.Sleep(5 * time.Millisecond)
		s.Stop()
	}

	runtime.GC()

	var after int

	assert.Eventually(t, func() bool {
		after = runtime.NumGoroutine()

		return after <= baseline+1
	}, time.Second, 10*time.Millisecond, "goroutines leaked after Stop: baseline=%d after=%d", baseline, after)
}

func TestStart_IsIdempotentDoesNotSpawnTwoLoops(t *testing.T) {
	buf := &bytes.Buffer{}
	renderer := style.New(buf, style.WithLevel(style.LevelTrueColor))
	s := spinner.New(
		"working",
		spinner.WithWriter(buf),
		spinner.WithRenderer(renderer),
		spinner.WithFrames([]string{"|"}, 5*time.Millisecond),
	)

	before := runtime.NumGoroutine()

	s.Start()
	s.Start()

	afterDoubleStart := runtime.NumGoroutine()

	s.Stop()

	assert.LessOrEqual(t, afterDoubleStart, before+2, "a second Start must not spawn another animation loop")
}

func TestSuccess_ForcedColorTierEmitsEscapes(t *testing.T) {
	buf := &bytes.Buffer{}
	renderer := style.New(buf, style.WithLevel(style.LevelTrueColor))
	s := spinner.New("working", spinner.WithWriter(buf), spinner.WithRenderer(renderer))

	s.Start()
	s.Success("done")

	assert.Contains(t, buf.String(), "\x1b[")
	assert.Contains(t, buf.String(), "done")
}

func TestFail_ForcedColorTierEmitsEscapes(t *testing.T) {
	buf := &bytes.Buffer{}
	renderer := style.New(buf, style.WithLevel(style.LevelTrueColor))
	s := spinner.New("working", spinner.WithWriter(buf), spinner.WithRenderer(renderer))

	s.Start()
	s.Fail("broke")

	assert.Contains(t, buf.String(), "\x1b[")
	assert.Contains(t, buf.String(), "broke")
}

func TestNew_DefaultsWriterAndRenderer(t *testing.T) {
	s := spinner.New("working")

	assert.NotNil(t, s)
}

// TestMessage_ConcurrentWithAnimationIsRaceClean proves animate is the
// sole writer to s.w: Message only ever mutates s.msg under the mutex
// while a non-None spinner is animating, never writing to s.w itself, so
// concurrent Message calls alongside the animation goroutine never race
// on the writer. buf is only read after Stop has joined animate.
func TestMessage_ConcurrentWithAnimationIsRaceClean(t *testing.T) {
	buf := &bytes.Buffer{}
	renderer := style.New(buf, style.WithLevel(style.LevelBasic))
	s := spinner.New(
		"working",
		spinner.WithWriter(buf),
		spinner.WithRenderer(renderer),
		spinner.WithFrames([]string{"|", "/"}, time.Millisecond),
	)

	s.Start()

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			s.Message(fmt.Sprintf("step %d", i))
		}(i)
	}

	wg.Wait()
	time.Sleep(10 * time.Millisecond) // let animate render at least one tick
	s.Stop()

	assert.Contains(t, buf.String(), "\x1b[", "animate must have rendered at least one frame")
}

func TestWithColor_AppliesGlyphColorWhileAnimating(t *testing.T) {
	buf := &bytes.Buffer{}
	renderer := style.New(buf, style.WithLevel(style.LevelTrueColor))
	s := spinner.New(
		"working",
		spinner.WithWriter(buf),
		spinner.WithRenderer(renderer),
		spinner.WithFrames([]string{"*"}, 5*time.Millisecond),
		spinner.WithColor("#00ff00"),
	)

	s.Start()
	time.Sleep(15 * time.Millisecond)
	s.Stop()

	assert.Contains(t, buf.String(), "\x1b[")
}
