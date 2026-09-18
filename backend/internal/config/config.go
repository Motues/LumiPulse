package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 结构体对应配置文件字段
type Config struct {
	Port               int  `yaml:"PORT"`
	InsecureSkipVerify bool `yaml:"INSECURE_SKIP_VERIFY"` // 全局跳过 HTTPS 证书验证（默认关闭，单服务可在后台单独开启）
	// HeartbeatRetentionDays 心跳原始数据保留天数（默认 30，与延迟接口的最大查询窗口一致）
	HeartbeatRetentionDays int `yaml:"HEARTBEAT_RETENTION_DAYS"`
	// DailyRetentionDays 每日汇总数据保留天数（默认 90，与每日统计接口的最大查询窗口一致）
	DailyRetentionDays int `yaml:"DAILY_RETENTION_DAYS"`
	// Lang 服务端生成内容（RSS / Atom 订阅源、告警邮件、月报邮件）的语言。
	// 支持 zh-CN / en-US，默认 zh-CN（与历史文案保持一致）。
	Lang string `yaml:"LANG"`
}

var GlobalConfig *Config

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Port:                   3000,
		HeartbeatRetentionDays: 30,
		DailyRetentionDays:     90,
		Lang:                   "zh-CN",
	}
}

// HeartbeatRetention 返回生效的心跳保留天数（含兜底）
func (c *Config) HeartbeatRetention() int {
	if c == nil || c.HeartbeatRetentionDays <= 0 {
		return 30
	}
	return c.HeartbeatRetentionDays
}

// DailyRetention 返回生效的每日汇总保留天数（含兜底）
func (c *Config) DailyRetention() int {
	if c == nil || c.DailyRetentionDays <= 0 {
		return 90
	}
	return c.DailyRetentionDays
}

// ContentLang 返回生效的对外内容语言（无法识别时退回 zh-CN）。
// 返回字符串而不是 i18n.Lang，避免 config 反向依赖 i18n 包。
func (c *Config) ContentLang() string {
	if c == nil {
		return "zh-CN"
	}
	lang := strings.TrimSpace(c.Lang)
	if strings.HasPrefix(strings.ToLower(lang), "en") {
		return "en-US"
	}
	return "zh-CN"
}

// LoadConfig 加载或初始化配置文件
func LoadConfig() (*Config, error) {
	configPath := "./config/config.yaml"

	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	// 如果文件不存在，创建并写入默认配置
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultCfg := DefaultConfig()
		data, err := yaml.Marshal(defaultCfg)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, err
		}
		GlobalConfig = defaultCfg
		return defaultCfg, nil
	}

	// 读取现有文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// PORT 环境变量优先
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			cfg.Port = p
		}
	}

	GlobalConfig = &cfg
	return &cfg, nil
}
