package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	InstalledVersion string    `json:"installed_version"`
	InstallPath      string    `json:"install_path"`
	LastChecked      time.Time `json:"last_checked"`
}

func filePath() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("LOCALAPPDATA is not set")
	}
	return filepath.Join(base, "Yucca", "state.json"), nil
}

func Load() (State, error) {
	var s State
	p, err := filePath()
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, err
	}
	return s, nil
}

func Save(s State) error {
	p, err := filePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func Delete() error {
	p, err := filePath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
