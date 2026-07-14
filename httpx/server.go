package httpx

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// defaultShutdownTimeout is how long Serve waits for in-flight requests to
// drain during a graceful shutdown before giving up.
const defaultShutdownTimeout = 10 * time.Second

// NewMux returns a *http.ServeMux with mcpHandler mounted at mcpPath. The
// consumer registers its own routes on the returned mux, so MCP-over-HTTP
// is just one handler among the consumer's own on a single mux/server.
// httpx never imports shesha/mcpx/go-sdk itself — mcpHandler is a bare
// http.Handler the consumer builds (e.g. via App.HTTPHandler()).
func NewMux(mcpPath string, mcpHandler http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(mcpPath, mcpHandler)

	return mux
}

// serveConfig holds Serve's configurable behavior. Populated by ServeOption
// funcs; zero value is not valid on its own (see WithShutdownTimeout /
// newServeConfig for defaulting).
type serveConfig struct {
	shutdownTimeout time.Duration
}

// ServeOption configures Serve. Functional options keep the signature
// stable as options are added (e.g. a future MC-16 WithTLSConfig /
// WithCertKey pair).
type ServeOption func(*serveConfig)

// WithShutdownTimeout overrides Serve's default 10s graceful-shutdown
// timeout.
func WithShutdownTimeout(d time.Duration) ServeOption {
	return func(c *serveConfig) {
		c.shutdownTimeout = d
	}
}

// newServeConfig builds a serveConfig from opts, applied over the
// defaults.
func newServeConfig(opts []ServeOption) serveConfig {
	cfg := serveConfig{shutdownTimeout: defaultShutdownTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// Serve runs an http.Server on addr with handler until ctx is cancelled or
// SIGINT/SIGTERM arrives, then gracefully shuts down within the shutdown
// timeout (default 10s, override via WithShutdownTimeout). Blocks; returns
// nil on clean shutdown, the listen error if the server failed to bind, or
// the Shutdown error if graceful shutdown fails or times out.
//
// Mirrors the stdio lifecycle in package cli (signal.NotifyContext +
// Server.Shutdown), but for an http.Server: ListenAndServe runs in a
// goroutine, and the ctx-cancel path and the signal path share the same
// shutdown code below, since signal.NotifyContext derives its own ctx from
// the caller's — either cancellation unblocks the same select.
func Serve(ctx context.Context, addr string, handler http.Handler, opts ...ServeOption) error {
	cfg := newServeConfig(opts)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	notifyCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		// The server exited on its own (bind failure, or some other
		// non-ErrServerClosed error) before ctx/signal fired.
		return err
	case <-notifyCtx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	// Drain the goroutine's result so it doesn't leak; Shutdown having
	// returned guarantees ListenAndServe has already unblocked with
	// ErrServerClosed, so this receive does not block.
	<-errCh

	return nil
}
