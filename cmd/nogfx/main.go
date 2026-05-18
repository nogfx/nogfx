package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/platform/headless"
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
	flags := flag.NewFlagSet("nogfx", flag.ContinueOnError)
	headlessMode := flags.Bool("headless", false,
		"run without the TUI; read commands from stdin and write game output to stdout")
	flags.Usage = func() {
		if _, err := fmt.Fprintln(flags.Output(), "usage: nogfx [--headless] example.com:23"); err != nil {
			log.Printf("usage: %v", err)
		}
		flags.PrintDefaults()
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	args := flags.Args()
	if len(args) < 1 {
		return errors.New("usage: nogfx [--headless] example.com:23")
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

	address, err := parseAddress(args[0])
	if err != nil {
		return err
	}

	return run(address, *headlessMode)
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

func run(address string, headlessMode bool) error {
	ctx := context.Background()

	conn, err := telnet.Dial(ctx, address)
	if err != nil {
		return err
	}

	var terminal app.Endpoint
	if headlessMode {
		terminal = headless.New()
	} else {
		terminal, err = newTUI()
		if err != nil {
			return err
		}
	}

	chain, err := buildChain(address, headlessMode)
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
func buildChain(address string, headlessMode bool) (app.Processor, error) {
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
		// Telnet option negotiation. Runs early so its replies (IAC DO,
		// IAC WILL, etc.) land in the same batch as the trigger
		// TelnetCommand and are dispatched ahead of any world or
		// auto-login reactions that depend on the option being agreed
		// (e.g. GMCP must be agreed before Core.Supports.Set / Char.Login
		// are sent).
		generic.TelnetNegotiation(generic.DefaultNegotiation()),
		// Message aggregator. Buffers TextLine and GMCPFrame events;
		// emits a derived connection.Message on each Prompt. Additive —
		// per-event consumers (output renderer, raw log, GMCP state
		// updaters) keep seeing the underlying events unchanged. See
		// docs/design/messages.md.
		generic.Aggregator(),
		generic.Input(),
		generic.SplitInputProcessor(sep),
		generic.RepeatInputProcessor(),
	}

	// The event log is a per-event probe used for protocol/feature
	// investigation. It is on by default in headless mode (the assistant's
	// canonical observation surface; see docs/agent/conduct.md) and
	// opt-in elsewhere via NOGFX_DEBUG_EVENTS. The probe sits at the very
	// head of the chain so it sees every event before any transformation.
	if headlessMode || os.Getenv("NOGFX_DEBUG_EVENTS") != "" {
		eventLog, err := generic.EventLogProcessor(
			logDir,
			fmt.Sprintf("%s-%s.events.log", host, now),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create event log processor: %w", err)
		}
		preWorld = append([]app.Processor{eventLog}, preWorld...)
	}

	creds, err := loadCredentials(directory, host)
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
	}

	var worldProcs []app.Processor
	switch address {
	case "achaea.com:23", "50.31.100.8:23":
		worldProcs = append(worldProcs, achaea.New().Processors()...)
	}
	// Auto-login is GMCP-based (Char.Login) and therefore world-agnostic
	// for the Mudlet/Iron Realms family. It sits after world processors so
	// the world's own GMCP-support announcement (Core.Supports.Set) is
	// queued first; with both in the same batch, the engine applies them
	// in order.
	worldProcs = append(worldProcs, generic.AutoLogin(creds))

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
