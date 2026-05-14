# GMCP — Generic MUD Communication Protocol

GMCP is the out-of-band data protocol many games use to send structured information: vitals updates, room descriptions, inventory deltas, channel events, target info, and so on. It lets us maintain rich state without having to scrape it from the human-readable output stream.

nogfx implements GMCP in [`pkg/gmcp/`](../../pkg/gmcp/), wrapped at the telnet layer through `pkg/gmcp/gmcp.go::Wrap` / `Unwrap`. World-specific extensions live under `pkg/gmcp/achaea/` and `pkg/gmcp/ironrealms/`.

## Telnet layer

GMCP rides inside a telnet subnegotiation block ([`telnet.md`](telnet.md)). The option byte is **201** (`0xC9`).

```
IAC SB GMCP <package.subpackage.command> <json> IAC SE
```

Decimal: `255 250 201 ... 255 240`.

A literal `0xFF` byte inside the JSON payload must be doubled (`IAC IAC`) to escape it from the surrounding `IAC SE`. In practice this is rare because GMCP payloads are UTF-8 JSON and `0xFF` isn't valid UTF-8.

## Negotiation

GMCP is enabled like any other telnet option. The canonical flow:

1. Server: `IAC WILL GMCP` ("I can do GMCP")
2. Client: `IAC DO GMCP` ("Please do it")
3. Client: `Core.Hello {"Client":"nogfx","Version":"..."}` — identifies the client
4. Client: `Core.Supports.Set ["Char 1","Char.Items 1",...]` — declares which packages it wants
5. Server begins sending GMCP messages from the negotiated packages

nogfx splits the response between two layers. The engine (`pkg/engine.go`) handles step 3 — when it sees `IAC WILL GMCP` it queues a `Core.Hello`. The world processor handles step 4 — Achaea's `cmdprocess` in `pkg/world/achaea/achaea.go` queues `Core.Supports.Set` on the same signal, with the package list Achaea cares about.

## Message format

Each message has two parts separated by a single space:

```
Package.Subpackage.Command <json>
```

- The package path is dot-separated, case-insensitive per the spec but conventionally `PascalCase` (e.g. `Char.Vitals`, `Comm.Channel.Text`).
- The JSON payload is optional — some messages are pure events.
- JSON keys are case-sensitive. UTF-8 encoded.

Example: `Char.Vitals {"hp":"4500","maxhp":"4800","mp":"3200",...}`

Iron Realms messages typically send numeric values as JSON strings (`"4500"` rather than `4500`). nogfx's `pkg/gmcp` decoders handle this through field-level converters. Don't assume numeric fields are JSON numbers.

## Package versioning

Packages are versioned by a trailing integer in the `Core.Supports.*` declaration:

```
Core.Supports.Set ["Char 1","Char.Items 1","Room 1"]
```

The number is the protocol version of that package, not a count. A bump to `Char 2` means the schema for `Char.*` messages has changed and the client is declaring it understands the new shape. The three Core.Supports verbs:

| Verb | Effect |
| --- | --- |
| `Core.Supports.Set [...]` | Replace the client's supported package list. |
| `Core.Supports.Add [...]` | Append to it. |
| `Core.Supports.Remove [...]` | Drop entries. |

## Standard packages

GMCP is a loose standard — Iron Realms publishes a baseline set and individual games extend it. The packages nogfx decodes today (from `pkg/gmcp/gmcp.go`):

| Package family | Purpose |
| --- | --- |
| `Core.*` | Lifecycle: `Hello`, `Goodbye`, `KeepAlive`, `Ping`, `Supports.{Set,Add,Remove}` |
| `Char.*` | Character state: `Login`, `Name`, `StatusVars`, `Status`, `Vitals` |
| `Char.Afflictions.*` | Negative status effects: `List`, `Add`, `Remove` |
| `Char.Defences.*` | Positive status effects: `List`, `Add`, `Remove` |
| `Char.Items.*` | Inventory and room contents: `Contents`, `Inv`, `Room`, `List`, `Add`, `Remove`, `Update` |
| `Char.Skills.*` | Skills: `Get`, `Groups`, `Info`, `List` |
| `Comm.Channel.*` | Communication channels: `Enable`, `List`, `Players`, `Text` |
| `Room.*` | Current room: `Info`, `Players`, `AddPlayer`, `RemovePlayer` |

## Iron Realms

Iron Realms games publish their own packages under the `IRE.*` prefix. The ones nogfx handles (under `pkg/gmcp/ironrealms/`):

| Package | Purpose |
| --- | --- |
| `IRE.Rift.*` | The Rift — Iron Realms' shared inventory store for herbs, salves, etc. nogfx sends `IRE.Rift.Request` to ask for the current contents. |
| `IRE.Target.*` | The current combat target: `Set` (we set it), `Info` (server tells us about it). |

Achaea also extends `Char.Status` and `Char.Vitals` beyond the baseline; those are decoded under `pkg/gmcp/achaea/`.

## Predecessors and cousins

- **ATCP** (option 200) — Achaea Telnet Client Protocol. GMCP's direct predecessor, similar wire format but a different payload convention. nogfx names the option byte (`telnet.ATCP`) but doesn't decode it.
- **MSDP** — MUD Server Data Protocol. A competing standard from the broader MUD community. Sometimes carried inside GMCP for portability. Not used by nogfx.

## References

- [Generic MUD Communication Protocol — tintin.mudhalla.net](https://tintin.mudhalla.net/protocols/gmcp/)
- [GMCP — mudstandards.org](https://mudstandards.org/mud/gmcp/)
- [Iron Realms Rapture GMCP manual](https://www.ironrealms.com/rapture/manual/files/FeatGMCP-txt.html)
- [Nexus GMCP help](https://nexus.ironrealms.com/GMCP)
