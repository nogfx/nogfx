.PHONY: all build test mocks

all: build test

build:
	go build ./...

test:
	go test ./...

# mocks regenerates the tcell.Screen mock. The pkg.Client/UI/Processor mocks
# the in-flight refactor used to regenerate referenced types that have since
# been retired; those targets will be reintroduced (against the new
# connection.Connection and ui.UI ports) once step 3 of the target-arch
# migration lands the port interfaces. See docs/plans/target-migration.md.
mocks: pkg/mock/tcell_screen.go

pkg/mock/tcell_screen.go: go.mod
	~/go/bin/moq -pkg mock ~/go/pkg/mod/github.com/gdamore/tcell/v2@v2.5.1/ Screen:ScreenMock > pkg/mock/tcell_screen.go
