const BASE = 'http://127.0.0.1:17320'

let token = ''
let tokenReady = false
// 优先从后端取得鉴权令牌（本机回环监听，无需 cookie/跨域）；取不到则用同源 token.txt，
// 仍失败则不再携带 token，仅能访问免鉴权接口。
;(async () => {
  try {
    const res = await fetch(BASE + '/api/token')
    if (res.ok) {
      const body = await res.json()
      if (body && body.code === 0) { token = String(body.data || '').trim(); return }
    }
  } catch (e) {}
  try {
    token = (await fetch('token.txt').then(r => r.text())).trim()
  } catch (e) {}
})()
  .catch(() => {})
  .finally(() => { tokenReady = true })

// 等待 token 初始化完成（最多 3s），避免请求与 token 读取竞态
async function ensureToken() {
  const deadline = Date.now() + 3000
  while (!tokenReady && Date.now() < deadline) {
    await new Promise(r => setTimeout(r, 30))
  }
}

async function request(url, options = {}) {
  await ensureToken()
  if (token) {
    options.headers = options.headers || {}
    options.headers['X-Token'] = token
  }
  const res = await fetch(BASE + url, options)
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  const body = await res.json()
  if (body.code !== 0) throw new Error(body.msg || 'error')
  return body.data !== undefined ? body.data : { ok: true, msg: body.msg }
}

export const api = {
  status() { return request('/api/status') },
  readCookie() { return request('/api/cookie') },
  upload(body) {
    const opts = { method: 'POST', headers: { 'Content-Type': 'application/json' } }
    if (body) opts.body = JSON.stringify(body)
    return request('/api/cookie', opts)
  },
  getConfig() { return request('/api/config') },
  setConfig(config) {
    return request('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config)
    })
  },
  testQL(forceCfg) {
    const opts = { method: 'POST', headers: { 'Content-Type': 'application/json' } }
    if (forceCfg) opts.body = JSON.stringify(forceCfg)
    return request('/api/test', opts)
  },
  clearLog() { return request('/api/log', { method: 'DELETE' }) }
}

export function logStream(onLine) {
  const url = BASE + '/api/log/stream' + (token ? '?token=' + encodeURIComponent(token) : '')
  const es = new EventSource(url)
  es.onmessage = (e) => { if (e.data) onLine(e.data) }
  es.onerror = () => { es.close(); setTimeout(() => { logStream(onLine) }, 2000) }
  return es
}
