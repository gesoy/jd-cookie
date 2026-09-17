package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// wxpusherAPI Wxpusher 消息发送接口
const wxpusherAPI = "https://wxpusher.zjiecode.com/api/send/message"

// wxUids 解析配置中的 UID（支持逗号分隔多个）
func wxUids(cfg *Config) []string {
	var uids []string
	for _, s := range strings.Split(cfg.WxUID, ",") {
		if s = strings.TrimSpace(s); s != "" {
			uids = append(uids, s)
		}
	}
	return uids
}

// notifyWx 通过 Wxpusher 发送通知；未配置 appToken/UID 时静默跳过
func notifyWx(cfg *Config, content string) {
	if cfg == nil || cfg.WxToken == "" {
		return
	}
	uids := wxUids(cfg)
	if len(uids) == 0 {
		return
	}
	body := map[string]interface{}{
		"appToken":    cfg.WxToken,
		"content":     content,
		"summary":     summaryOf(content),
		"contentType": 1, // 1=文本
		"uids":        uids,
	}
	resp, data, err := httpPost(wxpusherAPI, body)
	if err != nil {
		logf("Wxpusher 通知失败 - %v", err)
		return
	}
	if resp.StatusCode != 200 {
		logf("Wxpusher 通知失败 (HTTP %d): %s", resp.StatusCode, truncateBytes(data, 200))
		return
	}
	var r struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(data, &r) == nil && r.Code != 1000 {
		logf("Wxpusher 通知失败 (code=%d): %s", r.Code, truncateBytes(data, 200))
	}
}

// summaryOf 取通知内容第一行作为推送摘要（Wxpusher summary 上限 100 字）
func summaryOf(content string) string {
	if i := strings.IndexByte(content, '\n'); i > 0 {
		content = content[:i]
	}
	content = strings.TrimSpace(content)
	if len(content) > 100 {
		content = content[:100]
	}
	return content
}

// truncateBytes 截断响应内容用于日志展示
func truncateBytes(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		s = s[:n]
	}
	return fmt.Sprintf("%s", s)
}
