package ui

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// TestSetImgGetLoadedImagePathConcurrency stresses concurrent SetImg and
// GetLoadedImagePath calls to ensure there are no data races and that reads
// return either an empty string or one of the values set by writers.
func TestSetImgGetLoadedImagePathConcurrency(t *testing.T) {
	a := &App{}

	// prepare a list of known paths writers will set
	paths := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		paths = append(paths, "path-"+strconv.Itoa(i))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// channel to collect unexpected values
	errs := make(chan string, 100)

	// start writers
	for w := 0; w < 4; w++ {
		go func(id int) {
			i := id
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				// cycle through the paths
				p := paths[i%len(paths)]
				a.SetImg(Img{Path: p, EXIFData: make(map[string]string)})
				i += 1
			}
		}(w)
	}

	// start readers
	for r := 0; r < 8; r++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				p := a.GetLoadedImagePath()
				if p == "" {
					// allowed (may be before first writer runs)
					continue
				}
				// check it matches one of the known prefixes
				matched := false
				for _, v := range paths {
					if p == v {
						matched = true
						break
					}
				}
				if !matched {
					errs <- p
					return
				}
			}
		}()
	}

	// wait for context to expire or an error to occur
	select {
	case e := <-errs:
		t.Fatalf("read unexpected path: %s", e)
	case <-ctx.Done():
		// success; no unexpected values observed
	}
}
