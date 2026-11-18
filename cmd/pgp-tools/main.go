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

var appAssets = install.Assets{DesktopEntry: assets.DesktopEntry, IconSVG: assets.IconSVG}

func main() {
	if len(os.Args) > 1 {
		attachConsole()
	}
	var (
		doInstall   = flag.Bool("install", false, "install for the current user, no admin/root (Linux: app menu; Windows: Start Menu)")
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
		ui.Run(version, appAssets.IconSVG)
	}
}

func runInstall() error {
	if isRoot() {
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
	err = install.Install(p, exe, appAssets)
	switch {
	case errors.Is(err, install.ErrMenuEntry):
		fmt.Printf("installed without menu entry:\n  %s\n", p.Binary())
		return err
	case err != nil:
		return err
	}
	fmt.Printf("installed:\n  %s\n  %s\n", p.Binary(), p.MenuEntry())
	if !install.InPath(p.BinDir) {
		fmt.Printf("note: %s is not in PATH; start it from the menu entry\n", p.BinDir)
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
	fmt.Println("uninstalled; app settings remain in the Fyne preferences folder")
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
