package configsource

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func NewViperForCommand(cmd *cobra.Command, configFlagValue string) (*viper.Viper, error) {
	v := viper.New()

	v.SetEnvPrefix("SOUTH2MD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if err := bindViperFlags(v, cmd); err != nil {
		return nil, err
	}

	configPath, explicit, err := resolveConfigFilePath(cmd, configFlagValue)
	if err != nil {
		return nil, err
	}
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok && !explicit {
				return v, nil
			}
			return nil, fmt.Errorf("读取配置文件失败 %q: %w", configPath, err)
		}
	}

	applyDerivedOverrides(v, cmd)
	return v, nil
}

func bindViperFlags(v *viper.Viper, cmd *cobra.Command) error {
	visited := make(map[string]struct{})
	var bindErr error
	bindFlag := func(f *pflag.Flag) {
		if f == nil || bindErr != nil {
			return
		}
		if _, ok := visited[f.Name]; ok {
			return
		}
		visited[f.Name] = struct{}{}
		configName := strings.ReplaceAll(f.Name, "-", "_")
		if err := v.BindPFlag(configName, f); err != nil {
			bindErr = fmt.Errorf("绑定 flag %q 到 key %q 失败: %w", f.Name, configName, err)
		}
	}

	cmd.Flags().VisitAll(bindFlag)
	cmd.InheritedFlags().VisitAll(bindFlag)
	if bindErr != nil {
		return bindErr
	}

	// Keep struct tag naming with existing --output flag.
	v.RegisterAlias("output_file", "output")
	return nil
}

func applyDerivedOverrides(v *viper.Viper, cmd *cobra.Command) {
	_, hasEnvNoCache := os.LookupEnv("SOUTH2MD_NO_CACHE")
	if flagChanged(cmd, "no-cache") || hasEnvNoCache || v.InConfig("no_cache") {
		v.Set("enable_cache", !v.GetBool("no_cache"))
	}
}

func resolveConfigFilePath(cmd *cobra.Command, configFlagValue string) (string, bool, error) {
	if flagChanged(cmd, "config") {
		path := strings.TrimSpace(configFlagValue)
		if path == "" {
			return "", true, errors.New("--config 不能为空")
		}
		return path, true, nil
	}

	if value := strings.TrimSpace(os.Getenv("SOUTH2MD_CONFIG")); value != "" {
		return value, true, nil
	}

	candidates := []string{
		filepath.Join(".", "south2md.toml"),
	}
	if userConfigDir, err := os.UserConfigDir(); err == nil && userConfigDir != "" {
		candidates = append(candidates, filepath.Join(userConfigDir, "south2md", "config.toml"))
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, false, nil
		}
	}

	return "", false, nil
}

func flagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	if f := cmd.Flags().Lookup(name); f != nil {
		return f.Changed
	}
	if f := cmd.InheritedFlags().Lookup(name); f != nil {
		return f.Changed
	}
	return false
}
