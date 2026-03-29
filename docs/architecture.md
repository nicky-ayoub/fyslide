# FySlide — Software Architecture

## Overview

- **Purpose:** Describe FySlide's software architecture: components, startup sequence, interactions, concurrency patterns, strengths, and recommended improvements.
- **Scope:** Code structure and design decisions (not performance tuning).

## Components

- **UI Layer:** Responsible for the user-facing app and controllers. See `internal/ui/app_lifecycle.go`, `internal/ui/gui.go`.
- **Application State:** `ImageState` — thread-safe model of images, filters, current index, and permutation managers. See `internal/ui/image_state.go`.
- **Service Layer:** Coordinates scanning and tag persistence; abstracts `TagStore` and `FileScanner`. See `internal/service/service.go`.
- **Persistence / Tag DB:** BoltDB-backed implementation of `TagStore`. See `internal/tagging/tagging.go`.
- **File Scanning:** Discovers files and streams them to the app via channels; includes permutation manager. See `internal/scan/files.go`, `internal/scan/permutationmanager.go`.
- **Image Service:** Loads/inspects images and creates thumbnails for the UI. See `internal/service/image.go`.
- **Slideshow Manager:** Periodic advance of image index; integrates with `ImageState`. See `internal/slideshow/slideshow.go`.
- **Custom Widgets:** Fyne widgets used by UI. See `internal/customwidgets/`.
- **CLI:** Cobra-based CLI using the same services; testable via injected mocks. See `cmd/fyslide-cli/main.go`, `cmd/fyslide-cli/test_mockstore.go`.

## Startup Sequence (expanded)

1. `cmd/fyslide/main.go` calls `ui.CreateApplication()`.
2. `CreateApplication()` initializes persistence: open BoltDB (`TagDB`).
3. Construct `Service` with `TagStore` (TagDB) and `FileScanner`.
4. Construct `ImageService` (decoders, caches).
5. Build UI: controllers, widgets, and bind hosts (NavigationHost, ThumbnailHost, TagsViewHost).
6. Start background `FileScanner.Run(dir)` (non-blocking goroutine).
7. `FileScanner` streams `FileItem`s into `ImageState` via channel; `ImageState` notifies observers.
8. ThumbnailManager requests thumbnails from `ImageService` on demand; results returned asynchronously.
9. If enabled, `Slideshow Manager` starts and advances the index periodically (mutual exclusion on writes).
10. UI remains responsive — all background work marshalled back to the main thread for UI updates.

Shutdown: UI cancels scan (context), stops slideshow timer, closes DB, and exits.

### Startup Sequence Diagram

```mermaid
sequenceDiagram
  autonumber
  participant Main as cmd/fyslide/main.go
  participant UI as ui.CreateApplication()
  participant TagDB as TagDB (Bolt)
  participant Service as Service
  participant ImgSvc as ImageService
  participant FileScan as FileScanner (goroutine)
  participant ImageState as ImageState
  participant Thumb as ThumbnailManager
  participant Slideshow as Slideshow Manager

  Main->>UI: call CreateApplication()
  UI->>TagDB: open/initialize DB
  UI->>Service: construct Service(TagStore=TagDB, FileScanner)
  UI->>ImgSvc: construct ImageService (decoders, cache)
  UI->>UI: build UI components & controllers
  UI->>FileScan: start Run(dir)  (non-blocking)
  FileScan-->>ImageState: stream FileItem via channel
  ImageState-->>UI: emit state update / notify observers
  UI->>Thumb: request thumbnails (lazy)
  Thumb->>ImgSvc: request thumbnail generation
  ImgSvc-->>Thumb: return thumbnail (async)
  UI->>Slideshow: start if enabled
  Slideshow->>ImageState: periodic advance (mutex)
  Note over FileScan,ImgSvc: background goroutines; UI marshals updates to main thread
  UI->>TagDB: async tag operations via Service

  %% Shutdown sequence
  UI->>FileScan: request stop (context cancel)
  UI->>Slideshow: stop timer
  UI->>TagDB: close DB
  UI->>App: exit

```

## Data Flows & Interactions

- FileScanner streams discovered files into `ImageState` (channel → state update).
- `Service` mediates between UI controllers and persistence (`TagStore`) and scanning.
- UI components request image data from `ImageService` and marshal updates to the UI thread via Fyne event callbacks.
- Slideshow reads/writes the current index inside `ImageState`.
- ThumbnailManager requests thumbnails from `ImageService` and caches results locally.

