# Telnet

Telnet is the transport nogfx speaks. RFC 854 (1983) is the foundational spec; RFC 855 adds option negotiation. This page is scoped to the subset nogfx actually implements — see `pkg/telnet/telnet.go` for the constants and `pkg/telnet/negotiation.go` and `nvt.go` for the parsing.

## Telnet is symmetric, not client-server

This is the common misconception worth getting right. Telnet *transports* over TCP, where one endpoint accepts connections and the other initiates, but the *protocol* itself is symmetric. RFC 854 is explicit:

> As much as possible, the TELNET protocol has been made server-user symmetrical so that it easily and naturally covers the user-user (linking) and server-server (cooperating processes) cases.

Both ends are Network Virtual Terminals (NVTs). Either side can send option negotiations and either side can request the other do something. When the codebase reacts to `IAC_WILL_GMCP` by sending `IAC_DO_GMCP`, that's the protocol's symmetry — one end is announcing a capability, and the other is opting in. Nothing in the wire format distinguishes "client" from "server."

The asymmetry is operational, not architectural. The server tends to do most of the talking because that's what MUDs do, not because telnet requires it.

## The NVT model

Each endpoint is an NVT — a virtual line-oriented terminal that defaults to:

- 7-bit US ASCII with the high bit reserved for `IAC` framing.
- Half-duplex, with explicit Go-Ahead (`GA`) signals delimiting turns.
- `CR LF` as end-of-line.

Options upgrade the NVT: negotiating away the GA gives you full duplex (Suppress Go-Ahead), negotiating Echo gives you password masking, negotiating GMCP gives you out-of-band data. Without negotiation, both ends just speak basic NVT.

## The IAC escape and command framing

Every telnet command starts with the IAC byte (255). The byte values nogfx names in `pkg/telnet/telnet.go`:

| Constant | Decimal | Hex | Meaning |
| --- | --- | --- | --- |
| `IAC` | 255 | `0xFF` | Interpret As Command. Anything after this is protocol, not data. |
| `Will` | 251 | `0xFB` | "I want to start doing X" |
| `Wont` | 252 | `0xFC` | "I will not do X" |
| `Do` | 253 | `0xFD` | "Please start doing X" |
| `Dont` | 254 | `0xFE` | "Please stop doing X" |
| `SB` | 250 | `0xFA` | Subnegotiation begin |
| `SE` | 240 | `0xF0` | Subnegotiation end |
| `GA` | 249 | `0xF9` | Go Ahead — turn-taking marker (see below) |
| `Echo` | 1 | `0x01` | Echo option |
| `SuppressGoAhead` | 3 | `0x03` | Suppress GA option |
| `TType` | 24 | `0x18` | Terminal Type option |
| `ATCP` | 200 | `0xC8` | Achaea Telnet Client Protocol (GMCP's predecessor) |
| `GMCP` | 201 | `0xC9` | Generic MUD Communication Protocol — see [`gmcp.md`](gmcp.md) |

To embed a literal `0xFF` in the data stream, send it twice (`IAC IAC`). Inside a subnegotiation block, the same escape applies to the payload.

## Option negotiation

Each option is independent and either side can initiate. The four-verb negotiation prevents endless ping-ponging:

| Initiator says | Responder says | Result |
| --- | --- | --- |
| `WILL X` | `DO X` | Initiator now does X |
| `WILL X` | `DONT X` | Initiator must not do X |
| `DO X` | `WILL X` | Responder now does X |
| `DO X` | `WONT X` | Responder must not do X |

The rule is: only respond if the state would change. Otherwise the offer/request gets dropped silently, so two endpoints that disagree don't loop forever.

nogfx handles two negotiations explicitly in `pkg/engine.go`:

- `IAC WILL ECHO` from the server → the engine calls `UI.MaskInput()` (server takes over echoing, which it does to hide passwords).
- `IAC WONT ECHO` from the server → `UI.UnmaskInput()`.

GMCP negotiation happens both at the engine level (sending `Core.Hello` on `IAC WILL GMCP`) and in the world processor (sending `Core.Supports.Set` on the same signal). See [`gmcp.md`](gmcp.md).

## Go Ahead (GA) and prompt delimiting

GA was originally a half-duplex turn signal. In MUDs it has been repurposed as the **prompt terminator**: the server emits a batch of output (room description, combat lines, channel messages), then a prompt, then `IAC GA`, signalling "your turn." Without GA the client would have to guess where one batch ends and the next begins.

nogfx uses GA exactly this way in the engine loop: bytes accumulate in a buffer until a GA byte arrives, then the buffered output is flushed downstream as one logical batch.

A server that has negotiated `Suppress Go-Ahead` will never send GA, which means it's running fully duplex and the client has to use timing or framing heuristics instead. Iron Realms servers send GA, so nogfx relies on it.

## Subnegotiation (SB/SE)

For options that need more than a yes/no, telnet provides subnegotiation blocks:

```
IAC SB <option> <payload...> IAC SE
```

`<option>` is the option byte (e.g. `GMCP` = 201). `<payload>` is option-specific. The block ends with `IAC SE`, and any `0xFF` byte inside the payload must be doubled to `IAC IAC`. GMCP messages live entirely inside these blocks; see [`gmcp.md`](gmcp.md) for the payload format.

## What nogfx does *not* implement

There's no support for MCCP (compression), ATCP, MSDP, MXP, NAWS, or charset negotiation. The bet is that GMCP is enough for Iron Realms games, which is correct for the current scope. If a world ever needs one of those, this is where to document it.

## References

- [RFC 854 — Telnet Protocol Specification](https://www.rfc-editor.org/rfc/rfc854)
- [RFC 855 — Telnet Option Specifications](https://www.rfc-editor.org/rfc/rfc855)
- [RFC 857 — Echo Option](https://www.rfc-editor.org/rfc/rfc857)
- [RFC 858 — Suppress Go Ahead Option](https://www.rfc-editor.org/rfc/rfc858)
