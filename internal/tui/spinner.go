package tui

import (
	"fmt"
	"sync"
	"time"
)

// Spinner shows an animated status line and returns a stop function.
// In non-interactive terminals it prints the label once.
func Spinner(label string) func() {
	if !IsInteractive() {
		fmt.Printf("  %s…\n", label)
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-done:
				fmt.Printf("\r\033[K")
				return
			case <-time.After(80 * time.Millisecond):
				fmt.Printf("\r%s %s", Dim(frames[i%len(frames)]), label)
				i++
			}
		}
	}()
	return func() {
		once.Do(func() { close(done) })
		wg.Wait()
	}
}
