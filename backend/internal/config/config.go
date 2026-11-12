package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Pool     PoolConfig     `yaml:"pool"`
	Security SecurityConfig `yaml:"security"`
}

type PoolConfig struct {
	DBMaxOpen     int `yaml:"db_max_open"`      // 数据库最大打开连接数
	DBMaxIdle     int `yaml:"db_max_idle"`      // 数据库最大空闲连接数
	DBMaxLifeSec  int `yaml:"db_max_life_sec"`  // 连接最大存活秒数
	WSSendBuf     int `yaml:"ws_send_buf"`      // WebSocket发送缓冲区大小
	WSReadLimitMB int `yaml:"ws_read_limit_mb"` // WebSocket读取限制(MB)
}

type SecurityConfig struct {
	LoginMaxFail   int `yaml:"login_max_fail"`    // 最大登录失败次数
	LoginLockMin   int `yaml:"login_lock_min"`    // 锁定时长(分钟)
	RateLimitRPS   int `yaml:"rate_limit_rps"`    // API限流(请求/秒/IP)
	PasswordMinLen int `yaml:"password_min_len"`  // 密码最小长度
}

type ServerConfig struct {
	Port string `yaml:"port"`
	Mode string `yaml:"mode"`
}

type DatabaseConfig struct {
	Type     string `yaml:"type"` // 数据库类型: postgres / kingbase / dm (V1.0-14)
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Server:   ServerConfig{Port: ":8080", Mode: "release"},
		Database: DatabaseConfig{Host: "localhost", Port: "5432", User: "postgres", Password: "postgres", DBName: "lab_teaching", SSLMode: "disable"},
		JWT:      JWTConfig{Secret: "change-me-set-jwt-secret-in-config", ExpireHours: 24},
		Pool:     PoolConfig{DBMaxOpen: 50, DBMaxIdle: 10, DBMaxLifeSec: 1800, WSSendBuf: 512, WSReadLimitMB: 4},
		Security: SecurityConfig{LoginMaxFail: 5, LoginLockMin: 15, RateLimitRPS: 100, PasswordMinLen: 8},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// 配置文件不存在时使用默认值
		return cfg, nil
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	// 环境变量覆盖
	if v := os.Getenv("DB_TYPE"); v != "" {
		cfg.Database.Type = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	return cfg, nil
}
