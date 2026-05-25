---
name: tagging-safety
description: "Safely implement tag operations using TagStore and Fyne threading. Use when: adding tag operations, modifying tag persistence, refactoring tag callbacks, or implementing batch tagging features."
argument-hint: "Describe the tag operation: adding tags, removing tags, filtering, batch operations, etc."
---

# Safe Tagging Operations in FySlide

## When to Use

- Adding new tag operations (add, remove, filter, bulk tag)
- Modifying tag persistence or database interactions
- Refactoring or debugging tag-related callbacks
- Implementing batch tagging workflows
- Ensuring thread safety in tag operations
- Wrapping tag updates with Fyne UI marshalling

## Architecture

FySlide uses a **service layer** to isolate tag operations from UI logic:

```
UI Controller (tagging_controller.go)
  ↓
Service.TagStore interface (service.go)
  ↓
BoltDB Implementation (tagging.go)
  ↓
fyslide_tags.db (user config directory)
```

All UI updates from tag operation callbacks must be wrapped in `fyne.Do()` per the [Fyne threading rule](../check-fyne-threading/SKILL.md).

## Core Principles

1. **Use `Service.TagStore` interface**, not direct DB calls
2. **Batch tag operations** when possible (fewer DB roundtrips)
3. **Wrap UI callbacks** in `fyne.Do()` after tag operations
4. **Handle errors** gracefully in UI feedback
5. **Keep tag logic** separate from UI rendering

## Implementation Patterns

### Single Tag Operation (Add/Remove)

```go
// ❌ WRONG: Direct UI call without marshalling
func (tc *TaggingController) addTag(tag string) {
    err := tc.service.TagStore.AddTag(tc.currentImageID, tag)
    if err != nil {
        tc.display.updateStatusBar("Error: " + err.Error())  // Not marshalled!
        return
    }
    tc.display.RefreshTags()  // Not marshalled!
}

// ✅ CORRECT: Wrap UI updates in fyne.Do()
func (tc *TaggingController) addTag(tag string) {
    err := tc.service.TagStore.AddTag(tc.currentImageID, tag)
    
    fyne.Do(func() {
        if err != nil {
            tc.display.updateStatusBar("Error: " + err.Error())
            return
        }
        tc.display.RefreshTags()
        tc.display.updateInfoText()
    })
}
```

### Batch Tag Operations (Background Goroutine)

```go
// ✅ CORRECT: Async batch tagging with results via callback
func (tc *TaggingController) batchAddTags(imageIDs []string, tags []string) {
    go func() {
        var results []error
        
        // Perform expensive tag operations off UI thread
        for _, imageID := range imageIDs {
            for _, tag := range tags {
                err := tc.service.TagStore.AddTag(imageID, tag)
                if err != nil {
                    results = append(results, err)
                }
            }
        }
        
        // Marshal ALL UI updates back to Fyne thread
        fyne.Do(func() {
            successCount := len(imageIDs)*len(tags) - len(results)
            if len(results) > 0 {
                tc.display.updateStatusBar(
                    fmt.Sprintf("Tagged %d images; %d errors", successCount, len(results)),
                )
            } else {
                tc.display.updateStatusBar(
                    fmt.Sprintf("Successfully tagged %d images", successCount),
                )
            }
            tc.display.RefreshTags()
            tc.display.updateInfoText()
        })
    }()
}
```

### Filter by Tags (Read Operation)

```go
// ✅ CORRECT: Read tags and update filter (no background needed for simple ops)
func (tc *TaggingController) filterByTag(tag string) {
    // TagStore.GetImagesByTag is relatively fast
    imageIDs, err := tc.service.TagStore.GetImagesByTag(tag)
    if err != nil {
        tc.display.updateStatusBar("Filter failed: " + err.Error())
        return
    }
    
    // Update state and refresh UI
    tc.imageState.SetFilters(imageIDs)
    tc.display.RefreshImageList()
}
```

### Removing All Instances of a Tag (Batch + Cleanup)

```go
// ✅ CORRECT: Batch removal with progress feedback
func (tc *TaggingController) removeTagGlobally(tag string) {
    go func() {
        // Find all images with this tag
        imageIDs, err := tc.service.TagStore.GetImagesByTag(tag)
        if err != nil {
            fyne.Do(func() {
                tc.display.updateStatusBar("Lookup failed: " + err.Error())
            })
            return
        }
        
        // Remove tag from all images
        var removalErrors []error
        for _, imageID := range imageIDs {
            err := tc.service.TagStore.RemoveTag(imageID, tag)
            if err != nil {
                removalErrors = append(removalErrors, err)
            }
        }
        
        // Marshal UI updates
        fyne.Do(func() {
            if len(removalErrors) > 0 {
                tc.display.updateStatusBar(
                    fmt.Sprintf("Removed tag from %d images; %d failed",
                        len(imageIDs)-len(removalErrors), len(removalErrors)),
                )
            } else {
                tc.display.updateStatusBar(
                    fmt.Sprintf("Removed tag from %d images", len(imageIDs)),
                )
            }
            tc.display.RefreshTags()
            tc.display.updateInfoText()
        })
    }()
}
```

## TagStore Interface Reference

From [internal/service/service.go](../../../internal/service/service.go):

```go
type TagStore interface {
    // Add a tag to an image
    AddTag(imageID, tag string) error
    
    // Remove a tag from an image
    RemoveTag(imageID, tag string) error
    
    // Get all tags for an image
    GetTags(imageID string) ([]string, error)
    
    // Get all images with a given tag
    GetImagesByTag(tag string) ([]string, error)
    
    // Get all tags in the database
    GetAllTags() ([]string, error)
    
    // Remove a tag globally (from all images)
    RemoveTagGlobally(tag string) error
    
    // Close database connection
    Close() error
}
```

## Gotchas & Common Mistakes

| Mistake | Problem | Fix |
|---------|---------|-----|
| Calling `RefreshTags()` without `fyne.Do()` | GPU crash if called from goroutine | Always wrap in `fyne.Do()` |
| Forgetting error handling in UI | Silent failures confuse users | Update status bar with error messages |
| Batch operations without goroutine | UI blocks during heavy tagging | Move batch logic to goroutine, marshal callback |
| Nested `fyne.Do()` calls | Deadlock if outer Do() waits for inner | Don't nest; structure callback hierarchy |
| Concurrent writes to TagStore | Race conditions in BoltDB | All writes are serialized by mutex in TagStore |
| Not updating ImageState after filtering | Stale image list displayed | Call `tc.imageState.SetFilters(...)` or similar |

## Testing Tag Operations

Integration tests use real BoltDB [tagging_integration_test.go](../../../internal/tagging/tagging_integration_test.go):

```go
// Create test DB
store, cleanup := createTestStore(t)
defer cleanup()

// Test AddTag
err := store.AddTag("image1", "vacation")
assert.NoError(t, err)

// Verify
tags, _ := store.GetTags("image1")
assert.Contains(t, tags, "vacation")
```

CLI has mock store [test_mockstore.go](../../../cmd/fyslide-cli/test_mockstore.go) for unit testing without DB.

## References

- [AGENTS.md](../../../AGENTS.md) — Fyne threading rule and concurrency patterns
- [check-fyne-threading skill](../check-fyne-threading/SKILL.md) — Detect threading violations
- [Architecture Diagram](../../../docs/architecture.md) — Service layer design
- [TagStore Implementation](../../../internal/tagging/tagging.go) — BoltDB backend
