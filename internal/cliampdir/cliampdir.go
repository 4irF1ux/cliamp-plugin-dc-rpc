// Package cliampdir finds cliamp's config directory the way cliamp does, so
// the daemon and the installer agree on where cliamp.sock, config.toml and
// plugins/ live.
package cliampdir

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// Dir returns cliamp's config directory. Lookup order mirrors cliamp's appdir:
// CLIAMP_CONFIG_DIR, $XDG_CONFIG_HOME/cliamp, $HOME/.config/cliamp, then
// %APPDATA%\cliamp on Windows, then the user home fallback.
func Dir() (string, error) {
	if dir := os.Getenv("CLIAMP_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cliamp"), nil
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "cliamp"), nil
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "cliamp"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Join(errors.New("no config dir"), err)
	}
	return filepath.Join(home, ".config", "cliamp"), nil
}
