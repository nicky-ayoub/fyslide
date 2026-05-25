---
name: check-fyne-threading
description: "Detect and fix Fyne threading violations. Use when: adding background goroutines, modifying image loading, tagging operations, or any UI updates from async code. Prevents GPU memory corruption and segmentation faults."
argument-hint: "File path or code snippet to check for threading violations"
---

# Fyne Threading Violation Detector

## When to Use

- Adding new background goroutines that touch the UI
- Modifying image loading, thumbnail generation, or tag operations
- Updating UI state, menus, toolbars, or canvas from async callbacks
- Investigating crashes with error message: "Error in Fyne call thread, this should have been called in fyne.Do[AndWait]"
- Code review for tagging_controller, app_logic, display, or thumbnail modules

## The Rule

**Every single UI operation from background goroutines must be wrapped in `fyne.Do()`**, or GPU memory corruption and segmentation faults occur.

This is **not optional**—it's architectural law enforced by Fyne's OpenGL state machine.

## Anti-patterns (❌ DO NOT DO)

```go
// ❌ WRONG: Direct UI calls from goroutine
go func() {
    image := decodeImage(path)
    a.canvas.SetImage(image)      // GPU memory corruption!
    a.toolbar.Refresh()           // Crash!
    a.mainMenu.Refresh()          // Crash!
    a.display.updateStatusBar()   // Crash!
}()

// ❌ WRONG: Partial wrapping (some ops outside fyne.Do())
go func() {
    data := loadImageData(path)
    a.updateStatusBar()  // NOT wrapped!
    
    fyne.Do(func() {
        a.canvas.SetImage(data)  // Wrapped but too late
    })
}()

// ❌ WRONG: Wrapping only the final operation (early returns can slip out)
go func() {
    image, err := decodeImage(path)
    if err != nil {
        a.updateStatusBar()  // Not wrapped! Returns directly
        return
    }
    fyne.Do(func() {
        a.canvas.SetImage(image)
    })
}()
```

## Correct Patterns (✅ DO THIS)

```go
// ✅ CORRECT: Entire callback wrapped
go func() {
    image, err := decodeImage(path)
    
    fyne.Do(func() {
        if err != nil {
            a.updateStatusBar("Error: " + err.Error())
            return
        }
        a.canvas.SetImage(image)
        a.toolbar.Refresh()
        a.updateStatusBar("Image loaded")
    })
}()

// ✅ CORRECT: Separate async work, then marshalled UI update
go func() {
    // All expensive work here (no UI touching)
    image := decodeImage(path)
    thumbnail := generateThumbnail(image)
    
    // Then marshal ALL UI updates to Fyne thread
    fyne.Do(func() {
        a.canvas.SetImage(image)
        a.thumbnailCache[path] = thumbnail
        a.updateStatusBar("Ready")
    })
}()

// ✅ CORRECT: Batch tag operations with deferred UI updates
go func() {
    // Business logic without UI
    for _, tag := range tags {
        err := a.Service.TagStore.AddTag(imageID, tag)
        if err != nil {
            logError(err)
        }
    }
    
    // Marshal updates back to UI thread
    fyne.Do(func() {
        a.refreshTagsView()
        a.updateInfoText()
    })
}()
```

## Diagnostic Checklist

Before submitting code with goroutines:

- [ ] Every `go func()` is identified
- [ ] All UI operations (SetImage, Refresh, SetIcon, etc.) are inside `fyne.Do()`
- [ ] No early returns skip the `fyne.Do()` wrapper
- [ ] Panic/error paths also wrapped in `fyne.Do()`
- [ ] Custom update methods (e.g., `updateStatusBar()`, `RefreshTags()`) are wrapped when called from goroutines
- [ ] Slideshow/timer callbacks use `fyne.Do()` for state updates
- [ ] Image service callbacks use `fyne.Do()` for thumbnail caching

## Critical Files to Check

When reviewing or adding features:

| File | Common Threading Issues |
|------|------------------------|
| [internal/ui/app_logic.go](../../../internal/ui/app_logic.go) | Image loading goroutine; early empty-list returns |
| [internal/ui/thumbnail.go](../../../internal/ui/thumbnail.go) | Lazy thumbnail generation callbacks |
| [internal/ui/tagging_controller.go](../../../internal/ui/tagging_controller.go) | Batch tag operation callbacks |
| [internal/ui/display.go](../../../internal/ui/display.go) | Menu/toolbar refresh from zoom/pan handlers |
| [internal/slideshow/slideshow.go](../../../internal/slideshow/slideshow.go) | Ticker callbacks advancing index |
| [internal/service/image.go](../../../internal/service/image.go) | Image decoding async results |

## References

- [Architecture Diagram](../../../docs/architecture.md) — Understand data flow and goroutine boundaries
- [Crash History](/.memories/repo/fyslide-crash-fix.md) — Prior crashes and fixes (March 2026)
- [AGENTS.md](../../../AGENTS.md) — Threading rule and exemplary code patterns
- Fyne docs: [UI Thread Safety](https://pkg.go.dev/fyne.io/fyne/v2#Do)

## Quick Test

Use [check-threading.sh](./scripts/check-threading.sh) to scan for common violations:

```bash
./scripts/check-threading.sh <file.go>
```

This searches for `go func()` patterns that might be missing `fyne.Do()` wrappers.

## Common False Positives

- `fyne.Do` in comments (harmless, regex catches them)
- Goroutines that don't touch UI (e.g., pure file scanning)
- Non-Fyne UI operations in concurrent code

Review results manually to confirm violations.
