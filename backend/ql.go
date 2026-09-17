package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// qlResult 青龙面板 API 调用结果
type qlResult struct {
	OK  bool   `json:"ok"`
	Msg string `json:"msg"`
}

// httpCli 使用自定义 DNS/拨号逻辑，规避 Android /etc/resolv.conf 损坏导致的解析失败
var httpCli = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		DialContext:     dialContext,
		IdleConnTimeout: 10 * time.Second,
	},
}

func doHTTP(method, url string, body interface{}, token string) (*http.Response, []byte, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("序列化失败: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, nil, fmt.Errorf("无效 URL: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpCli.Do(req)
	if err != nil {
		return nil, nil, err
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, data, nil
}

func httpPost(url string, body interface{}, token ...string) (*http.Response, []byte, error) {
	t := ""
	if len(token) > 0 {
		t = token[0]
	}
	return doHTTP("POST", url, body, t)
}

func httpGet(url string, token string) (*http.Response, []byte, error) {
	return doHTTP("GET", url, nil, token)
}

func httpPut(url string, body interface{}, token string) (*http.Response, []byte, error) {
	return doHTTP("PUT", url, body, token)
}

// qlBaseURL 返回去除尾部斜杠的青龙面板地址
func qlBaseURL(cfg *Config) string {
	return strings.TrimRight(cfg.QLURL, "/")
}

// qlLogin 登录青龙面板，返回 token
func qlLogin(cfg *Config) (string, error) {
	_, data, err := httpPost(qlBaseURL(cfg)+"/api/user/login", map[string]string{
		"username": cfg.QLUser,
		"password": cfg.QLPass,
	})
	if err != nil {
		return "", fmt.Errorf("无法连接青龙面板 - %w", err)
	}
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &r) != nil || r.Code != 200 {
		return "", fmt.Errorf("登录失败 (code=%d): %s", r.Code, r.Msg)
	}
	return r.Data.Token, nil
}

// qlFindEnv 查询环境变量 ID，不存在返回 0
func qlFindEnv(cfg *Config, token string) (int, error) {
	u := qlBaseURL(cfg) + "/api/envs?searchValue=" + url.QueryEscape(cfg.EnvName)
	resp, data, err := httpGet(u, token)
	if err != nil {
		return 0, fmt.Errorf("查询环境变量失败 - %w", err)
	}
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("查询环境变量失败 (HTTP %d)", resp.StatusCode)
	}
	var r struct {
		Code int `json:"code"`
		Data []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &r) != nil || r.Code != 200 {
		raw := string(data)
		if len(raw) > 100 {
			raw = raw[:100]
		}
		return 0, fmt.Errorf("查询环境变量失败: %s", raw)
	}
	for _, env := range r.Data {
		if env.Name == cfg.EnvName {
			return env.ID, nil
		}
	}
	return 0, nil
}

// qlUpsertEnv 创建或更新环境变量
func qlUpsertEnv(cfg *Config, token string, id int, value string) error {
	body := map[string]interface{}{"name": cfg.EnvName, "value": value}
	if id > 0 {
		body["id"] = id
	}
	resp, data, err := httpPut(qlBaseURL(cfg)+"/api/envs", body, token)
	if err != nil {
		return fmt.Errorf("操作失败 - %w", err)
	}
	if resp.StatusCode != 200 {
		raw := string(data)
		if len(raw) > 200 {
			raw = raw[:200]
		}
		return fmt.Errorf("操作失败 (HTTP %d): %s", resp.StatusCode, raw)
	}
	return nil
}

// uploadCookie 上传指定 Cookie 到青龙面板
func uploadCookie(cfg *Config, cookie string) qlResult {
	if !cfg.isValid() {
		return qlResult{OK: false, Msg: "青龙配置不完整"}
	}
	if cookie == "" {
		return qlResult{OK: false, Msg: "Cookie 为空"}
	}
	token, err := qlLogin(cfg)
	if err != nil {
		return qlResult{OK: false, Msg: err.Error()}
	}
	id, err := qlFindEnv(cfg, token)
	if err != nil {
		return qlResult{OK: false, Msg: err.Error()}
	}
	if err := qlUpsertEnv(cfg, token, id, cookie); err != nil {
		return qlResult{OK: false, Msg: err.Error()}
	}
	return qlResult{OK: true, Msg: "环境变量已更新"}
}

// uploadFromDB 从京东数据库读取 Cookie 后上传
func uploadFromDB(cfg *Config) qlResult {
	if !cfg.isValid() {
		return qlResult{OK: false, Msg: "青龙配置不完整"}
	}
	cr := readCookie()
	if !cr.OK {
		return qlResult{OK: false, Msg: cr.Msg}
	}
	return uploadCookie(cfg, cr.Cookie)
}

// testQL 测试青龙面板连接
func testQL(cfg *Config) qlResult {
	if !cfg.isValid() {
		return qlResult{OK: false, Msg: "青龙配置不完整"}
	}
	_, err := qlLogin(cfg)
	if err != nil {
		return qlResult{OK: false, Msg: err.Error()}
	}
	return qlResult{OK: true, Msg: "连接成功"}
}
