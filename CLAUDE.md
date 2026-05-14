# CLAUDE.md

CLAUDE.md contains instructions for how Claude Code should approach work in the codebase.

@README.md contains the high-level project description, mainly aimed at human consumption.

The `docs/` directory contains durable documentation about the project as a whole, from architectural decisions and design principles to domain knowledge. Each subdirectory has an `INDEX.md` listing its contents — start there, and keep it in sync when adding or renaming files. Update existing files when you add new knowledge to a topic, so that the documentation makes sense as a whole, and add new files when you have enough material to justify it.

Make sure to continuously capture durable insights and decisions in these files as you work, and to keep them up to date with the evolving codebase. Prefer writing new documentation in `docs/` rather than expanding this file, which should remain a thin index of instructions for how to work in the codebase.

## Development

- Verify that the application works as expected after each change, in order:
  - `go build -o /dev/null ./...` verifies compilation.
  - `go test ./...` verifies functionality.
  - `golangci-lint run` verifies code quality.
