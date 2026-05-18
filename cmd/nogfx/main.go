package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/platform/telnet"
	"github.com/nogfx/nogfx/platform/tui"
	"github.com/nogfx/nogfx/processors/achaea"
	"github.com/nogfx/nogfx/processors/generic"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/net/idna"
)

func main() {
	if err := realMain(); err != nil {
		log.Fatal(err)
	}
}

// realMain is the real entry point. It exists so that deferred cleanups
// (e.g. closing the error log) actually run; log.Fatal would skip defers.
func realMain() error {
	if len(os.Args) < 2 {
		return errors.New("usage: nogfx example.com:23")
	}

	f, err := os.OpenFile(
		filepath.Join(directory, "errors.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600,
	)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			log.Printf("close error log: %v", cerr)
		}
	}()
	log.SetOutput(f)

	address, err := parseAddress(os.Args[1])
	if err != nil {
		return err
	}

	return run(address)
}

// parseAddress normalises a user-supplied server address into "host:port"
// form. An empty input defaults to the placeholder "example.com:23" so the
// usage example documented in the README parses cleanly.
func parseAddress(address string) (string, error) {
	if address == "" {
		return "example.com:23", nil
	}

	if !strings.Contains(address, ":") {
		address += ":23"
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" {
		return "", fmt.Errorf("invalid address '%s'", address)
	}

	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid port '%s'", port)
	}

	host, err = idna.Lookup.ToASCII(host)
	if err != nil {
		return "", fmt.Errorf("invalid host '%s': %w", host, err)
	}

	return net.JoinHostPort(host, port), nil
}

func run(address string) error {
	ctx := context.Background()

	conn, err := telnet.Dial(ctx, address)
	if err != nil {
		return err
	}

	terminal, err := newTUI()
	if err != nil {
		return err
	}

	chain, err := buildChain(address)
	if err != nil {
		return err
	}

	eng := &app.Engine{
		Connection: conn,
		UI:         terminal,
		Processor:  chain,
	}

	return eng.Run(ctx)
}

// buildChain assembles the processor chain for a session. The shape is:
//
//	[raw log] + [generic input chain] + [world processors] + [scripts] +
//	[generic output] + [processed log]
//
// The world is unaware of logging, generic input/output translation, and
// user scripts; the composition root owns that wiring.
func buildChain(address string) (app.Processor, error) {
	logDir := filepath.Join(directory, "logs")
	now := time.Now().Format("20060102-150405")
	host := strings.Split(address, ":")[0]

	rawLog, err := generic.LogProcessor(
		logDir,
		fmt.Sprintf("%s-%s.raw.log", host, now),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create raw log processor: %w", err)
	}

	sessionLog, err := generic.LogProcessor(
		logDir,
		fmt.Sprintf("%s-%s.log", host, now),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session log processor: %w", err)
	}

	// @todo Read the CommandSeparator configuration and use that instead.
	sep := []byte{';'}

	preWorld := []app.Processor{
		rawLog,
		generic.Input(),
		generic.SplitInputProcessor(sep),
		generic.RepeatInputProcessor(),
	}

	var worldProcs []app.Processor
	switch address {
	case "achaea.com:23", "50.31.100.8:23":
		worldProcs = achaea.New().Processors()
	}

	// User scripts go here once a loader exists. They sit between the
	// world and the generic Output/log so that they see decoded events
	// from the world but still affect the final rendered output.
	var scripts []app.Processor

	postWorld := []app.Processor{
		generic.Output(),
		sessionLog,
	}

	all := make([]app.Processor, 0,
		len(preWorld)+len(worldProcs)+len(scripts)+len(postWorld))
	all = append(all, preWorld...)
	all = append(all, worldProcs...)
	all = append(all, scripts...)
	all = append(all, postWorld...)

	return app.Chain(all...), nil
}

func newTUI() (*tui.TUI, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}

	return tui.NewTUI(screen), nil
}
