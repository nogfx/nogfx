.PHONY: all build test mocks

all: build test

build:
	go build ./...

test:
	go test ./...

# mocks regenerates the tcell.Screen mock used by platform/tui tests. The
# mock lives as a _test.go file in the tui package so it ships only at test
# time. Other mocks (for the connection.Connection and ui.UI ports) can be
# added here as needed.
mocks: platform/tui/screen_mock_test.go

platform/tui/screen_mock_test.go: go.mod
	~/go/bin/moq -pkg tui ~/go/pkg/mod/github.com/gdamore/tcell/v2@v2.5.1/ Screen:ScreenMock > platform/tui/screen_mock_test.go
