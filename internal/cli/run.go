package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/addidotlol/yucca/internal/windows/helium"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "install":
		return runInstall(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "status":
		return runStatus(args[1:])
	case "uninstall":
		return runUninstall(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	desktopShortcut := fs.Bool("desktop-shortcut", false, "create desktop shortcut")
	force := fs.Bool("force", false, "reinstall even if already up to date")
	jsonOut := fs.Bool("json", false, "print JSON output")
	quiet := fs.Bool("quiet", false, "suppress detailed progress output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := helium.Install(context.Background(), helium.InstallOptions{
		DesktopShortcut: *desktopShortcut,
		Force:           *force,
		Verbose:         !*jsonOut && !*quiet,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(st)
	}

	fmt.Println("Helium installed.")
	if st.InstallPath != "" {
		fmt.Println("Path:", st.InstallPath)
	}
	if st.InstalledVersion != "" {
		fmt.Println("Version:", st.InstalledVersion)
	}
	return nil
}

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	checkOnly := fs.Bool("check-only", false, "check for updates but do not install")
	force := fs.Bool("force", false, "force running update flow")
	jsonOut := fs.Bool("json", false, "print JSON output")
	quiet := fs.Bool("quiet", false, "suppress detailed progress output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := helium.Update(context.Background(), helium.UpdateOptions{
		CheckOnly: *checkOnly,
		Force:     *force,
		Verbose:   !*jsonOut && !*quiet,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(st)
	}

	if *checkOnly {
		if st.UpdateAvailable {
			fmt.Printf("Update available: %s -> %s\n", st.InstalledVersion, st.LatestVersion)
		} else {
			fmt.Println("No updates available.")
		}
		return nil
	}

	if st.InstalledVersion != "" {
		fmt.Printf("Helium version: %s\n", st.InstalledVersion)
	}
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := helium.CurrentStatus(context.Background())
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(st)
	}

	if !st.Installed {
		fmt.Println("Helium is not installed.")
		if st.LatestVersion != "" {
			fmt.Println("Latest:", st.LatestVersion)
		}
		return nil
	}

	fmt.Println("Helium is installed.")
	fmt.Println("Version:", st.InstalledVersion)
	fmt.Println("Path:", st.InstallPath)
	if st.LatestVersion != "" {
		fmt.Println("Latest:", st.LatestVersion)
	}
	if st.UpdateAvailable {
		fmt.Println("Update available: yes")
	} else {
		fmt.Println("Update available: no")
	}
	return nil
}

func runUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	purgeConfig := fs.Bool("purge-config", false, "remove Yucca state file")
	jsonOut := fs.Bool("json", false, "print JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	uninstalled, err := helium.Uninstall(context.Background(), helium.UninstallOptions{
		PurgeConfig: *purgeConfig,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(map[string]bool{"uninstalled": uninstalled})
	}

	if uninstalled {
		fmt.Println("Helium uninstalled.")
	} else {
		fmt.Println("Helium was not detected; shortcuts/state cleaned where possible.")
	}
	return nil
}

func printUsage() {
	usage := []string{
		"yucca - Helium installer/updater for Windows",
		"",
		"Usage:",
		"  yucca <command> [flags]",
		"",
		"Commands:",
		"  install    Install Helium and add Start Menu shortcut",
		"  update     Update Helium if a newer release exists",
		"  status     Show installed and latest versions",
		"  uninstall  Uninstall Helium and remove shortcuts",
		"",
		"Examples:",
		"  yucca install --desktop-shortcut",
		"  yucca update --check-only",
		"  yucca install --quiet",
		"  yucca status",
		"  yucca uninstall --purge-config",
	}
	fmt.Println(strings.Join(usage, "\n"))
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
