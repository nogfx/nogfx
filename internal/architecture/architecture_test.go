// Package architecture_test verifies that every in-module package only
// imports from other categories the architecture permits.
//
// The check classifies each package by path into a Category (app, lib,
// contract, codec, endpoint, processors, world, runtime, cmd) and looks
// up the set of categories that category is allowed to depend on. Any
// internal import that isn't in the allowed set is reported as a failure.
//
// The categories and the allow-list together encode the architecture
// described in docs/architecture/overview.md. Changing either side should
// be a deliberate decision — if a new rule needs to land, update both.
package architecture_test

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const modulePath = "github.com/nogfx/nogfx"

// Category labels the architectural role of a package.
type Category string

const (
	// CategoryApp is the abstract pipeline core: Batch, Event, Command,
	// Processor, Chain. Imports nothing else in the project.
	CategoryApp Category = "app"

	// CategoryLib holds general-purpose libraries usable by anything in
	// the project. They import nothing else in the project so they stay
	// extractable.
	CategoryLib Category = "lib"

	// CategoryContract is an endpoint contract: the events the endpoint
	// emits and the commands it accepts. Imports only the abstract core
	// and shared libraries.
	CategoryContract Category = "contract"

	// CategoryCodec is the GMCP wire codec — typed message definitions
	// and decoders. Imports lib (for navigation conversions) but not
	// endpoint or contract packages.
	CategoryCodec Category = "codec"

	// CategoryEndpoint is an endpoint implementation (telnet, tui).
	// Imports the abstract core, its own contract, and libraries.
	CategoryEndpoint Category = "endpoint"

	// CategoryProcessors holds generic, world-agnostic processors that
	// translate between contracts and the codec.
	CategoryProcessors Category = "processors"

	// CategoryWorld is a game-specific plugin. May import the codec,
	// contracts, libraries, and generic processors, but never endpoint
	// implementations or the runtime.
	CategoryWorld Category = "world"

	// CategoryEngine is the engine that pumps batches between endpoints.
	// May import the abstract core, contracts, and libraries.
	CategoryEngine Category = "engine"

	// CategoryCmd is the binary entry point. May import everything.
	CategoryCmd Category = "cmd"
)

// classify assigns a Category to a package given its import path within
// the module. Returns an empty Category for paths it doesn't recognise so
// the test can flag unclassified packages explicitly.
func classify(importPath string) Category {
	rel := strings.TrimPrefix(importPath, modulePath+"/")
	switch {
	case rel == "app":
		return CategoryApp
	case rel == "connection", rel == "ui":
		return CategoryContract
	case strings.HasPrefix(rel, "lib/"):
		return CategoryLib
	case rel == "platform/gmcp", strings.HasPrefix(rel, "platform/gmcp/"):
		return CategoryCodec
	case strings.HasPrefix(rel, "platform/"):
		return CategoryEndpoint
	case rel == "processors":
		return CategoryProcessors
	case strings.HasPrefix(rel, "worlds/"):
		return CategoryWorld
	case rel == "engine":
		return CategoryEngine
	case strings.HasPrefix(rel, "cmd/"):
		return CategoryCmd
	case strings.HasPrefix(rel, "internal/"):
		// Internal test/tooling packages live outside the runtime
		// dependency graph; we don't enforce rules on them.
		return ""
	}
	return ""
}

// allowed lists, per category, the categories its packages may import
// from. The current category is implicitly allowed (a codec package may
// import another codec package, etc.).
var allowed = map[Category][]Category{
	CategoryApp:        {},
	CategoryLib:        {},
	CategoryContract:   {CategoryApp, CategoryLib},
	CategoryCodec:      {CategoryLib},
	CategoryEndpoint:   {CategoryApp, CategoryContract, CategoryLib},
	CategoryProcessors: {CategoryApp, CategoryContract, CategoryLib, CategoryCodec},
	CategoryWorld:      {CategoryApp, CategoryContract, CategoryLib, CategoryCodec, CategoryProcessors},
	CategoryEngine:     {CategoryApp, CategoryContract, CategoryLib},
	CategoryCmd: {
		CategoryApp, CategoryContract, CategoryLib, CategoryCodec,
		CategoryEndpoint, CategoryProcessors, CategoryWorld, CategoryEngine,
	},
}

func TestPackagesAreClassified(t *testing.T) {
	pkgs := loadModulePackages(t)
	for _, pkg := range pkgs {
		rel := strings.TrimPrefix(pkg.PkgPath, modulePath+"/")
		if strings.HasPrefix(rel, "internal/") {
			continue // internal tooling sits outside the rule set
		}
		if classify(pkg.PkgPath) == "" {
			t.Errorf("package %q has no architectural category — extend classify() in this file", pkg.PkgPath)
		}
	}
}

func TestDependencyDirection(t *testing.T) {
	pkgs := loadModulePackages(t)

	for _, pkg := range pkgs {
		cat := classify(pkg.PkgPath)
		if cat == "" {
			// reported by TestPackagesAreClassified; skip here.
			continue
		}

		for importPath := range pkg.Imports {
			if !strings.HasPrefix(importPath, modulePath) {
				continue // stdlib or third-party
			}
			depCat := classify(importPath)
			if depCat == "" {
				t.Errorf("%s imports unclassified %s", pkg.PkgPath, importPath)
				continue
			}
			if depCat == cat {
				continue // same-category imports are always fine
			}
			if !slices.Contains(allowed[cat], depCat) {
				t.Errorf(
					"%s (%s) imports %s (%s) — %s may not depend on %s",
					pkg.PkgPath, cat, importPath, depCat, cat, depCat,
				)
			}
		}
	}
}

func loadModulePackages(t *testing.T) []*packages.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedImports,
	}
	pkgs, err := packages.Load(cfg, modulePath+"/...")
	if err != nil {
		t.Fatalf("load packages: %v", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatalf("packages had load errors")
	}
	return pkgs
}
