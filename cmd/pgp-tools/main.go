package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/yoyoucay/pgp-tools/assets"
	"github.com/yoyoucay/pgp-tools/internal/install"
	"github.com/yoyoucay/pgp-tools/internal/ui"
)

// version is overridden at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	var (
		doInstall   = flag.Bool("install", false, "install for current user (no root): ~/.local/bin + app menu entry")
		doUninstall = flag.Bool("uninstall", false, "remove the per-user install")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	switch {
	case *showVersion:
		fmt.Println(version)
	case *doInstall:
		exitOn(runInstall())
	case *doUninstall:
		exitOn(runUninstall())
	default:
		ui.Run(version, assets.IconSVG)
	}
}

func runInstall() error {
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "warning: running as root installs into root's home, not system-wide; run as your normal user")
	}
	p, err := install.UserPaths()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate running binary: %w", err)
	}
	if err := install.Install(p, exe, assets.DesktopEntry, assets.IconSVG); err != nil {
		return err
	}
	fmt.Printf("installed:\n  %s\n  %s\n  %s\n", p.Binary(), p.Desktop(), p.Icon())
	if !install.InPath(p.BinDir) {
		fmt.Printf("note: %s is not in PATH; the app menu entry still works\n", p.BinDir)
	}
	return nil
}

func runUninstall() error {
	p, err := install.UserPaths()
	if err != nil {
		return err
	}
	if err := install.Uninstall(p); err != nil {
		return err
	}
	fmt.Println("uninstalled; settings remain in the Fyne preferences directory")
	return nil
}

func exitOn(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	if errors.Is(err, install.ErrUnsupportedOS) {
		os.Exit(2)
	}
	os.Exit(1)
}
