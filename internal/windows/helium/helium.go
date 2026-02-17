package helium

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/addidotlol/yucca/internal/githubapi"
	"github.com/addidotlol/yucca/internal/state"
	"github.com/addidotlol/yucca/internal/version"
	"github.com/addidotlol/yucca/internal/windows/shortcut"
)

type InstallOptions struct {
	DesktopShortcut bool
	Force           bool
	Verbose         bool
}

type UpdateOptions struct {
	CheckOnly bool
	Force     bool
	Verbose   bool
}

type UninstallOptions struct {
	PurgeConfig bool
}

type Status struct {
	Installed        bool      `json:"installed"`
	InstalledVersion string    `json:"installed_version"`
	InstallPath      string    `json:"install_path"`
	LatestVersion    string    `json:"latest_version"`
	UpdateAvailable  bool      `json:"update_available"`
	LastChecked      time.Time `json:"last_checked"`
}

func Install(ctx context.Context, opts InstallOptions) (Status, error) {
	if err := ensureWindows(); err != nil {
		return Status{}, err
	}

	arch := windowsArch()
	logf(opts.Verbose, "Checking latest Helium release for %s...", arch)
	rel, err := githubapi.LatestRelease(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("fetch latest release: %w", err)
	}

	installerAsset, err := githubapi.SelectInstallerAssetForArch(rel, arch)
	if err != nil {
		return Status{}, err
	}
	portableAsset, _ := githubapi.SelectPortableAssetForArch(rel, arch)

	st, _ := state.Load()
	if !opts.Force {
		if _, ok := detectInstallPath(st.InstallPath); ok {
			if version.Compare(st.InstalledVersion, rel.TagName) >= 0 {
				logf(opts.Verbose, "Helium is already up to date.")
				return currentStatus(rel.TagName)
			}
		}
	}

	logf(opts.Verbose, "Using installer asset: %s", installerAsset.Name)
	installerPath, cleanupInstaller, err := downloadAsset(ctx, installerAsset, opts.Verbose)
	if err != nil {
		return Status{}, err
	}
	defer cleanupInstaller()

	logf(opts.Verbose, "Running installer...")
	installErr := runInstaller(ctx, installerPath)
	if installErr != nil {
		if portableAsset.Name == "" {
			return Status{}, installErr
		}
		logf(opts.Verbose, "Installer failed (%v). Falling back to portable asset: %s", installErr, portableAsset.Name)
		portablePath, cleanupPortable, err := downloadAsset(ctx, portableAsset, opts.Verbose)
		if err != nil {
			return Status{}, fmt.Errorf("installer failed and portable fallback download failed: %w", err)
		}
		defer cleanupPortable()

		portableExePath, err := installPortable(portablePath, opts.Verbose)
		if err != nil {
			return Status{}, fmt.Errorf("installer failed and portable fallback failed: %w", err)
		}
		st.InstallPath = portableExePath
	}

	installPath, ok := waitForInstallPath(st.InstallPath, 45*time.Second)
	if !ok {
		return Status{}, errors.New("installation completed but helium executable not found")
	}

	logf(opts.Verbose, "Creating Start Menu shortcut...")
	if err := shortcut.CreateStartMenuShortcut(installPath); err != nil {
		return Status{}, fmt.Errorf("create start menu shortcut: %w", err)
	}
	if opts.DesktopShortcut {
		logf(opts.Verbose, "Creating Desktop shortcut...")
		if err := shortcut.CreateDesktopShortcut(installPath); err != nil {
			return Status{}, fmt.Errorf("create desktop shortcut: %w", err)
		}
	}

	newState := state.State{
		InstalledVersion: version.Normalize(rel.TagName),
		InstallPath:      installPath,
		LastChecked:      time.Now(),
	}
	if err := state.Save(newState); err != nil {
		return Status{}, fmt.Errorf("save state: %w", err)
	}

	logf(opts.Verbose, "Install complete.")
	return currentStatus(rel.TagName)
}

func Update(ctx context.Context, opts UpdateOptions) (Status, error) {
	if err := ensureWindows(); err != nil {
		return Status{}, err
	}

	cur, err := currentStatus("")
	if err != nil {
		return Status{}, err
	}
	if !cur.Installed && !opts.Force {
		return cur, errors.New("helium is not installed; run `yucca install`")
	}

	if opts.CheckOnly {
		return cur, nil
	}

	if !cur.UpdateAvailable && !opts.Force {
		logf(opts.Verbose, "Helium is already up to date.")
		return cur, nil
	}

	return Install(ctx, InstallOptions{Force: true, Verbose: opts.Verbose})
}

