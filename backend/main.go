package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/spf13/cobra"
)

// pidAlive 检查进程是否存活 且 确认为 jd-cookie（防止 PID 复用误判）
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == "jd-cookie"
}

// lastCookie 上次成功上传的 Cookie 值，用于去重（内存常驻，重启从 SQLite 恢复）
var lastCookie string

// cookieTS Cookie 值最近一次发生变化的时间；配合强制重传周期判断 pt_key 是否可能过期
var cookieTS time.Time

// runDaemon 服务主循环：启动 HTTP 服务，每小时自动读取并上传 Cookie
func runDaemon() {
	kvSet("pid", strconv.Itoa(os.Getpid()))
	defer kvSet("pid", "")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sigCh; kvSet("pid", ""); os.Exit(0) }()

	logf("服务已启动  PID %d", os.Getpid())

	go func() {
		logf("HTTP 监听  %s", listenAddr)
		if err := httpListen(); err != nil {
			logf("HTTP 异常 - %v", err)
		}
	}()

	// upload 读取到的 Cookie 上传青龙，并在成功/失败时发送 Wxpusher 通知
	upload := func(cfg *Config, cr cookieData, forced bool) {
		if !cr.OK || cr.Cookie == "" {
			return
		}
		qr := uploadCookie(cfg, cr.Cookie)
		if qr.OK {
			logf("[自动] 上传成功  %.0fms", sinceMs(time.Now()))
			saveUpload(time.Now().Format("2006-01-02 15:04:05"))
			lastCookie = cr.Cookie
			kvSet("last_cookie", cr.Cookie)
			title := "[自动] 上传成功"
			if forced {
				title = "[自动] 强制重传成功（Cookie 长时间未变化）"
			}
			notifyWx(cfg, fmt.Sprintf("%s\n青龙面板：%s\n变量：%s\n%s", title, cfg.QLURL, cfg.EnvName, cr.Cookie))
		} else {
			logf("[自动] 上传失败 - %s", qr.Msg)
			notifyWx(cfg, fmt.Sprintf("[自动] 上传失败\n青龙面板：%s\n原因：%s", cfg.QLURL, qr.Msg))
		}
	}

	doCycle := func() {
		defer func() {
			if r := recover(); r != nil {
				logf("循环异常 - %v", r)
			}
		}()

		cfg := loadConfig()
		if !cfg.isValid() {
			logf("跳过 - 青龙配置不完整")
			return
		}

		logf("[自动] 开始执行")
		t0 := time.Now()
		cr := readCookie()
		if !cr.OK {
			logf("读取失败 - %s", cr.Msg)
			return
		}
		logf("读取成功  %s  %s  %.0fms", cr.Pin, cr.Key, sinceMs(t0))

		// 值变化：记录变化时间并正常上传
		if cr.Cookie != lastCookie {
			cookieTS = time.Now()
			kvSet("cookie_ts", strconv.FormatInt(cookieTS.Unix(), 10))
			upload(cfg, cr, false)
			return
		}

		// 值未变化：超过强制重传周期则强制覆盖上传（pt_key 可能已过期但数据库未更新）
		age := time.Since(cookieTS)
		interval := cfg.forceInterval()
		if age >= interval {
			logf("Cookie 已 %s 未变化，强制覆盖上传", fmtDuration(age))
			upload(cfg, cr, true)
			return
		}
		logf("Cookie 未变化（%s，< %s），跳过上传", fmtDuration(age), fmtDuration(interval))
	}

	// 启动后立即执行一次，之后每小时
	doCycle()
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		doCycle()
	}
}

// fmtDuration 简化时长展示，如 2h3m
func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// sinceMs 计算距 t 的毫秒数，用于日志性能计时
func sinceMs(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000
}

func main() {
	var dbFlag, cookieDBFlag, tokenFlag, tzFlag string

	root := &cobra.Command{
		Use:   "jd-cookie",
		Short: "京东助手 - 自动读取京东Cookie并同步至青龙面板",
	}

	daemon := &cobra.Command{
		Use:   "daemon",
		Short: "启动后台服务",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 设置访问令牌
			if tokenFlag != "" {
				apiToken = tokenFlag
			}
			// 设置时区
			if tzFlag != "" {
				if loc, err := time.LoadLocation(tzFlag); err == nil {
					time.Local = loc
				}
			}
			state.setStart()
			// 覆盖默认路径
			if dbFlag != "" {
				dbPathOverride = dbFlag
				modDir = filepath.Dir(dbFlag)
			}
			if cookieDBFlag != "" {
				cookieDBOverride = cookieDBFlag
			}

			os.MkdirAll(modDir, 0755)
			if err := openDB(); err != nil {
				return fmt.Errorf("存储初始化失败: %w", err)
			}
			defer closeDB()

			// 从 SQLite 恢复上次状态
			restoreState()
			lastCookie = kvGet("last_cookie")
			if ts, err := strconv.ParseInt(kvGet("cookie_ts"), 10, 64); err == nil && ts > 0 {
				cookieTS = time.Unix(ts, 0)
			}

			// 单实例检查
			if pidStr := kvGet("pid"); pidStr != "" {
				if pid, err := strconv.Atoi(pidStr); err == nil && pidAlive(pid) && pid != os.Getpid() {
					return fmt.Errorf("已有进程运行中")
				}
			}
			runDaemon()
			return nil
		},
	}
	daemon.Flags().StringVar(&dbFlag, "db", "", "SQLite 数据库路径")
	daemon.Flags().StringVar(&cookieDBFlag, "cookie-db", "", "京东 Cookie 数据库路径")
	daemon.Flags().StringVar(&tokenFlag, "token", "", "API 访问令牌")
	daemon.Flags().StringVar(&tzFlag, "tz", "", "时区，如 Asia/Shanghai")

	version := &cobra.Command{
		Use:   "version",
		Short: "显示版本",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("jd-cookie v4.0.0")
		},
	}

	root.AddCommand(daemon, version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
