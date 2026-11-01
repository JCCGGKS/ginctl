package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// check is one named validation rule. Keeping rules as data makes the startup
// contract easy to scan and extend without growing a long if/else chain.
type check struct {
	field string
	err   error
}

// Validate reports every invalid setting in one pass.
func (c Config) Validate() error {
	checks := []check{
		{"server.env", validateEnv(c.Server.Env)},
		{"server.port", inRange(c.Server.Port, 1, 65535)},
		{"auth.jwtSecret", required(c.Auth.JWTSecret)},
		{"auth.jwtSecret", prodSecret(c.Server.Env, c.Auth.JWTSecret)},
		{"auth.accessExpireSeconds", positive(c.Auth.AccessExpireSeconds)},
		{"auth.refreshExpireSeconds", positive(c.Auth.RefreshExpireSeconds)},
		{"auth.loginRatePerMinute", inRange(c.Auth.LoginRatePerMinute, 1, 10000)},
		{"cors.allowOrigins", validateOrigins(c.CORS.AllowOrigins, c.CORS.AllowCredentials)},
		{"cors.allowMethods", validateHTTPTokens(c.CORS.AllowMethods)},
		{"cors.allowHeaders", validateHTTPTokens(c.CORS.AllowHeaders)},
		{"cors.exposeHeaders", validateHTTPTokens(c.CORS.ExposeHeaders)},
		{"cors.maxAgeSeconds", inRange(c.CORS.MaxAgeSeconds, 0, 86400)},
		{"mysql.host", required(c.MySQL.Host)},
		{"mysql.port", inRange(c.MySQL.Port, 1, 65535)},
		{"mysql.user", required(c.MySQL.User)},
		{"mysql.password", prodRequired(c.Server.Env, c.MySQL.Password)},
		{"mysql.database", required(c.MySQL.Database)},
		{"mysql.charset", required(c.MySQL.Charset)},
		{"mysql.maxOpenConns", positive(c.MySQL.MaxOpenConns)},
		{"mysql.maxIdleConns", inRange(c.MySQL.MaxIdleConns, 0, c.MySQL.MaxOpenConns)},
		{"mysql.connMaxLifetimeSeconds", nonNegative(c.MySQL.ConnMaxLifetimeSeconds)},
		{"mysql.connMaxIdleTimeSeconds", nonNegative(c.MySQL.ConnMaxIdleTimeSeconds)},
		{"redis.host", required(c.Redis.Host)},
		{"redis.port", inRange(c.Redis.Port, 1, 65535)},
		{"redis.password", prodRequired(c.Server.Env, c.Redis.Password)},
		{"redis.db", nonNegative(c.Redis.DB)},
		{"redis.poolSize", positive(c.Redis.PoolSize)},
		{"redis.minIdleConns", inRange(c.Redis.MinIdleConns, 0, c.Redis.PoolSize)},
		{"redis.dialTimeoutSeconds", positive(c.Redis.DialTimeoutSeconds)},
		{"redis.readTimeoutSeconds", positive(c.Redis.ReadTimeoutSeconds)},
		{"redis.writeTimeoutSeconds", positive(c.Redis.WriteTimeoutSeconds)},
		{"redis.idleTimeoutSeconds", positive(c.Redis.IdleTimeoutSeconds)},
		{"log.level", oneOf(c.Log.Level, "debug", "info", "warn", "error")},
		{"log.format", oneOf(c.Log.Format, "console", "json")},
		{"log.rootDir", required(c.Log.RootDir)},
		{"log.timezone", required(c.Log.Timezone)},
		{"log.timezone", validTimezone(c.Log.Timezone)},
		{"log.maxSizeMB", nonNegative(c.Log.MaxSizeMB)},
		{"log.maxSizeMB", logRotation(c.Log.DailyRotate, c.Log.MaxSizeMB, c.Log.MaxAgeDays, c.Log.Compress)},
		{"log.maxAgeDays", nonNegative(c.Log.MaxAgeDays)},
	}

	var errs []error
	for _, rule := range checks {
		if rule.err != nil {
			errs = append(errs, fmt.Errorf("%s %w", rule.field, rule.err))
		}
	}
	return errors.Join(errs...)
}

