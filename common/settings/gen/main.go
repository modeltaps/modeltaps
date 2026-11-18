// Command gen writes the console's copy of the settings catalog
// (web/src/generated/settings-catalog.json) and the settings reference of the operator docs
// (docs/ops/<language>/settings/). It runs from common/settings (go generate) or the repository
// root.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/settings"
	"github.com/modeltaps/modeltaps/model"
)

// check compares the files instead of writing them, and fails on the first that differs.
var check_ = flag.Bool("check", false, "report out-of-date files instead of writing them")

func main() {
	flag.Parse()
	root := "."
	for _, prefix := range []string{".", "../.."} {
		if _, err := os.Stat(filepath.Join(prefix, "web/src")); err == nil {
			root = prefix
			break
		}
	}
	out, err := settings.GeneratedJSON()
	check(err)
	check(write(filepath.Join(root, settings.GeneratedPath()), out))

	model.RegisterOptions()
	langs, err := settings.LoadDocsLanguages(root)
	check(err)
	files := settings.DocsFiles(langs, config.GlobalOption.Defaults())
	for _, p := range settings.SortedPaths(files) {
		check(write(filepath.Join(root, "docs", "ops", p), []byte(files[p])))
	}
}

func write(path string, data []byte) error {
	if *check_ {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, data) {
			return fmt.Errorf("%s is out of date: run go generate ./common/settings", path)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
