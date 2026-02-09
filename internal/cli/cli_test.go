package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fdkevin0/south2md"
	"github.com/spf13/pflag"
)

func resetCLIStateForTest(t *testing.T) {
	t.Helper()

	defaultConfig := south2mdDefaultConfigForTest()
	flagConfigFile = ""
	flagTID = ""
	flagInputFile = ""
	flagOutputFile = ""
	flagOffline = false
	flagCacheDir = defaultConfig.CacheDir
	flagBaseURL = defaultConfig.BaseURL
	flagCookieFile = defaultConfig.HTTPCookieFile
	flagNoCache = false
	flagTimeout = int(defaultConfig.HTTPTimeout.Seconds())
	flagMaxConcurrent = defaultConfig.HTTPMaxConcurrent
	flagDebug = false
	flagUserAgent = defaultConfig.HTTPUserAgent
	flagGofileEnable = defaultConfig.GofileEnable
	flagGofileTool = defaultConfig.GofileTool
	flagGofileDir = defaultConfig.GofileDir
	flagGofileToken = defaultConfig.GofileToken
	flagGofileVenvDir = defaultConfig.GofileVenvDir
	flagGofileSkipExisting = defaultConfig.GofileSkipExisting
	flagCookieImportFile = ""

	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		_ = f.Value.Set(f.DefValue)
	})
}

func south2mdDefaultConfigForTest() *south2md.Config {
	return south2md.NewDefaultConfig()
}

func TestBuildRuntimeConfigUsesPositionalTID(t *testing.T) {
	resetCLIStateForTest(t)

	cfg, err := buildRuntimeConfig(rootCmd, []string{"2636739"})
	if err != nil {
		t.Fatalf("buildRuntimeConfig returned error: %v", err)
	}

	if cfg.App.TID != "2636739" {
		t.Fatalf("expected positional tid, got %q", cfg.App.TID)
	}
}

func TestBuildRuntimeConfigFlagOverridesPositionalTID(t *testing.T) {
	resetCLIStateForTest(t)
	if err := rootCmd.PersistentFlags().Set("tid", "9999999"); err != nil {
		t.Fatalf("set tid flag: %v", err)
	}

	cfg, err := buildRuntimeConfig(rootCmd, []string{"2636739"})
	if err != nil {
		t.Fatalf("buildRuntimeConfig returned error: %v", err)
	}

	if cfg.App.TID != "9999999" {
		t.Fatalf("expected flag tid override, got %q", cfg.App.TID)
	}
}

func TestBuildRuntimeConfigEnvOverridesConfigFile(t *testing.T) {
	resetCLIStateForTest(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "south2md.toml")
	content := strings.Join([]string{
		"tid = \"1111111\"",
		"cache_dir = \"from-config\"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("SOUTH2MD_CONFIG", configPath)
	t.Setenv("SOUTH2MD_TID", "2222222")

	cfg, err := buildRuntimeConfig(rootCmd, nil)
	if err != nil {
		t.Fatalf("buildRuntimeConfig returned error: %v", err)
	}

	if cfg.App.TID != "2222222" {
		t.Fatalf("expected env tid override, got %q", cfg.App.TID)
	}
	if cfg.App.CacheDir != "from-config" {
		t.Fatalf("expected config file cache_dir, got %q", cfg.App.CacheDir)
	}
}

func TestBuildRuntimeConfigFlagOverridesEnv(t *testing.T) {
	resetCLIStateForTest(t)
	t.Setenv("SOUTH2MD_TID", "2222222")

	if err := rootCmd.PersistentFlags().Set("tid", "3333333"); err != nil {
		t.Fatalf("set tid flag: %v", err)
	}

	cfg, err := buildRuntimeConfig(rootCmd, nil)
	if err != nil {
		t.Fatalf("buildRuntimeConfig returned error: %v", err)
	}

	if cfg.App.TID != "3333333" {
		t.Fatalf("expected flag tid override, got %q", cfg.App.TID)
	}
}

func TestBuildRuntimeConfigRejectsOfflineWithInput(t *testing.T) {
	resetCLIStateForTest(t)
	if err := rootCmd.PersistentFlags().Set("offline", "true"); err != nil {
		t.Fatalf("set offline flag: %v", err)
	}
	if err := rootCmd.PersistentFlags().Set("input", "xx.html"); err != nil {
		t.Fatalf("set input flag: %v", err)
	}
	if err := rootCmd.PersistentFlags().Set("tid", "2636739"); err != nil {
		t.Fatalf("set tid flag: %v", err)
	}

	_, err := buildRuntimeConfig(rootCmd, nil)
	if err == nil {
		t.Fatal("expected offline/input conflict error")
	}
	if !strings.Contains(err.Error(), "--offline 模式下不支持 --input") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRuntimeConfigMissingExplicitConfigFile(t *testing.T) {
	resetCLIStateForTest(t)
	t.Setenv("SOUTH2MD_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	_, err := buildRuntimeConfig(rootCmd, []string{"2636739"})
	if err == nil {
		t.Fatal("expected error for explicit missing config file")
	}
	if !strings.Contains(err.Error(), "读取配置文件失败") {
		t.Fatalf("unexpected error: %v", err)
	}
}
