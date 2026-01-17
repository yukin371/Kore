package utils

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetProjectRoot finds the project root directory by looking for .git or go.mod
func GetProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if we've reached the filesystem root
		if dir == filepath.Dir(dir) {
			// No project root found, return current directory
			return dir, nil
		}

		// Check for common project markers
		if hasFile(dir, ".git") || hasFile(dir, "go.mod") {
			return dir, nil
		}

		// Move up one directory
		dir = filepath.Dir(dir)
	}
}

// hasFile checks if a file/directory exists in the given directory
func hasFile(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// IsDir checks if a path is a directory
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	if !IsDir(path) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// GetHomeDir returns the user's home directory
func GetHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home, nil
}

// GetConfigDir returns the platform-specific config directory
func GetConfigDir(appName string) (string, error) {
	home, err := GetHomeDir()
	if err != nil {
		return "", err
	}

	var configDir string
	switch runtime.GOOS {
	case "windows":
		configDir = filepath.Join(home, "AppData", "Roaming", appName)
	case "darwin":
		configDir = filepath.Join(home, "Library", "Application Support", appName)
	default: // linux, etc.
		// Check XDG_CONFIG_HOME first
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			configDir = filepath.Join(xdg, appName)
		} else {
			configDir = filepath.Join(home, ".config", appName)
		}
	}

	return configDir, nil
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
