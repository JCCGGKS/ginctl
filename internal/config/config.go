// Package config defines what can be configured and how it is loaded.
//
// Loading is handled by spf13/viper, which gives layered configuration with a
// clear precedence: defaults < config file < environment variables. Env keys
// have no prefix (e.g. SERVER_PORT=9090 overrides server.port).
package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

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

// Config is the root configuration tree. mapstructure tags are what
// viper.Unmarshal reads.
type Config struct {
	Server ServerConf `mapstructure:"server"`
	Auth   AuthConf   `mapstructure:"auth"`
	Log    LogConf    `mapstructure:"log"`
	MySQL  MySQLConf  `mapstructure:"mysql"`
	Redis  RedisConf  `mapstructure:"redis"`
}

type ServerConf struct {
	Host                     string `mapstructure:"host"`
	Port                     int    `mapstructure:"port"`
	Mode                     string `mapstructure:"mode"`
	ReadTimeoutSeconds       int    `mapstructure:"readTimeoutSeconds"`
	WriteTimeoutSeconds      int    `mapstructure:"writeTimeoutSeconds"`
	ReadHeaderTimeoutSeconds int    `mapstructure:"readHeaderTimeoutSeconds"`
	IdleTimeoutSeconds       int    `mapstructure:"idleTimeoutSeconds"`
	ShutdownTimeoutSeconds   int    `mapstructure:"shutdownTimeoutSeconds"`
}

type AuthConf struct {
	JWTSecret           string `mapstructure:"jwtSecret"`
	AccessExpireSeconds int    `mapstructure:"accessExpireSeconds"`
}

type LogConf struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// MySQLConf holds connection-pool settings for a MySQL instance. DSN builds the
// go-sql-driver/mysql connection string; callers should pass it to sql.Open or
// gorm's mysql dialector.
type MySQLConf struct {
	Host                   string `mapstructure:"host"`
	Port                   int    `mapstructure:"port"`
	User                   string `mapstructure:"user"`
	Password               string `mapstructure:"password"`
	Database               string `mapstructure:"database"`
	Charset                string `mapstructure:"charset"`
	MaxOpenConns           int    `mapstructure:"maxOpenConns"`
	MaxIdleConns           int    `mapstructure:"maxIdleConns"`
	ConnMaxLifetimeSeconds int    `mapstructure:"connMaxLifetimeSeconds"`
	ConnMaxIdleTimeSeconds int    `mapstructure:"connMaxIdleTimeSeconds"`
}

// RedisConf holds connection-pool settings for a Redis instance. Addr builds
// the host:port dial address used by redis clients.
type RedisConf struct {
	Host                string `mapstructure:"host"`
	Port                int    `mapstructure:"port"`
	Password            string `mapstructure:"password"`
	DB                  int    `mapstructure:"db"`
	PoolSize            int    `mapstructure:"poolSize"`
	MinIdleConns        int    `mapstructure:"minIdleConns"`
	DialTimeoutSeconds  int    `mapstructure:"dialTimeoutSeconds"`
	ReadTimeoutSeconds  int    `mapstructure:"readTimeoutSeconds"`
	WriteTimeoutSeconds int    `mapstructure:"writeTimeoutSeconds"`
	IdleTimeoutSeconds  int    `mapstructure:"idleTimeoutSeconds"`
}

// DSN returns a go-sql-driver/mysql DSN. ParseTime and Loc are fixed to scan
// DATETIME/TIMESTAMP into time.Time in the local timezone.
func (m MySQLConf) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local",
		m.User, m.Password, m.Host, m.Port, m.Database, m.Charset)
}

// Addr returns the host:port address for Redis clients.
func (r RedisConf) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// Load applies defaults, reads the YAML file at path, then overlays any
// matching environment variables.
func Load(path string) (Config, error) {
	v, err := newViper(path)
	if err != nil {
		return Config{}, err
	}
	return unmarshal(v)
}

func newViper(path string) (*viper.Viper, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// defaults < file < env. The replacer maps server.port -> SERVER_PORT.
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
	v.SetDefault("server.readHeaderTimeoutSeconds", 5)
	v.SetDefault("server.idleTimeoutSeconds", 60)
	v.SetDefault("server.shutdownTimeoutSeconds", 10)
	v.SetDefault("auth.jwtSecret", "")
	v.SetDefault("auth.accessExpireSeconds", 900)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("mysql.host", "127.0.0.1")
	v.SetDefault("mysql.port", 3306)
	v.SetDefault("mysql.user", "root")
	v.SetDefault("mysql.password", "")
	v.SetDefault("mysql.database", "gin_scaffold")
	v.SetDefault("mysql.charset", "utf8mb4")
	v.SetDefault("mysql.maxOpenConns", 100)
	v.SetDefault("mysql.maxIdleConns", 10)
	v.SetDefault("mysql.connMaxLifetimeSeconds", 300)
	v.SetDefault("mysql.connMaxIdleTimeSeconds", 60)
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.poolSize", 100)
	v.SetDefault("redis.minIdleConns", 10)
	v.SetDefault("redis.dialTimeoutSeconds", 5)
	v.SetDefault("redis.readTimeoutSeconds", 3)
	v.SetDefault("redis.writeTimeoutSeconds", 3)
	v.SetDefault("redis.idleTimeoutSeconds", 300)
}
