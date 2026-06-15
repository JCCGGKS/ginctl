// Package config defines what can be configured and how it is loaded.
//
// Loading is handled by spf13/viper, which gives layered configuration with a
// clear precedence: defaults < config file < environment variables. Env keys
// use the APP_ prefix (e.g. APP_SERVER_PORT=9090 overrides server.port).
// Optional hot-reload is available via Watch. The Load/MustLoad API is
// unchanged, so the rest of the app does not know viper sits underneath.
package config

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// envPrefix namespaces environment overrides so they never collide with
// unrelated host env vars.
const envPrefix = "APP"

// DefaultEnv is the environment assumed when the caller supplies none.
const DefaultEnv = "dev"

// FileForEnv builds the conventional per-environment config path, e.g.
// FileForEnv("etc", "dev") -> "etc/config_dev.yaml". An empty env falls back
// to DefaultEnv so a flag value can be passed straight through.
func FileForEnv(dir, env string) string {
	if env == "" {
		env = DefaultEnv
	}
	return filepath.Join(dir, "config_"+env+".yaml")
}

// Config is the root configuration tree. Add sub-configs (MySQL, Redis, ...)
// here as the project grows; mapstructure tags are what viper.Unmarshal reads.
type Config struct {
	Server ServerConf `mapstructure:"server"`
	Auth   AuthConf   `mapstructure:"auth"`
	Log    LogConf    `mapstructure:"log"`
}

type ServerConf struct {
	Host                   string `mapstructure:"host"`
	Port                   int    `mapstructure:"port"`
	Mode                   string `mapstructure:"mode"`
	ReadTimeoutSeconds     int    `mapstructure:"readTimeoutSeconds"`
	WriteTimeoutSeconds    int    `mapstructure:"writeTimeoutSeconds"`
	ShutdownTimeoutSeconds int    `mapstructure:"shutdownTimeoutSeconds"`
}

type AuthConf struct {
	JWTSecret           string `mapstructure:"jwtSecret"`
	AccessExpireSeconds int    `mapstructure:"accessExpireSeconds"`
}

type LogConf struct {
	Level string `mapstructure:"level"`
}

// MustLoad loads the config or panics. Use at startup only.
func MustLoad(path string) Config {
	c, err := Load(path)
	if err != nil {
		panic(err)
	}
	return c
}

// Load applies defaults, reads the YAML file at path, then overlays any
// matching APP_* environment variables.
func Load(path string) (Config, error) {
	v, err := newViper(path)
	if err != nil {
		return Config{}, err
	}
	return unmarshal(v)
}

// Watch hot-reloads the file and calls onChange with the fresh Config on every
// edit. Opt-in: call it from main if you want live config. Note that values
// already copied by value (e.g. into ServiceContext) keep their old value
// unless you re-read them inside onChange.
func Watch(path string, onChange func(Config)) error {
	v, err := newViper(path)
	if err != nil {
		return err
	}
	v.OnConfigChange(func(e fsnotify.Event) {
		c, err := unmarshal(v)
		if err != nil {
			slog.Error("reload config failed", "file", e.Name, "error", err)
			return
		}
		onChange(c)
	})
	v.WatchConfig()
	return nil
}

func newViper(path string) (*viper.Viper, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// defaults < file < env. The replacer maps server.port -> APP_SERVER_PORT.
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	return v, nil
}

func unmarshal(v *viper.Viper) (Config, error) {
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	return c, nil
}

// setDefaults registers every key. This both lets a sparse file still boot and
// lets viper's AutomaticEnv bind env overrides for keys absent from the file
// (Unmarshal only visits keys viper already knows about).
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.readTimeoutSeconds", 15)
	v.SetDefault("server.writeTimeoutSeconds", 15)
	v.SetDefault("server.shutdownTimeoutSeconds", 10)
	v.SetDefault("auth.jwtSecret", "")
	v.SetDefault("auth.accessExpireSeconds", 900)
	v.SetDefault("log.level", "info")
}
