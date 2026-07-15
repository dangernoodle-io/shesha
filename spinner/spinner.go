package spinner

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/dangernoodle-io/shesha/style"
)

const (
	defaultInterval = 80 * time.Millisecond

	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
	clearLine  = "\r\x1b[K"

	successGlyph = "✓" // ✓
	failGlyph    = "✗" // ✗

	successColor = "2" // ANSI green
	failColor    = "1" // ANSI red
)

var defaultFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner is the seam: a terminal progress indicator that can be started,
// updated, and resolved as a success or failure. A consumer may supply its
// own implementation; New returns shesha's default.
type Spinner interface {
	// Start begins the spinner and returns it, for one-line chaining
	// (spinner.New("...").Start()). Idempotent: calling Start again while
	// already running is a no-op.
	Start() Spinner
	// Message updates the in-progress text thread-safely.
	Message(msg string)
	// Stop halts the spinner and clears its line. Idempotent.
	Stop()
	// Success stops the spinner and prints a styled success line.
	Success(msg string)
	// Fail stops the spinner and prints a styled failure line.
	Fail(msg string)
}

type config struct {
	w        io.Writer
	renderer style.Renderer
	frames   []string
	interval time.Duration
	color    string
}

// Option configures New.
type Option func(*config)

// WithWriter sets the spinner's output stream. Default is os.Stderr —
// never stdout, which carries MCP stdio protocol traffic.
func WithWriter(w io.Writer) Option {
	return func(c *config) { c.w = w }
}

// WithRenderer overrides the style.Renderer used to draw frames and
// resolve lines. Default is style.New scoped to the configured writer.
func WithRenderer(r style.Renderer) Option {
	return func(c *config) { c.renderer = r }
}

// WithFrames overrides the animation frame set and tick interval. Default
// is a Braille dot sequence at 80ms.
func WithFrames(frames []string, interval time.Duration) Option {
	return func(c *config) {
		c.frames = frames
		c.interval = interval
	}
}

// WithColor sets the spinning glyph's style.Style.Fg (hex or ANSI index).
// Default is no color (dim/plain glyph).
func WithColor(color string) Option {
	return func(c *config) { c.color = color }
}

// spinner is shesha's default Spinner implementation.
type spinner struct {
	w        io.Writer
	renderer style.Renderer
	frames   []string
	interval time.Duration
	fg       string

	mu      sync.Mutex
	msg     string
	running bool
	done    chan struct{}
	wg      sync.WaitGroup
}

// New returns shesha's default Spinner, initially carrying msg, configured
// by opts.
func New(msg string, opts ...Option) Spinner {
	cfg := config{
		frames:   defaultFrames,
		interval: defaultInterval,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.w == nil {
		cfg.w = os.Stderr
	}
	if cfg.renderer == nil {
		cfg.renderer = style.New(cfg.w)
	}

	return &spinner{
		w:        cfg.w,
		renderer: cfg.renderer,
		frames:   cfg.frames,
		interval: cfg.interval,
		fg:       cfg.color,
		msg:      msg,
	}
}

// Start begins animating. At style.LevelNone (non-TTY/NO_COLOR/CI) it
// instead prints the message once, plainly, with no animation and no
// escape sequences. Idempotent.
func (s *spinner) Start() Spinner {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return s
	}
	s.running = true

	if s.renderer.Level() == style.LevelNone {
		fmt.Fprintln(s.w, s.msg) //nolint:errcheck

		return s
	}

	s.done = make(chan struct{})

	fmt.Fprint(s.w, hideCursor) //nolint:errcheck

	s.wg.Add(1)
	go s.animate()

	return s
}

// animate ticks frames at s.interval, rendering "<frame> <msg>" until
// s.done is closed by Stop. It is the sole background writer to s.w;
// Stop always joins it (wg.Wait) before writing anything itself, so no
// two goroutines ever write to s.w concurrently.
func (s *spinner) animate() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	i := 0

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			frame := s.frames[i%len(s.frames)]
			msg := s.msg
			s.mu.Unlock()

			line := s.renderer.Render(frame, style.Style{Fg: s.fg, Dim: s.fg == ""})
			fmt.Fprint(s.w, clearLine+line+" "+msg) //nolint:errcheck

			i++
		}
	}
}

// Message updates the in-progress text thread-safely. It never writes to
// s.w itself while animating — animate renders the current s.msg on its
// own next tick, avoiding a second concurrent writer. At style.LevelNone
// (no animation goroutine; single-threaded with the caller) each Message
// call deliberately reprints a new plain line, so CI/log-tail consumers
// still see progress updates.
func (s *spinner) Message(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.msg = msg

	if s.running && s.renderer.Level() == style.LevelNone {
		fmt.Fprintln(s.w, msg) //nolint:errcheck
	}
}

// Stop halts the spinner, joining its animation goroutine, and only then
// clears the line and restores the cursor — guaranteeing that final write
// happens after animate has fully exited, never concurrently with it.
// Idempotent — a second call is a no-op.
//
// Stop restores the cursor only on this normal path. shesha installs no
// signal handlers (see package doc); an uncaught SIGINT/SIGTERM during
// Start...Stop can leave the terminal cursor hidden. A consumer that
// wants the cursor restored on interrupt must call Stop from its own
// signal handler.
func (s *spinner) Stop() {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()

		return
	}
	s.running = false

	level := s.renderer.Level()
	done := s.done

	s.mu.Unlock()

	if level == style.LevelNone {
		return
	}

	close(done)
	s.wg.Wait()

	fmt.Fprint(s.w, clearLine+showCursor) //nolint:errcheck
}

// Success stops the spinner (joining animate) and only then prints a
// styled "✓ msg" line (plain text at style.LevelNone).
func (s *spinner) Success(msg string) {
	s.Stop()
	s.finish(successGlyph, successColor, msg)
}

// Fail stops the spinner (joining animate) and only then prints a styled
// "✗ msg" line (plain text at style.LevelNone).
func (s *spinner) Fail(msg string) {
	s.Stop()
	s.finish(failGlyph, failColor, msg)
}

// finish reads only s.renderer, which is set once at construction and
// never mutated afterward, so no lock is needed here; Stop has already
// joined animate, so this is the only writer to s.w at this point.
func (s *spinner) finish(glyph, color, msg string) {
	if s.renderer.Level() == style.LevelNone {
		fmt.Fprintln(s.w, msg) //nolint:errcheck

		return
	}

	fmt.Fprintln(s.w, s.renderer.Render(glyph, style.Style{Fg: color})+" "+msg) //nolint:errcheck
}
