package south2md

import (
	"os"
	"path/filepath"
)

// DefaultDataDir returns XDG data home or a platform fallback.
func DefaultDataDir(app string) string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".")
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	if app == "" {
		return dataHome
	}
	return filepath.Join(dataHome, app)
}
