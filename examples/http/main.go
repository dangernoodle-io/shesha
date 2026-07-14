// Command http demonstrates serving a shesha App over streamable-HTTP as
// one handler among the consumer's own on a single mux/server — proving
// shesha's HTTPHandler is a bare, path-agnostic http.Handler and that
// MCP-over-HTTP is entirely opt-in.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/dangernoodle-io/shesha"
	"github.com/dangernoodle-io/shesha/host/generic"
	"github.com/dangernoodle-io/shesha/httpx"
	"github.com/dangernoodle-io/shesha/mcpx"
)

type helloIn struct {
	Name string `json:"name" jsonschema:"name to greet"`
}

type helloOut struct {
	Greeting string `json:"greeting"`
}

type helloCap struct{}

func (helloCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{
		Name:        "hello",
		Description: "greets the caller by name",
	}, shesha.ReadOnly, func(_ context.Context, _ *mcpx.CallToolRequest, in helloIn) (*mcpx.CallToolResult, helloOut, error) {
		name := in.Name
		if name == "" {
			name = "world"
		}
		return nil, helloOut{Greeting: fmt.Sprintf("hello, %s!", name)}, nil
	})
	return nil
}

// newMux builds the co-mount example: MCP as just one handler among the
// consumer's own on a single mux, via httpx.NewMux.
func newMux(app *shesha.App) *http.ServeMux {
	// The mount path is the consumer's choice — "/mcp" here is illustrative,
	// not required. shesha imposes no route, and calling HTTPHandler at all
	// is opt-in: a server that never serves MCP over HTTP simply never calls it.
	mux := httpx.NewMux("/mcp", app.HTTPHandler())
	// The same mux/server can serve unrelated purposes alongside MCP.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func main() {
	app, err := shesha.New(shesha.Info{Name: "http-demo", Version: "0.0.1"}, generic.New(), helloCap{})
	if err != nil {
		log.Fatalf("compose app: %v", err)
	}

	// httpx.Serve handles SIGINT/SIGTERM itself, so context.Background()
	// is all that's needed here: Ctrl-C triggers a graceful shutdown
	// instead of the bare log.Fatal(ListenAndServe(...)) this example used
	// to have.
	if err := httpx.Serve(context.Background(), ":8080", newMux(app)); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
