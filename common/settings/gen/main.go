// Command gen writes the settings catalog for the console to web/src/generated/settings-catalog.json.
// It runs from common/settings (go generate) or from the repository root.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/modeltaps/modeltaps/common/settings"
)

func main() {
	out, err := settings.GeneratedJSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	path := settings.GeneratedPath()
	for _, prefix := range []string{"", "../../"} {
		if _, err := os.Stat(filepath.Join(prefix, "web/src")); err == nil {
			path = filepath.Join(prefix, path)
			break
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
