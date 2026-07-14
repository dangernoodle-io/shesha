package mcpx

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Transport is shesha's swappable protocol-transport seam. Only mcpx
// constructs or unwraps a Transport, so an HTTP implementation (MC-5) can be
// added later without breaking callers outside this package.
type Transport interface {
	transport() mcp.Transport
}

type wrappedTransport struct {
	t mcp.Transport
}

func (w wrappedTransport) transport() mcp.Transport {
	return w.t
}

// Stdio returns a Transport that communicates over stdin/stdout.
func Stdio() Transport {
	return wrappedTransport{t: &mcp.StdioTransport{}}
}

// InMemoryPair returns two Transports connected to each other for in-process
// testing (see testkit). Connect a Server to the first and a Client to the
// second, server first.
func InMemoryPair() (Transport, Transport) {
	a, b := mcp.NewInMemoryTransports()
	return wrappedTransport{t: a}, wrappedTransport{t: b}
}
