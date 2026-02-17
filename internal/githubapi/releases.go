package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	Owner = "imputnet"
	Repo  = "helium-windows"
)

type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func LatestRelease(ctx context.Context) (Release, error) {
	var rel Release
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", Owner, Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return rel, err
	}
	req.Header.Set("User-Agent", "yucca-cli")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return rel, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return rel, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return rel, err
	}

	return rel, nil
}

func SelectInstallerAsset(rel Release) (Asset, error) {
	return SelectInstallerAssetForArch(rel, "")
}

func SelectInstallerAssetForArch(rel Release, arch string) (Asset, error) {
	candidates := make([]Asset, 0)
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if !strings.HasSuffix(name, ".exe") {
			continue
		}
		if strings.Contains(name, "sig") || strings.Contains(name, "sha") || strings.Contains(name, "checksum") {
			continue
		}
		candidates = append(candidates, a)
	}

	if len(candidates) == 0 {
		return Asset{}, fmt.Errorf("no installer asset (.exe) found in latest release")
	}

	arch = strings.ToLower(strings.TrimSpace(arch))
	archNeeds := map[string][]string{
		"x64":   {"x64", "amd64", "64"},
		"arm64": {"arm64", "aarch64"},
	}
	archAvoid := map[string][]string{
		"x64":   {"arm64", "aarch64"},
		"arm64": {"x64", "amd64"},
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		score := func(name string) int {
			n := strings.ToLower(name)
			s := 0
			if strings.Contains(n, "installer") || strings.Contains(n, "setup") {
				s += 10
			}
			if strings.Contains(n, "x64") || strings.Contains(n, "amd64") {
				s += 2
			}
			if strings.Contains(n, "portable") || strings.Contains(n, "zip") {
				s -= 10
			}
			for _, want := range archNeeds[arch] {
				if strings.Contains(n, want) {
					s += 25
				}
			}
			for _, avoid := range archAvoid[arch] {
				if strings.Contains(n, avoid) {
					s -= 30
				}
			}
			return s
		}
		return score(candidates[i].Name) > score(candidates[j].Name)
	})

	return candidates[0], nil
}

func SelectPortableAssetForArch(rel Release, arch string) (Asset, error) {
	candidates := make([]Asset, 0)
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if !(strings.HasSuffix(name, ".zip") || strings.Contains(name, "portable")) {
			continue
		}
		if strings.Contains(name, "sig") || strings.Contains(name, "sha") || strings.Contains(name, "checksum") {
			continue
		}
		candidates = append(candidates, a)
	}

	if len(candidates) == 0 {
		return Asset{}, fmt.Errorf("no portable asset found in latest release")
	}

	arch = strings.ToLower(strings.TrimSpace(arch))
	archNeeds := map[string][]string{
		"x64":   {"x64", "amd64", "64"},
		"arm64": {"arm64", "aarch64"},
	}
	archAvoid := map[string][]string{
		"x64":   {"arm64", "aarch64"},
		"arm64": {"x64", "amd64"},
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		score := func(name string) int {
			n := strings.ToLower(name)
			s := 0
			if strings.Contains(n, ".zip") || strings.Contains(n, "windows") {
				s += 4
			}
			for _, want := range archNeeds[arch] {
				if strings.Contains(n, want) {
					s += 25
				}
			}
			for _, avoid := range archAvoid[arch] {
				if strings.Contains(n, avoid) {
					s -= 30
				}
			}
			return s
		}
		return score(candidates[i].Name) > score(candidates[j].Name)
	})

	return candidates[0], nil
}
