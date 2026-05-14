# nogfx [![Codacy grade badge](https://app.codacy.com/project/badge/Grade/6168e833879a4fd5b56a6776ffd05d7f)](https://app.codacy.com/gh/tobiassjosten/nogfx) [![Codacy coverage badge](https://app.codacy.com/project/badge/Coverage/6168e833879a4fd5b56a6776ffd05d7f)](https://app.codacy.com/gh/tobiassjosten/nogfx) [![MIT license](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Because the book is always better.

nogfx is a terminal MUD client written in Go. It speaks telnet and GMCP, renders a tcell-based TUI, and runs world-specific game logic on top — currently focused on Achaea and other Iron Realms games.

## Demonstration

![nogfx demonstration](nogfx-demo.gif)

## Installation

With [Homebrew](https://brew.sh/):

```bash
brew tap tobiassjosten/nogfx
brew install nogfx
```

## Usage

```bash
nogfx achaea.com:23
```