## Concurrency & State Management

- Channels: file scanning outputs; batch tag operation results.
- Goroutines: asynchronous image loading, scanning, thumbnail generation, batch tagging operations.
- Mutexes/RWMutex: `ImageState`, `LogUIManager`, and mock stores.
- UI thread marshalling: background-to-UI updates use Fyne-safe mechanisms.

## Strengths

- Clear package separation: UI, service, scan, tagging, slideshow, and widgets are separated.
- Interfaces and mocks enable testability (`TagStore`, `FileScanner`).
- Asynchronous design prevents UI blocking.
- Thread-safety applied to shared state where needed.
- Asset pipeline and `go:generate` usage for embedding assets.

## Architectural Concerns & Recommendations

- **UI–Core coupling:** UI controllers sometimes contain business logic. Introduce thin application-layer facades so controllers only coordinate UI events.
- **Multiple writers to `ImageState`:** scanning, slideshow, filters and UI actions mutate `ImageState`. Centralize mutations via an `ImageStateManager` to serialize writes and emit events.
- **Interface granularity:** split broad interfaces into read/write roles (e.g., `TagStoreReader` / `TagStoreWriter`).
- **Error propagation & logging:** background goroutines log but seldom propagate structured errors to UI; add an error channel or event bus.
- **Lifecycle cancellation:** use `context.Context` for scanning, thumbnail generation and other long-running ops to enable coordinated shutdown.
- **Configuration centralization:** use a typed `Config` struct injected into services and controllers instead of package-level constants.

## Appendix A — ImageStateManager (refactor draft)

- **Problem:** `ImageState` is mutated by multiple components (scanner, slideshow, filter operations), making ownership and concurrency reasoning harder.
- **Proposal:** add an `ImageStateManager` (single-writer actor) with responsibilities:
  - Methods for intent operations: `AddFiles(...)`, `SetFilter(...)`, `AdvanceIndex(...)`, `SetIndex(...)`, `ReplaceAll(...)`.
  - Serialize state modifications via a command channel or dedicated goroutine.
  - Broadcast state-change events to subscribers (controllers, thumbnail manager) via channels or callbacks executed on the UI thread.
  - Provide read-only snapshot accessors for UI reads.
- **Why it helps:** clarifies ownership, reduces race risk, simplifies testing and reasoning about state transitions.
- **Minimal sketch:**

```go
// pseudocode sketch
type ImageCommand struct { Op string; Payload interface{} }
func (m *ImageStateManager) Run(ctx context.Context, cmds <-chan ImageCommand) {
  for {
    select {
    case cmd := <-cmds:
      // apply command to internal state and emit snapshot
    case <-ctx.Done():
      return
    }
  }
}
```

Start by converting a single writer (e.g., `Slideshow`) to call manager methods as a pilot.

## Appendix B — Integration Test Scaffold

- **Goal:** add integration tests validating the pipeline: scan → state → service interactions without the GUI.
- **Why:** unit tests exist, but higher-level integration tests catch regressions in component interactions (scanning, tag persistence, permutation logic).
- **Scaffold:**
  - `test/integration/scan_integration_test.go` or `internal/integration/scan_integration_test.go`.
  - Test pattern:
    1. Create a temp directory with test files (fixtures or empty files with correct extensions).
    2. Start a BoltDB `TagDB` on a temp file (or use the in-memory mock for fast tests).
    3. Construct a `Service` with the `FileScanner` and `TagStore` pointing to temp resources.
    4. Run the scanner (with a `context.Context`) and wait/poll for expected items.
    5. Assert that `ImageState` contains expected items and tag operations persist correctly.

- **Minimal test sketch (pseudocode):**

```go
func TestScanAndPersist(t *testing.T) {
  dir := t.TempDir()
  // create files in dir
  dbPath := filepath.Join(t.TempDir(), "tags.db")
  tagDB := tagging.NewBoltTagDB(dbPath)
  svc := service.New(tagDB, scan.NewScanner())
  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()
  go svc.Scan(ctx, dir)
  // wait/poll for state population
  // assert expected length and persistence
}
```

This test avoids the GUI entirely and exercises core interactions.

## Next Steps

- I saved this architecture document as [docs/architecture.md](docs/architecture.md).
- I can implement a small pilot refactor for `ImageStateManager` or add the integration test scaffold—tell me which to implement next.
