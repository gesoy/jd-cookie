package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"
)

const listenAddr = "127.0.0.1:17320"

var apiToken string

func initToken() string {
	if apiToken != "" {
		return apiToken
	}
	b := make([]byte, 16)
	rand.Read(b)
	apiToken = hex.EncodeToString(b)
	return apiToken
}

type apiResp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func respOK(data ...interface{}) apiResp {
	if len(data) == 0 {
		return apiResp{Code: 0, Msg: "ok"}
	}
	return apiResp{Code: 0, Msg: "ok", Data: data[0]}
}

func respErr(msg string) apiResp {
	return apiResp{Code: 1, Msg: msg}
}

type statusSnapshot struct {
	Running    bool   `json:"running"`
	StartTime  string `json:"start_time"`
	LastUpload string `json:"last_upload"`
}

func writeResp(w http.ResponseWriter, r apiResp) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(r)
}

// authMux 全局 token 校验，无 token 直接 403
func authMux(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		t := r.Header.Get("X-Token")
		if t == "" {
			t = r.URL.Query().Get("token")
		}
		if t != apiToken {
			w.WriteHeader(403)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Token")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func httpListen() error {
	initToken()

	// API 采用子 mux。健康检查 GET /status 免鉴权（用于前端判定在线状态）；
	// 其余 /api/* 接口需要 token 鉴权。staticHandler 提供 WebUI 静态页面与 token.txt，
	// 供浏览器同源加载。监听仅限 127.0.0.1，无 token 时仅能读状态，操作会被拒绝。
	api := http.NewServeMux()

	api.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		writeResp(w, respOK(state.snapshot()))
	})

	api.HandleFunc("GET /cookie", func(w http.ResponseWriter, r *http.Request) {
		cr := readCookie()
		if cr.OK {
			logf("读取Cookie  %s  %s", cr.Pin, cr.Key)
		} else {
			logf("读取Cookie失败 - %s", cr.Msg)
		}
		writeResp(w, respOK(cr))
	})

	api.HandleFunc("POST /cookie", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Cookie string `json:"cookie"` }
		data, _ := io.ReadAll(r.Body)
		cfg := loadConfig()
		var qr qlResult
		val := ""
		if len(data) > 0 && json.Unmarshal(data, &body) == nil && body.Cookie != "" {
			val = body.Cookie
			qr = uploadCookie(cfg, body.Cookie)
		} else {
			if cr := readCookie(); cr.OK {
				val = cr.Cookie
			}
			qr = uploadFromDB(cfg)
		}
		if qr.OK {
			saveUpload(time.Now().Format("2006-01-02 15:04:05"))
			logf("[手动] 上传成功")
			if val != "" {
				notifyWx(cfg, fmt.Sprintf("[手动] 上传成功\n青龙面板：%s\n变量：%s\n%s", cfg.QLURL, cfg.EnvName, val))
			} else {
				notifyWx(cfg, fmt.Sprintf("[手动] 上传成功\n青龙面板：%s\n变量：%s", cfg.QLURL, cfg.EnvName))
			}
		} else {
			logf("[手动] 上传失败 - %s", qr.Msg)
			notifyWx(cfg, fmt.Sprintf("[手动] 上传失败\n青龙面板：%s\n原因：%s", cfg.QLURL, qr.Msg))
		}
		writeResp(w, respOK(qr))
	})

	api.HandleFunc("GET /config", func(w http.ResponseWriter, r *http.Request) {
		writeResp(w, respOK(loadConfig()))
	})
	api.HandleFunc("PUT /config", func(w http.ResponseWriter, r *http.Request) {
		var cfg Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeResp(w, respErr("JSON 解析失败"))
			return
		}
		if cfg.EnvName == "" {
			writeResp(w, respErr("请填写环境变量名称"))
			return
		}
		if !cfg.isValid() {
			writeResp(w, respErr("请填写面板地址、用户名和密码"))
			return
		}
		saveConfig(&cfg)
		logf("保存配置  %s  %s:***@%s", cfg.EnvName, cfg.QLUser, cfg.QLURL)
		writeResp(w, respOK())
	})

	api.HandleFunc("POST /test", func(w http.ResponseWriter, r *http.Request) {
		var tmp Config
		cfg := loadConfig()
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			if len(body) > 0 && json.Unmarshal(body, &tmp) == nil {
				if tmp.QLURL != "" {
					cfg.QLURL = tmp.QLURL
				}
				if tmp.QLUser != "" {
					cfg.QLUser = tmp.QLUser
				}
				if tmp.QLPass != "" {
					cfg.QLPass = tmp.QLPass
				}
				if tmp.EnvName != "" {
					cfg.EnvName = tmp.EnvName
				}
			}
		}
		qr := testQL(cfg)
		if qr.OK {
			logf("测试连接成功  %s", cfg.QLURL)
			writeResp(w, respOK())
		} else {
			logf("测试连接失败 - %s", qr.Msg)
			writeResp(w, respErr(qr.Msg))
		}
	})

	api.HandleFunc("GET /log/stream", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		rows := logQuery(1000)
		for i := len(rows) - 1; i >= 0; i-- {
			b, _ := json.Marshal(rows[i])
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		}

		ch := subscribe()
		defer unsubscribe(ch)
		for {
			select {
			case <-r.Context().Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", msg)
				fl.Flush()
			}
		}
	})

	api.HandleFunc("DELETE /log", func(w http.ResponseWriter, r *http.Request) {
		logClear()
		writeResp(w, respOK())
	})

	// 组装：GET /api/status 免鉴权（前端在线判定），其余 /api/* 走 token 鉴权，
	// 其余路径提供 WebUI 静态资源（免鉴权）。
	mux := http.NewServeMux()

	// status 单独暴露（不含敏感信息），避免前端 token 未就绪导致界面永远 offline
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeResp(w, respOK(state.snapshot()))
	})

	// token 也单独暴露（本机回环监听、不对外），使 WebUI 能在无同源 token.txt 时取得令牌，
	// 从而正常执行读取/上传等受保护操作；其余 /api/* 仍需 token 校验予以防盗链。
	mux.HandleFunc("/api/token", func(w http.ResponseWriter, r *http.Request) {
		writeResp(w, respOK(initToken()))
	})

	mux.Handle("/api/", http.StripPrefix("/api", authMux(api)))
	mux.Handle("/", staticHandle(modDir+"/webroot"))

	return http.ListenAndServe(listenAddr, cors(mux))
}

// staticHandle 提供 WebUI 静态文件与 token.txt；
// 让浏览器可直接打开 http://127.0.0.1:17320 使用界面（无需 KernelSU Manager）。
func staticHandle(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	fsd := http.Dir(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 未命中文件时回退到 index.html（SPA 路由兜底）
		p := r.URL.Path
		if r.Method == http.MethodGet && p != "/" {
			if _, err := fsd.Open(p); err != nil {
				http.ServeFile(w, r, filepath.Join(dir, "index.html"))
				return
			}
		}
		fs.ServeHTTP(w, r)
	})
}
