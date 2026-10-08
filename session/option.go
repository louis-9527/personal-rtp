package session

import "time"

const defaultIdleTTL = time.Hour

// Config 是策略会话的可配置参数。
type Config struct {
	IdleTTL time.Duration // 相邻两笔投注仍归属同一会话的最大时间间隔。
}

// Option 用于配置 Manager。
type Option func(*Config)

// DefaultConfig 返回策略会话的默认配置。
func DefaultConfig() Config {
	return Config{IdleTTL: defaultIdleTTL}
}

// WithIdleTTL 设置相邻两笔投注仍归属同一会话的最大时间间隔。
func WithIdleTTL(ttl time.Duration) Option {
	return func(c *Config) {
		if ttl > 0 {
			c.IdleTTL = ttl
		}
	}
}
