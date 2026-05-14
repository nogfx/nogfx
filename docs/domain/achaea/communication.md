# Communication

Communication in Achaea happens through a handful of actions, scoped from "the person standing next to you" out to "everyone on the continent."

## Local (room / area)

- **Say** — speech audible to anyone in the current room. Requires a shared language to be understood.
- **Whisper** — speech directed at one target in the room; quieter but still potentially overheard.
- **Yell** — loud speech audible to everyone in the surrounding area, regardless of level.
- **Emote** — non-verbal action description, visible in the room.

## Private (player-to-player)

- **Tell** — telepathic private message between any two players, anywhere in the world. Requires a shared language. The closest analogue to a DM.

## Group channels

- **City** (`CT`, `CITY`) — broadcast within the player's city ([`cities.md`](cities.md)). One per city.
- **House** (`HT`, `HNT`, `HTS`) — broadcast within the player's House. Variants exist for different ranks and for messaging house leaders.
- **Clan** (`CLT`) — broadcast within a single clan; players can belong to multiple.
- **Order** (`OT`) — broadcast within the player's divine order (members of the same god's order).

## Public

- **Newbie** — open-to-all help channel for new players; experienced players answer questions here.
- **Market** — buying and selling, available to players level 11+.
- **Shouts** — continent-wide broadcast, level 20+ players via a horn artefact.

## What nogfx models

`pkg/gmcp/comm.go` decodes the `Comm.Channel.*` family:

- `Comm.Channel.List` — which channels are available to the character.
- `Comm.Channel.Enable` — toggle whether the client wants events from a channel.
- `Comm.Channel.Players` — who is on a given channel.
- `Comm.Channel.Text` — actual channel messages, as they arrive.

The client can present each channel in its own pane, filter by channel, or merge into the main output stream — that's a UI decision, not a protocol one.

## References

- [Help: Communication — Achaea](https://www.achaea.com/game-help?what=communication)
- [Category:Channels — AchaeaWiki](https://wiki.achaea.com/Channels)
