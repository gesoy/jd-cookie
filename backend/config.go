package main

import (
	"net/url"
	"strconv"
	"time"
)

// Config 青龙面板连接配置
type Config struct {
	QLURL   string `json:"ql_url"`
	QLUser  string `json:"ql_user"`
	QLPass  string `json:"ql_pass"`
	EnvName string `json:"env_name"`

	// ForceHours 强制周期重传（小时）。Cookie 值超过该时长未变化时强制覆盖上传，用于规避
	// pt_key 过期但数据库值不变导致的“永不重传”。0 表示使用默认值 12 小时。
	ForceHours int `json:"force_hours"`

	// WxToken / WxUID Wxpusher 通知配置（appToken / 接收 UID，UID 支持逗号分隔多个）
	WxToken string `json:"wx_token"`
	WxUID   string `json:"wx_uid"`
}

// forceInterval 返回强制重传周期，未配置时默认 12 小时
func (c *Config) forceInterval() time.Duration {
	if c.ForceHours > 0 {
		return time.Duration(c.ForceHours) * time.Hour
	}
	return 12 * time.Hour
}

func (c *Config) isValid() bool {
	if c.QLURL == "" || c.QLUser == "" || c.QLPass == "" {
		return false
	}
	u, err := url.Parse(c.QLURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	return true
}

// loadConfig 从 KV 表读取青龙面板配置
func loadConfig() *Config {
	c := &Config{
		QLURL:   kvGet("ql_url"),
		QLUser:  kvGet("ql_user"),
		QLPass:  kvGet("ql_pass"),
		EnvName: kvGet("env_name"),
		WxToken: kvGet("wx_token"),
		WxUID:   kvGet("wx_uid"),
	}
	if c.EnvName == "" {
		c.EnvName = "JD_COOKIE"
	}
	if v, err := strconv.Atoi(kvGet("force_hours")); err == nil && v > 0 {
		c.ForceHours = v
	}
	return c
}

// saveConfig 将青龙面板配置写入 KV 表
func saveConfig(c *Config) {
	kvSet("ql_url", c.QLURL)
	kvSet("ql_user", c.QLUser)
	kvSet("ql_pass", c.QLPass)
	kvSet("env_name", c.EnvName)
	kvSet("force_hours", strconv.Itoa(c.ForceHours))
	kvSet("wx_token", c.WxToken)
	kvSet("wx_uid", c.WxUID)
}
