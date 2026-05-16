package telnet

import (
	"fmt"
	"net"
)

// Dial opens a TCP connection to address and wraps it in an NVT.
func Dial(address string) (*NVT, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", address, err)
	}
	return NewNVT(conn), nil
}