func Uninstall(ctx context.Context, opts UninstallOptions) (bool, error) {
	if err := ensureWindows(); err != nil {
		return false, err
	}

	st, _ := state.Load()
	installPath, ok := detectInstallPath(st.InstallPath)
	if !ok {
		_ = shortcut.RemoveStartMenuShortcut()
		_ = shortcut.RemoveDesktopShortcut()
		if opts.PurgeConfig {
			_ = state.Delete()
		}
		return false, nil
	}

	uninstallerCandidates := []string{
		filepath.Join(filepath.Dir(installPath), "uninstall.exe"),
		filepath.Join(filepath.Dir(installPath), "unins000.exe"),
		filepath.Join(filepath.Dir(installPath), "uninst.exe"),
	}

	runErr := error(nil)
	hadUninstaller := false
	for _, cand := range uninstallerCandidates {
		if _, err := os.Stat(cand); err != nil {
			continue
		}
		hadUninstaller = true
		cmd := exec.CommandContext(ctx, cand, "/S")
		if out, err := cmd.CombinedOutput(); err != nil {
			runErr = fmt.Errorf("uninstaller failed: %w (%s)", err, strings.TrimSpace(string(out)))
			cmd2 := exec.CommandContext(ctx, cand)
			if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
				runErr = fmt.Errorf("uninstaller failed: %w (%s)", err2, strings.TrimSpace(string(out2)))
			} else {
				runErr = nil
			}
		}
		break
	}

	if !hadUninstaller {
		_ = os.RemoveAll(filepath.Dir(installPath))
	}

	if runErr != nil {
		return true, runErr
	}

	_ = shortcut.RemoveStartMenuShortcut()
	_ = shortcut.RemoveDesktopShortcut()

	if opts.PurgeConfig {
		_ = state.Delete()
	} else {
		_ = state.Save(state.State{})
	}

	return true, nil
}

func CurrentStatus(ctx context.Context) (Status, error) {
	if err := ensureWindows(); err != nil {
		return Status{}, err
	}
	_ = ctx
	return currentStatus("")
}

func currentStatus(latestHint string) (Status, error) {
	st, _ := state.Load()
	installPath, installed := detectInstallPath(st.InstallPath)

	latestVersion := latestHint
	if latestVersion == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		rel, err := githubapi.LatestRelease(ctx)
		if err == nil {
			latestVersion = rel.TagName
		}
	}

	installedVersion := st.InstalledVersion
	if installedVersion == "" {
		installedVersion = "unknown"
	}
	if !installed {
		installedVersion = ""
		installPath = ""
	}

	res := Status{
		Installed:        installed,
		InstalledVersion: version.Normalize(installedVersion),
		InstallPath:      installPath,
		LatestVersion:    version.Normalize(latestVersion),
		UpdateAvailable:  installed && latestVersion != "" && version.Compare(installedVersion, latestVersion) < 0,
		LastChecked:      st.LastChecked,
	}

	st.LastChecked = time.Now()
	_ = state.Save(st)

	return res, nil
}

