package server

import (
	"fmt"
	"net"

	platformerr "train/internal/platform/errors"
)

// Bind synchronously reserves a TCP listener on the given address.
//
// Pre-binding is a critical fail-fast invariant: the main goroutine claims the
// port BEFORE starting any background work or announcing readiness. If the port
// is already occupied, the process aborts immediately instead of failing late
// after databases and workers have already been initialized.
func Bind(addr string) (net.Listener, error) {
	const op = "infrastructure.server.Bind"

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, platformerr.Unavailable(op,
			fmt.Sprintf("bind address %s: %v", addr, err), err)
	}
	return ln, nil
}
