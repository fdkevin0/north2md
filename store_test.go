package south2md_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	main "github.com/fdkevin0/south2md"
)

func TestPostStoreLoadAndExport(t *testing.T) {
	tmpDir := t.TempDir()
	storeRoot := filepath.Join(tmpDir, "store")
	store := main.NewPostStore(storeRoot)
	if err := store.EnsureRoot(); err != nil {
		t.Fatalf("ensure root: %v", err)
	}

	post := &main.Post{TID: "2636739", Title: "hello"}
	postDir := filepath.Join(storeRoot, post.TID)
	if err := os.MkdirAll(postDir, 0755); err != nil {
		t.Fatalf("mkdir post dir: %v", err)
	}
	metadata, err := toml.Marshal(post)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "metadata.toml"), metadata, 0644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "post.md"), []byte("# post"), 0644); err != nil {
		t.Fatalf("write post: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(postDir, "images"), 0755); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "images", "a.txt"), []byte("img"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	loaded, err := store.LoadPostFromStore(post.TID)
	if err != nil {
		t.Fatalf("load post: %v", err)
	}
	if loaded.TID != post.TID || loaded.Title != post.Title {
		t.Fatalf("unexpected loaded post: %+v", loaded)
	}

	exportRoot := filepath.Join(tmpDir, "exports")
	exportedDir, err := store.ExportPost(post.TID, exportRoot)
	if err != nil {
		t.Fatalf("export post: %v", err)
	}
	if exportedDir != filepath.Join(exportRoot, post.TID) {
		t.Fatalf("unexpected export dir: %s", exportedDir)
	}

	if _, err := os.Stat(filepath.Join(exportedDir, "post.md")); err != nil {
		t.Fatalf("exported post missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportedDir, "images", "a.txt")); err != nil {
		t.Fatalf("exported image missing: %v", err)
	}
}

func TestPostStoreExportMissingPost(t *testing.T) {
	store := main.NewPostStore(t.TempDir())
	if _, err := store.ExportPost("missing", t.TempDir()); err == nil {
		t.Fatal("expected error for missing post")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
