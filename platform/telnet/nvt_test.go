package telnet_test

import (
	"net"

	"github.com/nogfx/nogfx/platform/telnet"
)

// Verify interface fulfilments.
var _ net.Conn = &telnet.NVT{}
