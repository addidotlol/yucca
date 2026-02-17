package shortcut

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	shortcutName = "Helium.lnk"
	appName      = "Helium"
)

func StartMenuShortcutPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA is not set")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", appName, shortcutName), nil
}

func DesktopShortcutPath() (string, error) {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		return "", fmt.Errorf("USERPROFILE is not set")
	}
	return filepath.Join(profile, "Desktop", shortcutName), nil
}

func CreateStartMenuShortcut(targetPath string) error {
	p, err := StartMenuShortcutPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return createShortcut(targetPath, p)
}

func CreateDesktopShortcut(targetPath string) error {
	p, err := DesktopShortcutPath()
	if err != nil {
		return err
	}
	return createShortcut(targetPath, p)
}

func RemoveStartMenuShortcut() error {
	roots := startMenuRoots()
	candidates := make([]string, 0, 4)
	if p, err := StartMenuShortcutPath(); err == nil {
		candidates = append(candidates, p)
		candidates = append(candidates, filepath.Join(filepath.Dir(filepath.Dir(p)), shortcutName))
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		candidates = append(candidates,
			filepath.Join(root, appName, shortcutName),
			filepath.Join(root, shortcutName),
		)
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	for _, root := range roots {
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(d.Name()), ".lnk") {
				return nil
			}
			if strings.EqualFold(d.Name(), shortcutName) {
				_ = os.Remove(path)
			}
			return nil
		})
		_ = os.Remove(filepath.Join(root, appName))
	}

	return nil
}

func RemoveDesktopShortcut() error {
	p, err := DesktopShortcutPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func createShortcut(targetPath, shortcutPath string) error {
	targetPath = filepath.Clean(targetPath)
	shortcutPath = filepath.Clean(shortcutPath)
	workingDir := filepath.Dir(targetPath)

	escape := func(s string) string {
		return strings.ReplaceAll(s, "'", "''")
	}

	script := fmt.Sprintf(
		"$w=New-Object -ComObject WScript.Shell; $s=$w.CreateShortcut('%s'); $s.TargetPath='%s'; $s.WorkingDirectory='%s'; $s.IconLocation='%s,0'; $s.Save()",
		escape(shortcutPath), escape(targetPath), escape(workingDir), escape(targetPath),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create shortcut failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	return nil
}

func startMenuRoots() []string {
	roots := make([]string, 0, 2)
	if appData := os.Getenv("APPDATA"); appData != "" {
		roots = append(roots, filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	if pd := os.Getenv("ProgramData"); pd != "" {
		roots = append(roots, filepath.Join(pd, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	return roots
}
