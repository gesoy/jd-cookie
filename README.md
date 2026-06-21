# jd-cookie

KernelSU 模块 —— 自动读取京东 Cookie 并同步至青龙面板。

搭配 [jdpro](https://github.com/6dylan6/jdpro) 使用，彻底告别 Cookie 过期问题。

## 为什么需要这个模块

jdpro 等京东脚本依赖 `JD_COOKIE` 环境变量，但 Cookie 有效期越来越短（部分用户每天过期）。手动抓包费时费力，本模块直接从京东 App WebView 数据库自动读取，每小时自动上传青龙面板，实现零人工维护。

## 功能

- 从京东 App WebView Cookie 数据库读取 `pt_key` / `pt_pin`
- Cookie 去重后自动上传至青龙面板 `JD_COOKIE` 环境变量
- Go 守护进程，内存占用约 7.5 MB
- WebUI 配置管理 + SSE 实时日志流
- Token 鉴权，外部请求一律拒绝

## 完整教程：jdpro + 本模块

### 1. 部署青龙面板

推荐内网部署，青龙 2.15+ 即可。

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

运行订阅 → 依赖安装任务 → 配置通知。

### 3. 安装本模块

下载 [Releases](https://github.com/Gesoy/jd-cookie/releases) 中最新 `jd_assistant.zip`，KernelSU Manager 刷入。

### 4. 配置

1. 手机打开**京东 App** 登录账号（Cookie 写入 WebView 数据库）
2. KernelSU Manager → 模块 → 京东助手 → 打开
3. WebUI 填写青龙面板**地址、用户名、密码**，保存
4. 点击**读取**按钮，或等待自动上传

### 5. 验证

青龙面板 → 环境变量 → `JD_COOKIE` 应出现最新值。jdpro 脚本会自动使用。

## 流程示意

```
京东 App 登录 → WebView Cookie 数据库
                    ↓ 本模块自动读取
            pt_key=xxx;pt_pin=xxx
                    ↓ 上传青龙面板
            JD_COOKIE 环境变量
                    ↓ jdpro 脚本调用
              自动签到、领豆...
```

## 结构

```
├── backend/        Go 守护进程
├── frontend/       Vue 3 WebUI
├── kernelsu/       模块打包目录
│   ├── service.sh  启动脚本
│   └── module.prop
└── .github/workflows/  CI/CD
```

## 构建

```bash
# 后端
cd backend
CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -ldflags="-s -w" -o ../kernelsu/bin/jd-cookie .

# 前端
cd frontend && npm install && npm run build

# 打包
cd kernelsu && zip -r ../jd_assistant.zip .
```

## License

MIT
