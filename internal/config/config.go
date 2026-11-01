// Package config defines what can be configured and how it is loaded.
//
// Loading is handled by spf13/viper, which gives layered configuration with a
// clear precedence: defaults < config file < environment variables. Env keys
// have no prefix (e.g. SERVER_PORT=9090 overrides server.port).
package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"gin-scaffold/common/logger"

	"github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

// DefaultEnv is the environment used when the config file omits env.
const DefaultEnv = "dev"

// Config is the root configuration tree. mapstructure tags are what
// viper.Unmarshal reads.
type Config struct {
	Server ServerConf    `mapstructure:"server"`
	Auth   AuthConf      `mapstructure:"auth"`
	CORS   CORSConf      `mapstructure:"cors"`
	Log    logger.Config `mapstructure:"log"`
	MySQL  MySQLConf     `mapstructure:"mysql"`
	Redis  RedisConf     `mapstructure:"redis"`
}

type ServerConf struct {
	Env  string `mapstructure:"env"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type AuthConf struct {
	JWTSecret            string `mapstructure:"jwtSecret"`
	AccessExpireSeconds  int    `mapstructure:"accessExpireSeconds"`
	RefreshExpireSeconds int    `mapstructure:"refreshExpireSeconds"`
	LoginRatePerMinute   int    `mapstructure:"loginRatePerMinute"`
}

// CORSConf controls cross-origin access. An empty AllowOrigins disables CORS.
// AllowCredentials cannot be combined with a wildcard origin.
type CORSConf struct {
	AllowOrigins     []string `mapstructure:"allowOrigins"`
	AllowMethods     []string `mapstructure:"allowMethods"`
	AllowHeaders     []string `mapstructure:"allowHeaders"`
	ExposeHeaders    []string `mapstructure:"exposeHeaders"`
	AllowCredentials bool     `mapstructure:"allowCredentials"`
	MaxAgeSeconds    int      `mapstructure:"maxAgeSeconds"`
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
// DATETIME/TIMESTAMP into time.Time in the local timezone. Dial is bounded to
// 5s and socket reads/writes to 30s so an unreachable server cannot block
// startup or requests indefinitely.
func (m MySQLConf) DSN() string {
	cfg := mysql.NewConfig()
	cfg.User = m.User
	cfg.Passwd = m.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	cfg.DBName = m.Database
	cfg.ParseTime = true
	cfg.Loc = time.Local
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 30 * time.Second
	cfg.WriteTimeout = 30 * time.Second
	cfg.Params = map[string]string{"charset": m.Charset}
	return cfg.FormatDSN()
}

// Addr returns the host:port address for Redis clients.
func (r RedisConf) Addr() string {
	return net.JoinHostPort(r.Host, strconv.Itoa(r.Port))
}

var environmentGinModes = map[string]string{
	"dev":  "debug",
	"test": "test",
	"prod": "release",
}

// GinMode maps the configured environment to Gin's runtime mode.
func (c Config) GinMode() string {
	if mode, ok := environmentGinModes[strings.ToLower(strings.TrimSpace(c.Server.Env))]; ok {
		return mode
	}
	return "debug"
}

// Load applies defaults, reads the YAML file at path, then overlays any
// matching environment variables.
func Load(path string) (Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// defaults < file < env. The replacer maps server.port -> SERVER_PORT.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	c.Server.Env = strings.ToLower(strings.TrimSpace(c.Server.Env))
	return c, nil
}

// setDefaults registers every key. This both lets a sparse file still boot and
// lets viper's AutomaticEnv bind env overrides for keys absent from the file
// (Unmarshal only visits keys viper already knows about).
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.env", DefaultEnv)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("auth.jwtSecret", "")
	v.SetDefault("auth.accessExpireSeconds", 900)
	v.SetDefault("auth.refreshExpireSeconds", 604800)
	v.SetDefault("auth.loginRatePerMinute", 20)
	v.SetDefault("cors.allowOrigins", []string{})
	v.SetDefault("cors.allowMethods", []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allowHeaders", []string{"Authorization", "Content-Type", "X-Request-Id"}) // 认证凭据、JSON 媒体类型、请求追踪 ID
	v.SetDefault("cors.exposeHeaders", []string{"X-Request-Id"})                                 // 允许前端读取请求追踪 ID
	v.SetDefault("cors.allowCredentials", false)
	v.SetDefault("cors.maxAgeSeconds", 600)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("log.rootDir", "logs")
	v.SetDefault("log.dailyRotate", true)
	v.SetDefault("log.timezone", "Asia/Shanghai")
	v.SetDefault("log.maxSizeMB", 300)
	v.SetDefault("log.maxAgeDays", 30)
	v.SetDefault("log.compress", true)
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