func validateEnv(env string) error {
	if _, ok := environmentGinModes[strings.ToLower(strings.TrimSpace(env))]; !ok {
		return fmt.Errorf("must be dev, test, or prod, got %q", env)
	}
	return nil
}

func isProd(env string) bool {
	return strings.EqualFold(strings.TrimSpace(env), "prod")
}

func required(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("is required")
	}
	return nil
}

func prodRequired(env, value string) error {
	if !isProd(env) {
		return nil
	}
	if strings.TrimSpace(value) == "" {
		return errors.New("is required in prod")
	}
	return nil
}

func prodSecret(env, secret string) error {
	if !isProd(env) {
		return nil
	}
	if len(strings.TrimSpace(secret)) < 32 {
		return errors.New("must be at least 32 bytes in prod")
	}
	return nil
}

func positive(value int) error {
	if value <= 0 {
		return fmt.Errorf("must be positive, got %d", value)
	}
	return nil
}

func nonNegative(value int) error {
	if value < 0 {
		return fmt.Errorf("must not be negative, got %d", value)
	}
	return nil
}

func inRange(value, min, max int) error {
	if value < min || value > max {
		return fmt.Errorf("must be between %d and %d, got %d", min, max, value)
	}
	return nil
}

func oneOf(value string, allowed ...string) error {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if normalized == candidate {
			return nil
		}
	}
	return fmt.Errorf("must be one of %s, got %q", strings.Join(allowed, ", "), value)
}

func validTimezone(timezone string) error {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		return nil
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return fmt.Errorf("is invalid: %w", err)
	}
	return nil
}

func logRotation(dailyRotate bool, maxSizeMB, maxAgeDays int, compress bool) error {
	if dailyRotate || maxSizeMB > 0 || (maxAgeDays == 0 && !compress) {
		return nil
	}
	return errors.New("must be positive when daily rotation is disabled and retention or compression is enabled")
}

func validateOrigins(origins []string, allowCredentials bool) error {
	var errs []error
	wildcard := false
	for index, raw := range origins {
		origin := strings.TrimSpace(raw)
		if origin == "" {
			errs = append(errs, fmt.Errorf("item %d is empty", index))
			continue
		}
		if origin == "*" {
			wildcard = true
			continue
		}
		if strings.ContainsAny(origin, " \t\r\n") {
			errs = append(errs, fmt.Errorf("item %d contains whitespace", index))
			continue
		}

		parsed, err := url.Parse(origin)
		if err != nil || parsed.User != nil {
			errs = append(errs, fmt.Errorf("item %d is not a valid origin", index))
			continue
		}
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "http" && scheme != "https" {
			errs = append(errs, fmt.Errorf("item %d must use http or https", index))
			continue
		}
		if parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			errs = append(errs, fmt.Errorf("item %d must contain only scheme and host", index))
		}
	}
	if wildcard && allowCredentials {
		errs = append(errs, errors.New("wildcard origin cannot be used with credentials"))
	}
	return errors.Join(errs...)
}

func validateHTTPTokens(values []string) error {
	var errs []error
	for index, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "*" {
			errs = append(errs, fmt.Errorf("item %d: wildcard is only supported in cors.allowOrigins", index))
			continue
		}
		if value == "" || !isHTTPToken(value) {
			errs = append(errs, fmt.Errorf("item %d is not a valid HTTP token", index))
		}
	}
	return errors.Join(errs...)
}

func isHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			strings.ContainsRune("!#$%&'*+-.^_`|~", rune(ch)) {
			continue
		}
		return false
	}
	return true
}
