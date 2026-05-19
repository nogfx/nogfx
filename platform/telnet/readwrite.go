package telnet

import (
	"bytes"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
)

// SplitFunc looks for string termination based on negotiated options. By
// default, newline and GA is used, but the latter can be negotiated.
func (nvt *NVT) SplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	lastCR := false
	for i, b := range data {
		if lastCR && b == '\n' && nvt.options[theirside][SuppressGoAhead].On() {
			return i + 1, data[:i+1], nil
		}

		lastCR = b == '\r'

		if b == GA {
			return i + 1, data[:i+1], nil
		}
	}

	if atEOF {
		return len(data), data, nil
	}

	// @todo Test this.
	return 0, nil, nil
}

// Read parses and returns data received from the server.
func (nvt *NVT) Read(buffer []byte) (count int, err error) {
	l := len(buffer)
	if l == 0 {
		return 0, nil
	}

	for lastCR := false; count < l; {
		b, err := nvt.buffer.ReadByte()
		if err != nil {
			return count, err
		}

		if b != IAC && len(nvt.cmdBuffer) == 0 {
			buffer[count] = b
			count++

			if lastCR && b == '\n' && nvt.options[theirside][SuppressGoAhead].On() {
				return count, nil
			}

			lastCR = b == '\r'

			continue
		}

		nvt.cmdBuffer = append(nvt.cmdBuffer, b)

		if !IsCommand(nvt.cmdBuffer) {
			continue
		}

		if bytes.Equal(nvt.cmdBuffer, []byte{IAC, IAC}) {
			nvt.cmdBuffer = []byte{}

			continue
		}

		if bytes.Equal(nvt.cmdBuffer, []byte{IAC, GA}) {
			nvt.cmdBuffer = []byte{}

			buffer[count] = GA
			count++

			return count, nil
		}

		nvt.surface(nvt.cmdBuffer)
		nvt.cmdBuffer = []byte{}
	}

	return count, nil
}

// surface decodes a complete IAC sequence into a typed event and pushes
// it onto the engine's events channel (or buffers it for later if Run
// hasn't been called yet). WILL/WONT/DO/DONT options arrive as
// TelnetCommand; IAC SB GMCP <payload> IAC SE arrives as GMCPFrame
// (payload only, no envelope bytes). Other subnegotiation packets
// surface as TelnetCommand for any processor that wants to interpret
// them.
//
// State inference is intentionally not done here — Write owns the
// outgoing side and that's what SplitFunc reads. Incoming WILL/DO
// doesn't flip our state until a processor decides to acknowledge it
// (the Send command flowing back through Apply updates the state).
//
// @todo Handle doubled-IAC escaping inside SB payloads (RFC 855).
// Achaea's GMCP JSON never contains raw 0xFF bytes, so this is
// theoretical for now; surface a real-world misframing before adding the
// unescape pass.
func (nvt *NVT) surface(cmd []byte) {
	ev := decodeIAC(cmd)
	if nvt.events != nil {
		nvt.events <- ev

		return
	}

	nvt.pendingEvents = append(nvt.pendingEvents, ev)
}

func decodeIAC(cmd []byte) app.Event {
	cp := append([]byte{}, cmd...)

	if len(cp) >= 5 && cp[0] == IAC && cp[1] == SB && cp[2] == GMCP &&
		cp[len(cp)-2] == IAC && cp[len(cp)-1] == SE {
		payload := append([]byte{}, cp[3:len(cp)-2]...)

		return connection.GMCPFrame{Payload: payload}
	}

	return connection.TelnetCommand{Bytes: cp}
}

// Write sends data to the server.
func (nvt *NVT) Write(data []byte) (int, error) {
	// Telnet specifies <CR><LF> endings, so we make sure we adhere.
	if ld := len(data); len(data) == 0 || data[0] != IAC {
		if ld > 2 && data[ld-2] == '\r' && data[ld-1] == '\n' {
			data = data[0 : ld-2]
		}

		data = append(data, '\r', '\n')
	}

	for i := 0; i < len(data); i++ {
		if data[i] != IAC {
			continue
		}

		// @todo Test this.
		if data[i+1] == SB {
			ii := bytes.IndexByte(data[i+1:], IAC)
			if ii < 0 {
				break
			}
		}

		switch data[i+1] {
		case Do:
			nvt.options[theirside][data[i+2]] = StateEnabling
		case Dont:
			nvt.options[theirside][data[i+2]] = StateDisabled
		case Will:
			nvt.options[ourside][data[i+2]] = StateEnabling
		case Wont:
			nvt.options[ourside][data[i+2]] = StateDisabled
		}

		if bytes.IndexByte([]byte{Do, Dont, Will, Wont}, data[i+1]) >= 0 {
			i += 2
		}
	}

	// @todo Pick up commands and mutate nvt.ourCoulds and nvt.theirCoulds.
	// @todo Potentially add Do(), Dont(), Will(), Wont() methods.

	return nvt.Conn.Write(data)
}
