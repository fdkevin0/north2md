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

// DefaultCacheDir returns XDG cache home or a platform fallback.
func DefaultCacheDir(app string) string {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".")
		}
		cacheHome = filepath.Join(home, ".cache")
	}
	if app == "" {
		return cacheHome
	}
	return filepath.Join(cacheHome, app)
}

// DefaultCookieFile returns the default cookie file path in data dir.
func DefaultCookieFile(app string) string {
	return filepath.Join(DefaultDataDir(app), "cookies.txt")
}

// DefaultGofileToolPath returns the default gofile downloader path in data dir.
func DefaultGofileToolPath(app string) string {
	return filepath.Join(DefaultDataDir(app), "gofile-downloader", "gofile-downloader.py")
}

// DefaultGofileVenvDir returns the default gofile venv dir in data dir.
func DefaultGofileVenvDir(app string) string {
	return filepath.Join(DefaultDataDir(app), "py", "gofile")
}
