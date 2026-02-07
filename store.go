package south2md

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// PostStore manages local persistence in user data directory.
type PostStore struct {
	rootDir string
}

// NewPostStore creates a post store under the given root directory.
func NewPostStore(rootDir string) *PostStore {
	return &PostStore{rootDir: rootDir}
}

// RootDir returns the root directory of the store.
func (ps *PostStore) RootDir() string {
	return ps.rootDir
}

// EnsureRoot creates the root directory if missing.
func (ps *PostStore) EnsureRoot() error {
	if ps == nil {
		return fmt.Errorf("post store is nil")
	}
	if ps.rootDir == "" {
		return fmt.Errorf("post store root dir is empty")
	}
	return os.MkdirAll(ps.rootDir, 0755)
}

// PostDir returns the directory path for one thread id.
func (ps *PostStore) PostDir(tid string) string {
	return filepath.Join(ps.rootDir, tid)
}

// LoadPostFromStore loads metadata.toml from local store by tid.
func (ps *PostStore) LoadPostFromStore(tid string) (*Post, error) {
	if ps == nil {
		return nil, fmt.Errorf("post store is nil")
	}
	if tid == "" {
		return nil, fmt.Errorf("tid is empty")
	}
	metadataPath := filepath.Join(ps.PostDir(tid), "metadata.toml")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata from store: %w", err)
	}

	var post Post
	if err := toml.Unmarshal(data, &post); err != nil {
		return nil, fmt.Errorf("failed to decode metadata from store: %w", err)
	}
	return &post, nil
}

// ExportPost exports one stored post directory to target directory.
func (ps *PostStore) ExportPost(tid string, targetDir string) (string, error) {
	if ps == nil {
		return "", fmt.Errorf("post store is nil")
	}
	if tid == "" {
		return "", fmt.Errorf("tid is empty")
	}
	if targetDir == "" {
		return "", fmt.Errorf("target dir is empty")
	}

	srcDir := ps.PostDir(tid)
	if _, err := os.Stat(srcDir); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("post %s not found in local store", tid)
		}
		return "", fmt.Errorf("failed to stat source dir: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create target dir: %w", err)
	}
	dstDir := filepath.Join(targetDir, tid)
	if err := copyDir(srcDir, dstDir); err != nil {
		return "", err
	}
	return dstDir, nil
}

func copyDir(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination root: %w", err)
	}

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to build relative path: %w", err)
		}
		if rel == "." {
			return nil
		}

		dstPath := filepath.Join(dstDir, rel)
		if d.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return fmt.Errorf("failed to create destination dir: %w", err)
			}
			return nil
		}
		return copyFile(path, dstPath)
	})
}

func copyFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := os.Chmod(dstPath, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set destination file mode: %w", err)
	}

	return nil
}
