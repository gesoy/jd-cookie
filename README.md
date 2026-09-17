# 京东助手 jd-cookie

> 自动读取京东 App Cookie 并同步至青龙面板的 **Magisk / KernelSU / APatch** 模块。

无需抓包、无需浏览器插件——只要手机京东 App 登录态有效，模块会自动从 WebView 数据库提取 `pt_key` / `pt_pin`，上传到青龙面板 `JD_COOKIE` 环境变量，配合 [jdpro](https://github.com/6dylan6/jdpro) 等脚本自动完成任务，显著减少 Cookie 过期后的手动维护。

---

## ✨ 特性

- **自动读取**：直接读取 `/data/data/com.jingdong.app.mall/app_webview/Default/Cookies`，提取 `pt_key` / `pt_pin`
- **自动上传**：Cookie 去重后自动写入青龙面板 `JD_COOKIE` 环境变量
- **多平台支持**：同时兼容 **Magisk / KernelSU / APatch** 三种 Root 方案
- **WebUI 管理**：内置可视化配置页面，支持青龙面板设置、Token 鉴权、实时日志流
- **低占用**：Go 静态编译的轻量守护进程，内存占用约 7.5 MB
- **强制重传**：Cookie 长时间未变化（默认 12 小时，可配置）会强制覆盖上传，规避 `pt_key` 过期但数据库值不变导致的"永不重传"
- **Wxpusher 通知**：自动 / 手动上传成功、失败、强制重传时均可推送微信通知
- **故障自愈**：守护进程崩溃自动重启；内置自建 DNS 解析器，规避 Android DNS 失效问题

---

## 🛠 相比原版 KernelSU-only 的增强

| 能力 | 说明 |
| --- | --- |
| **Magisk / APatch 适配** | 新增 `sepolicy.rule` 放行 SELinux 策略、`uninstall.sh` 卸载清理、`customize.sh` / `service.sh` / `module.prop` 多方案适配 |
| **WebUI 可达** | 原版依赖 KernelSU Manager 渲染，Magisk 下无入口；现由 Go 守护进程直接托管静态页面，浏览器访问 `http://127.0.0.1:17320` 即达 |
| **DNS 修复** | 解决 Android `/etc/resolv.conf` 缺失/损坏导致的 `dial tcp: lookup ... connection refused` / `i/o timeout`，青龙面板无法连接的问题 |
| **Token 修复** | 新增免鉴权 `/api/status` 与 `/api/token` 端点，前端优先从后端取令牌，修复 WebUI 一直显示 offline、读取/上传报 403 的问题 |
| **强制重传** | 记录 Cookie 最近变化时间，超过配置周期（默认 12h）仍未变化时强制覆盖上传，避免过期 Cookie 长时间不刷新 |
| **Wxpusher 通知** | 上传成功/失败/强制重传时通过 Wxpusher 推送微信通知，可配置 appToken 与多个接收 UID |

> 详见本项目 [`README`](README.md) 底部「常见问题」。

---

## 📖 完整教程：jdpro + 本模块

### 1. 部署青龙面板

推荐内网部署，青龙 **2.15+** 即可。

### 2. 订阅 jdpro

青龙面板 → 订阅管理 → 创建订阅：

```
名称: jdpro
类型: 公开仓库
链接: https://github.com/6dylan6/jdpro.git
分支: main
白名单: jd_|jx_|jddj_
黑名单: backUp
依赖文件: ^jd[^_]|USER|JD|function|sendNotify|utils
```

运行订阅 → 安装依赖 → 配置通知。

### 3. 安装本模块

下载 [Releases](https://github.com/Gesoy/jd-cookie/releases) 中的最新 zip，用 **Magisk Manager / KernelSU Manager / APatch** 刷入并重启。

- **Magisk 用户**：模块会自动注入 `sepolicy.rule` 放行 SELinux 权限、注册 `uninstall.sh` 清理脚本，无需额外配置。
- **KernelSU / APatch 用户**：行为与原版 KernelSU 一致，WebUI 由守护进程直接托管。

### 4. 配置

1. 手机打开**京东 App** 并登录（Cookie 写入 WebView 数据库）
2. **Magisk / KernelSU Manager** → 模块 → **京东助手** → 开启（守护进程挂载在 `127.0.0.1:17320`）
3. 浏览器访问 `http://127.0.0.1:17320` 打开内置 WebUI
4. 填写青龙面板**地址、用户名、密码**（可选自定义环境变量名，默认 `JD_COOKIE`），保存
5. 点击**测试连接**确认可达，再点**读取**手动触发，或等待自动定时上传

### 5. 验证

青龙面板 → 环境变量 → `JD_COOKIE` 出现最新值即成功，jdpro 脚本会自动使用。

---

## 🔄 流程示意

```
京东 App 登录 → WebView Cookie 数据库
                    ↓ 本模块自动读取
            pt_key=xxx;pt_pin=xxx
                    ↓ 上传青龙面板
            JD_COOKIE 环境变量
                    ↓ jdpro 脚本调用
              自动签到、领豆...
```

---

## 🗂 目录结构

```
├── backend/          Go 守护进程
│   ├── dns.go        自建 DNS 解析器（规避 Android DNS 失效）
│   ├── ql.go         青龙面板 API 客户端（自定义 DNS 拨号）
│   ├── server.go     HTTP 服务 / WebUI 静态资源托管
│   ├── main.go       守护进程入口 & 定时循环
│   └── ...
├── frontend/         Vue 3 WebUI 前端源码
├── kernelsu/         模块打包目录（兼容 Magisk / KernelSU / APatch）
│   ├── customize.sh  安装 / 权限 / SELinux 适配脚本
│   ├── service.sh    开机启动脚本（boot_completed 等待 + 崩溃重启）
│   ├── sepolicy.rule Magisk SELinux 策略
│   ├── uninstall.sh  卸载清理脚本
│   ├── module.prop   模块元信息
│   ├── bin/          jd-cookie 二进制（构建产物）
│   └── webroot/      内置 WebUI 静态资源
└── .github/workflows/ CI / 发布
```

---

## 🔧 开发 & 构建

前置：Go 1.21+、Node.js 18+。

```bash
# 1. 构建后端（Android arm64 静态编译）
cd backend
CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -ldflags="-s -w" -o ../kernelsu/bin/jd-cookie .

# 2. 构建前端
cd ../frontend && npm install && npm run build
# 构建产物输出到 kernelsu/webroot

# 3. 打包模块
cd ../kernelsu && zip -r ../jd-cookie-<version>.zip .
```

> 说明：
> - `CGO_ENABLED=0` 保证纯静态链接，规避 Android 动态库依赖；`-ldflags="-s -w"` 精简体积。
> - 后端 `main.go` 已 `import _ "time/tzdata"`，内置时区数据库，无需依赖系统 tzdata。
> - CI 见 `.github/workflows/`，可自动完成上述构建与 Release 发布。

---

## ❓ 常见问题

**Q1：Magisk 下打开模块后没有图标 / 弹窗？**
现版本无需图标——守护进程自身托管 WebUI，浏览器访问 `http://127.0.0.1:17320` 即可进入配置页。

**Q2：报错 `dial tcp: lookup xxx: i/o timeout` 或 `connection refused`？**
这是 Android 下 `/etc/resolv.conf` 缺失/损坏导致的 DNS 解析失败（常见于仅 IPv6 或自定义网络）。本模块已内置自建 DNS 解析器：读取系统真实 DNS（`getprop net.dns*`）并回退公共 DNS（含 IPv6），逐服务器重试，优先 IPv6。

**Q3：青龙面板走反向代理 / 非标准端口？**
在 WebUI 的青龙地址中填写完整 URL（含协议、域名、端口），例如 `https://ql.example.com:16667`，保存后测试连接即可。

**Q4：如何卸载？**
在模块管理器中直接卸载即可。`uninstall.sh` 会自动终止守护进程并清理 `token.txt`、`webroot`、日志等运行时文件。

**Q5：Cookie 过期后为什么模块不主动重传 / 一直显示 offline 或报 403？**
旧版依靠"值变化才上传"去重，Cookie 过期但数据库值不变时会一直跳过；且前端靠同源 `token.txt` 取令牌可能读不到而报 403。现版本新增**强制重传**（Cookie 超过周期未变化即强制覆盖上传，默认 12 小时，可在 WebUI 调整）与**免鉴权状态/令牌端点**，已解决上述问题。

**Q6：如何配置 Wxpusher 微信通知？**
1. 在 [Wxpusher](https://wxpusher.zjiecode.com/) 注册并创建应用，获取 **AppToken（`AT_` 开头）**；
2. 关注应用得到你的 **UID（`UID_` 开头）**，多个接收人用逗号分隔；
3. 在 WebUI → "Wxpusher 通知" 填入 AppToken 与 UID，保存即可。上传成功、失败、强制重传都会推送。

---

## 📜 License

MIT