func detectInstallPath(statePath string) (string, bool) {
	candidates := make([]string, 0, 12)
	if statePath != "" {
		candidates = append(candidates, statePath)
	}
	local := os.Getenv("LOCALAPPDATA")
	if local != "" {
		candidates = append(candidates,
			filepath.Join(local, "Programs", "Helium", "helium.exe"),
			filepath.Join(local, "Programs", "Helium", "chrome.exe"),
			filepath.Join(local, "Helium", "helium.exe"),
			filepath.Join(local, "Helium", "chrome.exe"),
			filepath.Join(local, "imput", "Helium", "Application", "helium.exe"),
			filepath.Join(local, "imput", "Helium", "Application", "chrome.exe"),
		)
	}
	pf := os.Getenv("ProgramFiles")
	if pf != "" {
		candidates = append(candidates,
			filepath.Join(pf, "Helium", "helium.exe"),
			filepath.Join(pf, "Helium", "chrome.exe"),
		)
	}
	pf86 := os.Getenv("ProgramFiles(x86)")
	if pf86 != "" {
		candidates = append(candidates,
			filepath.Join(pf86, "Helium", "helium.exe"),
			filepath.Join(pf86, "Helium", "chrome.exe"),
		)
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

func waitForInstallPath(statePath string, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	for {
		if p, ok := detectInstallPath(statePath); ok {
			return p, true
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return "", false
}

func downloadAsset(ctx context.Context, asset githubapi.Asset, verbose bool) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "yucca-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	path := filepath.Join(tmpDir, asset.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	req.Header.Set("User-Agent", "yucca-cli")

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("download asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cleanup()
		return "", nil, fmt.Errorf("download asset returned status %d", resp.StatusCode)
	}

	if verbose {
		fmt.Printf("Downloading %s\n", asset.Name)
	}

	f, err := os.Create(path)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	defer f.Close()

	pw := &progressWriter{total: resp.ContentLength, enabled: verbose}
	writer := io.MultiWriter(f, pw)
	if _, err := io.Copy(writer, resp.Body); err != nil {
		cleanup()
		return "", nil, err
	}
	pw.done()

	return path, cleanup, nil
}

func runInstaller(ctx context.Context, installerPath string) error {
	cmd := exec.CommandContext(ctx, installerPath, "/S")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("run installer failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func installPortable(zipPath string, verbose bool) (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", errors.New("LOCALAPPDATA is not set")
	}
	targetDir := filepath.Join(local, "Programs", "Helium")
	tmpDir, err := os.MkdirTemp("", "yucca-portable-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if verbose {
		fmt.Println("Extracting portable package...")
	}
	if err := unzip(zipPath, tmpDir); err != nil {
		return "", err
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}

	if verbose {
		fmt.Println("Installing portable files...")
	}
	if err := copyDir(tmpDir, targetDir); err != nil {
		return "", err
	}

	exe, ok := findHeliumExe(targetDir)
	if !ok {
		return "", errors.New("portable install completed but helium.exe was not found")
	}

	return exe, nil
}

func unzip(src, dst string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		name := filepath.Clean(f.Name)
		outPath := filepath.Join(dst, name)
		if !strings.HasPrefix(outPath, filepath.Clean(dst)+string(os.PathSeparator)) && filepath.Clean(outPath) != filepath.Clean(dst) {
			return fmt.Errorf("invalid zip entry path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		wf, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, copyErr := io.Copy(wf, rc)
		closeErr := wf.Close()
		rcErr := rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if rcErr != nil {
			return rcErr
		}
	}

	return nil
}

func findHeliumExe(root string) (string, bool) {
	var found string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), "helium.exe") {
			found = path
			return io.EOF
		}
		return nil
	})
	if errors.Is(err, io.EOF) && found != "" {
		return found, true
	}
	if err != nil {
		return "", false
	}
	return "", false
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		rf, err := os.Open(path)
		if err != nil {
			return err
		}

		wf, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			rf.Close()
			return err
		}

		_, copyErr := io.Copy(wf, rf)
		closeWErr := wf.Close()
		closeRErr := rf.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeWErr != nil {
			return closeWErr
		}
		if closeRErr != nil {
			return closeRErr
		}

		if _, err := os.Stat(target); err != nil {
			return err
		}
		return nil
	})
}

type progressWriter struct {
	total      int64
	downloaded int64
	lastPrint  time.Time
	enabled    bool
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	if !p.enabled {
		p.downloaded += int64(n)
		return n, nil
	}

	p.downloaded += int64(n)
	now := time.Now()
	if p.lastPrint.IsZero() || now.Sub(p.lastPrint) >= 120*time.Millisecond {
		p.print(false)
		p.lastPrint = now
	}

	return n, nil
}

func (p *progressWriter) done() {
	if !p.enabled {
		return
	}
	p.print(true)
}

func (p *progressWriter) print(final bool) {
	if p.total > 0 {
		pct := float64(p.downloaded) / float64(p.total) * 100
		fmt.Printf("\r  %6.2f%%  %.1f/%.1f MB", pct, bytesToMB(p.downloaded), bytesToMB(p.total))
	} else {
		fmt.Printf("\r  %.1f MB", bytesToMB(p.downloaded))
	}
	if final {
		fmt.Print("\n")
	}
}

func bytesToMB(n int64) float64 {
	return float64(n) / (1024 * 1024)
}

func windowsArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return strings.ToLower(runtime.GOARCH)
	}
}

func logf(verbose bool, format string, args ...any) {
	if !verbose {
		return
	}
	fmt.Printf(format+"\n", args...)
}

func ensureWindows() error {
	if os.PathSeparator != '\\' {
		return errors.New("yucca currently supports Windows only")
	}
	return nil
}
