package editor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

var unixFallbacks = []string{"nano", "vim", "vi"}

// Open opens path in the system editor and waits for it to exit.
func Open(path string) error {
	bin, err := resolveEditor()
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %q: %w", bin, err)
	}
	return nil
}

func resolveEditor() (string, error) {
	if v := os.Getenv("VISUAL"); v != "" {
		return v, nil
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e, nil
	}
	fallbacks := unixFallbacks
	if runtime.GOOS == "windows" {
		fallbacks = []string{"notepad"}
	}
	for _, name := range fallbacks {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no editor found: set $VISUAL or $EDITOR")
}
