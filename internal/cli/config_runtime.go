package cli

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/fdkevin0/south2md"
	"github.com/fdkevin0/south2md/internal/configsource"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type runtimeConfig struct {
	App        *south2md.Config
	InputFile  string
	Offline    bool
	Debug      bool
	ConfigFile string
}

type runtimeConfigValues struct {
	south2md.Config `mapstructure:",squash"`
	InputFile       string `mapstructure:"input"`
	Offline         bool   `mapstructure:"offline"`
	Debug           bool   `mapstructure:"debug"`
}

func buildRuntimeConfig(cmd *cobra.Command, args []string) (*runtimeConfig, error) {
	v, err := configsource.NewViperForCommand(cmd, flagConfigFile)
	if err != nil {
		return nil, err
	}

	values := runtimeConfigValues{
		Config: *south2md.NewDefaultConfig(),
	}
	if err := v.Unmarshal(&values, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		durationDecodeHook(),
		mapstructure.StringToTimeDurationHookFunc(),
	))); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	values.TID = strings.TrimSpace(values.TID)
	values.InputFile = strings.TrimSpace(values.InputFile)
	values.OutputFile = strings.TrimSpace(values.OutputFile)
	values.CacheDir = strings.TrimSpace(values.CacheDir)
	values.BaseURL = strings.TrimSpace(values.BaseURL)
	values.HTTPCookieFile = strings.TrimSpace(values.HTTPCookieFile)
	values.HTTPUserAgent = strings.TrimSpace(values.HTTPUserAgent)
	values.GofileTool = strings.TrimSpace(values.GofileTool)
	values.GofileDir = strings.TrimSpace(values.GofileDir)
	values.GofileToken = strings.TrimSpace(values.GofileToken)
	values.GofileVenvDir = strings.TrimSpace(values.GofileVenvDir)

	if values.TID == "" && len(args) > 0 {
		values.TID = args[0]
	}

	cfg := &runtimeConfig{
		App:        &values.Config,
		InputFile:  values.InputFile,
		Offline:    values.Offline,
		Debug:      values.Debug,
		ConfigFile: v.ConfigFileUsed(),
	}

	if err := validateRuntimeConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validateRuntimeConfig(cfg *runtimeConfig) error {
	if cfg.Offline && cfg.InputFile != "" {
		return fmt.Errorf("--offline 模式下不支持 --input")
	}
	if cfg.Offline && cfg.App.TID == "" {
		return fmt.Errorf("--offline 模式必须指定帖子ID")
	}
	if cfg.App.HTTPTimeout <= 0 {
		return fmt.Errorf("timeout 必须大于 0")
	}
	if cfg.App.HTTPMaxConcurrent <= 0 {
		return fmt.Errorf("max-concurrent 必须大于 0")
	}
	if !cfg.Offline && cfg.App.TID == "" && cfg.InputFile == "" {
		return fmt.Errorf("必须指定帖子ID或 --input 参数")
	}
	return nil
}

func durationDecodeHook() mapstructure.DecodeHookFuncType {
	durationType := reflect.TypeOf(time.Duration(0))
	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		if to != durationType {
			return data, nil
		}

		switch value := data.(type) {
		case int:
			return time.Duration(value) * time.Second, nil
		case int64:
			return time.Duration(value) * time.Second, nil
		case float64:
			return time.Duration(value) * time.Second, nil
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return time.Duration(0), nil
			}
			if strings.ContainsAny(trimmed, "hmsuµns") {
				parsed, err := time.ParseDuration(trimmed)
				if err != nil {
					return nil, err
				}
				return parsed, nil
			}
			return time.ParseDuration(trimmed + "s")
		default:
			return data, nil
		}
	}
}
