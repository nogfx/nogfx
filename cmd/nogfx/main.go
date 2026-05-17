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
	"github.com/nogfx/nogfx/pkg"
	"github.com/nogfx/nogfx/platform/telnet"
	"github.com/nogfx/nogfx/platform/tui"
	"github.com/nogfx/nogfx/processors"
	"github.com/nogfx/nogfx/worlds/achaea"

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
		filepath.Join(pkg.Directory, "errors.log"),
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

	logProcessor, err := processors.LogProcessor(
		filepath.Join(pkg.Directory, "logs"),
		fmt.Sprintf(
			"%s-%s.log",
			strings.Split(address, ":")[0],
			time.Now().Format("20060102-150405"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create log processor: %w", err)
	}

	engine := &pkg.Engine{
		Connection: conn,
		UI:         terminal,
		Processor: app.Chain(
			processors.Input(),
			processors.RepeatInputProcessor(),
			processors.Output(),
			logProcessor,
		),
	}

	switch address {
	case "achaea.com:23", "50.31.100.8:23":
		processor, err := achaea.Processor()
		if err != nil {
			return fmt.Errorf("failed to create Achaea processor: %w", err)
		}

		engine.Processor = processor
	}

	return engine.Run(ctx)
}

func newTUI() (*tui.TUI, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}

	return tui.NewTUI(screen), nil
}
