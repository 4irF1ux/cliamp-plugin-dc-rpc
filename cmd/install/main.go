// Command install sets up the Discord Rich Presence plugin for cliamp in one
// step:
//
//	go run github.com/4irF1ux/cliamp-plugin-dc-rpc/cmd/install@latest
//
// It installs dc-rpc.lua with cliamp's own plugin installer, builds
// cliamp-rpcd with go install, and adds cliamp-rpcd to allowed_binaries in
// cliamp's config.toml. Restart cliamp afterwards to load the plugin.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/4irF1ux/cliamp-plugin-dc-rpc/internal/cliampdir"
)

const (
	defaultSource = "4irF1ux/cliamp-plugin-dc-rpc"
	defaultDaemon = "github.com/4irF1ux/cliamp-plugin-dc-rpc/cmd/cliamp-rpcd@latest"
	pluginName    = "dc-rpc"
	daemonName    = "cliamp-rpcd"
)

func main() {
	source := flag.String("source", defaultSource, "plugin source passed to cliamp plugins install")
	daemon := flag.String("daemon", defaultDaemon, "daemon package passed to go install")
	flag.Parse()
	if err := run(*source, *daemon); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(source, daemon string) error {
	if _, err := exec.LookPath("cliamp"); err != nil {
		return errors.New("cliamp is not on PATH; install cliamp first")
	}
	dir, err := cliampdir.Dir()
	if err != nil {
		return err
	}
	fmt.Println("cliamp config dir:", dir)

	// cliamp refuses to install over an existing plugin, so an update removes it first.
	pluginFile := filepath.Join(dir, "plugins", pluginName+".lua")
	if _, err := os.Stat(pluginFile); err == nil {
		fmt.Println("replacing existing plugin", pluginName)
		if err := runTool("cliamp", "plugins", "remove", pluginName); err != nil {
			return err
		}
	}

	fmt.Println("installing plugin", pluginName, "from", source)
	if err := runTool("cliamp", "plugins", "install", "--yes", source); err != nil {
		return err
	}

	fmt.Println("building daemon", daemon)
	if err := runTool("go", "install", daemon); err != nil {
		return err
	}
	if _, err := exec.LookPath(daemonName); err != nil {
		fmt.Println("warning:", daemonName, "is not on PATH; add the folder go install wrote to (GOBIN, or GOPATH/bin) to PATH")
	}

	configPath := filepath.Join(dir, "config.toml")
	changed, err := addAllowedBinary(configPath, daemonName)
	if err != nil {
		return err
	}
	if changed {
		fmt.Println("allowed_binaries now includes", daemonName, "in", configPath)
	} else {
		fmt.Println(daemonName, "already in allowed_binaries")
	}
	fmt.Println("done: restart cliamp to load the plugin")
	return nil
}

// addAllowedBinary adds name to [plugins] allowed_binaries in the config file
// at path. It creates the file if needed and keeps a backup of the previous
// content before writing. It reports whether the file changed.
func addAllowedBinary(path, name string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	out, changed, err := allowBinary(string(data), name)
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if !changed {
		return false, nil
	}
	if len(data) > 0 {
		if err := os.WriteFile(path+".bak", data, 0o600); err != nil {
			return false, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(out), 0o644)
}

func runTool(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}
