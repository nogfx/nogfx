package telnet

import (
	"context"
	"fmt"
	"net"
)

// Dial opens a TCP connection to address and wraps it in an NVT. The context
// bounds the dial attempt; pass context.Background() if no deadline is wanted.
func Dial(ctx context.Context, address string) (*NVT, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", address, err)
	}
	return NewNVT(conn), nil
}
