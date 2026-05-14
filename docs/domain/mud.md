# MUDs

A **MUD** (Multi-User Dungeon — sometimes Multi-User Dimension or Domain) is a real-time, text-based virtual world that multiple players connect to over a network. The genre dates to MUD1 (1978), the first virtual world in video gaming, and remains active today via games like Achaea, Aardwolf, and Discworld MUD.

Most MUDs share a common vocabulary, mostly inherited from their tabletop and adventure-game roots. This page is the rosetta stone for that vocabulary; world-specific terms live under each world's directory (e.g. [`achaea/`](achaea/INDEX.md)).

## Core terms

| Term | Meaning |
| --- | --- |
| **Realm** / **game** / **world** | A single MUD instance. nogfx connects to one realm per session, via `nogfx host:port`. |
| **Room** | The atomic unit of location. The world is a graph of rooms connected by exits (north, south, up, etc.). A player is always "in" exactly one room. |
| **Exit** | A directed connection between two rooms. Often a compass direction; sometimes named (`enter portal`, `climb tree`). |
| **Area** / **zone** | A themed cluster of rooms (a forest, a city district, a dungeon). |
| **Prompt** | The short status line the server emits at the end of each output batch, summarising vitals like HP/mana/movement. The prompt is what `pkg/telnet`'s `GA` (Go Ahead) terminator delimits. |
| **NPC** / **mob** | Non-player character. Anything in the world that acts but isn't a player — monsters, shopkeepers, quest-givers. |
| **PC** | Player character — another human's avatar. |
| **Tell** / **whisper** | Private text from one player to another, regardless of location. |
| **Say** | Text spoken in the current room, visible to anyone else in that room. |
| **Channel** | A persistent broadcast group (city, clan, market, newbie). See per-world docs for which channels exist. |
| **Tick** | A periodic server event, often used for regeneration, resource respawns, or weather changes. |
| **Trigger** | A client-side rule that watches incoming text and reacts (sends a command, alerts the player, rewrites the line). Implemented in nogfx through `pkg/process` + `pkg/simpex`. |
| **Alias** | A client-side short name that expands into a longer command before being sent. |
| **GMCP** / **ATCP** / **MSDP** | Out-of-band data protocols layered on telnet. See [`gmcp.md`](gmcp.md). |

## Why text

MUDs trade graphics for descriptive density: a single room can convey weather, lighting, occupants, exits, and lore in a few lines. The terminal is the native frontend, which is why nogfx is a TUI rather than a graphical client. The tagline in the README — *"because the book is always better"* — is this idea in a sentence.
