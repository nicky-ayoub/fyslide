# AI Agent Guide for FySlide

FySlide is a Fyne-based image browser and slideshow application with tagging, filtering, and random playback. This guide accelerates AI agent productivity by highlighting architectural patterns, critical pitfalls, and development conventions.

## Quick Start

**Build:** `make build` (builds both GUI and CLI)  
**Test:** `make test` (runs with race detector)  
**Format & Check:** `make fmt` && `make check`  
**Run:** `./bin/fyslide /path/to/images`

For detailed architecture, see [docs/architecture.md](docs/architecture.md). For feature list and flags, see [README.md](README.md).

## Critical: Fyne Threading Rule

⚠️ **Every UI operation from background goroutines must be wrapped in `fyne.Do()`**, or GPU memory corruption and segmentation faults occur. This is not optional—it's enforced by Fyne's OpenGL state machine.

**Pattern:**
```go
// ❌ WRONG — will crash GPU
go func() {
    app.canvas.Refresh()  // GPU memory corruption!
}()

// ✅ CORRECT
go func() {
    fyne.CurrentApp().Driver().CanvasForObject(widget).(*canvas.Canvas)
    fyne.Do(func() {
        app.canvas.Refresh()
    })
}()
```

**Where this applies:** Image loading in [app_logic.go](internal/ui/app_logic.go), thumbnail generation in [thumbnail.go](internal/ui/thumbnail.go), tag operations in [tagging_controller.go](internal/ui/tagging_controller.go), and status bar updates.

**Why it fails:** Background goroutines can corrupt GPU state if they interact with Fyne's OpenGL renderer. The fix ensures all UI operations run on Fyne's event loop thread.

**History:** Prior crashes in display.go lines 181–234 and app_logic.go lines 23–29 were caused by unwrapped `MainMenu().Refresh()`, `Toolbar.Refresh()`, and `SetImage()` calls. See [repo memory](/.memories/repo/fyslide-crash-fix.md) for details.

## Architecture Overview

**Layered design** with clear separation:

- **UI Layer** ([internal/ui/](internal/ui/)): Controllers, widgets, state management. Main entry: [gui.go](internal/ui/gui.go).
- **Application State** ([image_state.go](internal/ui/image_state.go)): Thread-safe hub for image lists, filters, current index, permutation managers (random mode).
- **Service Layer** ([internal/service/](internal/service/)): Mediates scanning, persistence, and image decoding. Single `Service` instance injected into controllers.
- **Persistence** ([internal/tagging/tagging.go](internal/tagging/tagging.go)): BoltDB-backed tag store. Global singleton opened at startup.
- **File Scanning** ([internal/scan/](internal/scan/)): Async file discovery via goroutine. Streams results to `ImageState` through channels.
- **Image Service** ([service/image.go](internal/service/image.go)): Decodes, caches, and generates thumbnails asynchronously.
- **Slideshow** ([internal/slideshow/slideshow.go](internal/slideshow/slideshow.go)): Periodic index advance using ticker; integrates with `ImageState` mutex.
- **CLI** ([cmd/fyslide-cli/](cmd/fyslide-cli/)): Cobra-based tool reusing the same services; testable via mock stores.

**Data Flow:** FileScanner streams files → ImageState batches updates → UI controllers observe state → ThumbnailManager lazy-loads thumbnails → Slideshow periodically advances index.

## Concurrency Patterns

- **Mutexes:** `ImageState`, `LogUIManager`, and mock stores use RWMutex for thread safety.
- **Channels:** FileScanner streams results; batch tag operations return results via channels.
- **Goroutines:** Scanning, image loading, thumbnail generation, and tag operations run in background.
- **Fyne Marshalling:** All UI updates from goroutines use `fyne.Do()` callbacks.
- **Shutdown:** UI cancels scan context, stops slideshow, closes DB. Check `if a.UI.MainWin == nil` for shutdown detection (not yet coordinated with context cancellation).

## Common Pitfalls & Notes

### 1. ImageState Multiple Writers Problem
Scanning, slideshow, filter operations, and direct navigation all write to `ImageState` without a serializing actor. Reads are safe (RWMutex), but concurrent writes can create subtle race windows. **Recommendation:** Use an `ImageStateManager` actor pattern (documented in [architecture.md](docs/architecture.md)).

### 2. Goroutine Lifecycle
Background workers detect shutdown by checking `if a.UI.MainWin == nil`, not via coordinated context cancellation. This is fragile—refactor to use `context.Context` passed through goroutines.

### 3. Custom Abstractions
- `PermutationManager` ([permutationmanager.go](internal/scan/permutationmanager.go)): Manages random-order iteration; replaces standard patterns.
- `Binder` ([binder.go](internal/customwidgets/binder.go)): Custom data binding for UI widgets.
- `NormalizeTags()` ([tagging.go](internal/tagging/tagging.go)): Uses regex splitting for tag normalization—worth documenting why regex is needed.

### 4. Testing
- CLI has testable mock store ([test_mockstore.go](cmd/fyslide-cli/test_mockstore.go)).
- GUI tests are minimal; integration tests in [tagging_integration_test.go](internal/tagging/tagging_integration_test.go) use real BoltDB.
- Run tests with: `make test` (enables race detector by default).

## File Structure & Key Files

```
cmd/fyslide/main.go              # GUI entry point
cmd/fyslide-cli/main.go          # CLI entry point
internal/ui/
  app.go                         # Main app widget
  app_lifecycle.go               # Init & startup
  image_state.go                 # Shared state model (RWMutex)
  app_logic.go                   # Image loading logic (Fyne.Do pattern)
  display.go                     # Canvas & zoom/pan
  tagging_controller.go          # Tag operation handlers
  thumbnail.go                   # Lazy thumbnail loading
  shortcut_table.go              # Keyboard shortcut definitions
internal/scan/
  files.go                       # Async file discovery
  permutationmanager.go          # Random iteration
internal/service/
  service.go                     # Mediator (scanning, persistence)
  image.go                       # Image decoding & caching
internal/tagging/tagging.go      # BoltDB tag store
docs/architecture.md             # Detailed architecture + sequence diagram
```

## Dependencies & Build Notes

- **Go 1.23+** required.
- **Fyne v2.6.0:** Desktop UI framework. Note: newer versions may have breaking changes.
- **BoltDB (bbolt):** Embedded key-value store for tags.
- **Cobra:** CLI framework.
- **C compiler required:** Fyne has C dependencies (build on Linux/macOS/Windows).

Asset generation: `make gen` uses `go:generate` to embed icons and assets into binary.

## Development Workflow

1. **New feature in UI:** Add to controller, respect Fyne threading rule.
2. **New tag operation:** Use `Service.TagStore` interface, wrap in `fyne.Do()` for callback updates.
3. **New image format support:** Extend `ImageService.DecodedImage()` decoder registry.
4. **Debugging crashes:** Check [repo memory crash fix](/.memories/repo/fyslide-crash-fix.md) for Fyne threading symptoms.

## When to Ask for Clarification

- Any assumptions about `ImageState` concurrent mutation (refactor to actor pattern first).
- Shutdown lifecycle or goroutine cleanup strategy changes.
- Adding new background workers (ensure Fyne.Do() wrapping and cancel-on-shutdown).
