package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultFrameworks is the curated list of commonly used frameworks.
var DefaultFrameworks = []string{
	"Swift", "Foundation", "SwiftUI", "SwiftUICore", "UIKit", "AppKit",
	"Combine", "Observation", "SwiftData", "CoreData",
	"CoreGraphics", "CoreImage", "CoreText", "CoreLocation",
	"MapKit", "AVFoundation", "AVKit",
	"StoreKit", "CloudKit", "AuthenticationServices",
	"Network", "WebKit",
	"CoreML", "Vision", "NaturalLanguage",
	"UserNotifications", "AppIntents", "WidgetKit",
	"Metal", "SceneKit", "SpriteKit", "ARKit",
}

// Config holds doq configuration.
type Config struct {
	SDKPath    string   `json:"sdk_path"`
	Target     string   `json:"target"`
	Frameworks []string `json:"frameworks"`
	MaxResults int      `json:"max_results"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "doq")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

// IndexPath returns the path to the SQLite index.
func IndexPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "doq", "index.db")
}

// Load reads the config file, returning defaults if it doesn't exist.
func Load() (*Config, error) {
	cfg := &Config{
		Frameworks: DefaultFrameworks,
		MaxResults: 20,
	}

	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("corrupt config: %w", err)
	}
	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}

// DetectSDK runs xcrun to find the macOS SDK path.
func DetectSDK() (string, error) {
	out, err := exec.Command("xcrun", "--show-sdk-path", "--sdk", "macosx").Output()
	if err != nil {
		return "", fmt.Errorf("xcrun --show-sdk-path failed: %w — is Xcode installed?", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DetectTarget returns the symbolgraph extraction target triple.
func DetectTarget() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	// Get macOS version for target
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return arch + "-apple-macos15.0"
	}
	ver := strings.TrimSpace(string(out))
	parts := strings.Split(ver, ".")
	if len(parts) >= 2 {
		ver = parts[0] + "." + parts[1]
	} else {
		ver = parts[0] + ".0"
	}
	return arch + "-apple-macos" + ver
}

// ListAllFrameworks returns all available framework module names from the SDK.
func ListAllFrameworks(sdkPath string) ([]string, error) {
	frameworksDir := filepath.Join(sdkPath, "System", "Library", "Frameworks")
	entries, err := os.ReadDir(frameworksDir)
	if err != nil {
		return nil, fmt.Errorf("reading frameworks dir: %w", err)
	}

	var modules []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".framework") {
			mod := strings.TrimSuffix(name, ".framework")
			modules = append(modules, mod)
		}
	}
	return modules, nil
}
