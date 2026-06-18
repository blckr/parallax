package main

import (
	"fmt"
	"os"

	"codeberg.org/blckr/parallax/internal/tui"
)

func main() {
	if err := tui.StartApp(); err != nil {
		fmt.Printf("Alas, TUI crashed: %v\n", err)
		os.Exit(1)
	}
}
