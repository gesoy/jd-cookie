<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { api, logStream } from './api.js'

const connected = ref(false)
const connecting = ref(true)
const uptime = ref('0s')
const lastUpload = ref('')
const cookie = ref(null)
const cfgUrl = ref('')
const cfgUser = ref('')
const cfgPass = ref('')
const cfgEnv = ref('JD_COOKIE')
const cfgForce = ref(0)
const cfgWxToken = ref('')
const cfgWxUID = ref('')
const busyRead = ref(false)
const busyUploadOnly = ref(false)
const busyTest = ref(false)
const busySave = ref(false)
const logs = ref([])
const toast = ref({ show: false, msg: '', ok: true })
let es = null
let toastTimer = null
let uptimeTimer = null
let startTimeMs = 0

function fmtUptime(ms) {
  const s = Math.floor(ms / 1000)
  if (s < 60) return s + 's'
  if (s < 3600) return Math.floor(s / 60) + 'm' + (s % 60) + 's'
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h + 'h' + m + 'm'
}

function tickUptime() {
  uptime.value = fmtUptime(Date.now() - startTimeMs)
}

function showToast(msg, ok = true) {
  toast.value = { show: true, msg, ok }
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toast.value.show = false }, 3000)
}

onMounted(async () => {
  for (let i = 0; i < 15; i++) {
    try {
      const s = await api.status()
      if (s.running) {
        connected.value = true; connecting.value = false
        startTimeMs = new Date(s.start_time).getTime()
        lastUpload.value = s.last_upload || ''
        tickUptime()
        break
      }
    } catch (e) {}
    await new Promise(r => setTimeout(r, 1000))
  }
  connecting.value = false
  if (!connected.value) return

  uptimeTimer = setInterval(tickUptime, 1000)

  try { const c = await api.getConfig(); cfgUrl.value = c.ql_url || ''; cfgUser.value = c.ql_user || ''; cfgPass.value = c.ql_pass || ''; cfgEnv.value = c.env_name || 'JD_COOKIE'; cfgForce.value = c.force_hours || 0; cfgWxToken.value = c.wx_token || ''; cfgWxUID.value = c.wx_uid || '' } catch (e) {}

  nextTick(() => {
    es = logStream(line => {
      try {
        const j = JSON.parse(line)
        logs.value.unshift({ t: j.time.slice(5), m: j.msg })
      } catch (e) {}
    })
  })

  setInterval(async () => { try { const s = await api.status(); lastUpload.value = s.last_upload || '' } catch (e) {} }, 10000)
})
onUnmounted(() => { if (es) es.close(); if (uptimeTimer) clearInterval(uptimeTimer) })

async function doRead() {
  busyRead.value = true
  try { cookie.value = await api.readCookie(); showToast('读取成功') } catch (e) { showToast(e.message, false) }
  busyRead.value = false
}
async function doUploadOnly() {
  busyUploadOnly.value = true
  try {
    await api.upload()
    showToast('上传成功')
    const d = new Date()
    const pad = n => String(n).padStart(2, '0')
    lastUpload.value = `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  } catch (e) { showToast(e.message, false) }
  busyUploadOnly.value = false
}
async function doSave() {
  if (!cfgUrl.value.trim() || !cfgUser.value.trim() || !cfgPass.value.trim()) { showToast('请填写面板地址、用户名和密码', false); return }
  busySave.value = true
  try { await api.setConfig({ ql_url: cfgUrl.value.trim(), ql_user: cfgUser.value.trim(), ql_pass: cfgPass.value.trim(), env_name: cfgEnv.value.trim() || 'JD_COOKIE', force_hours: Number(cfgForce.value) || 0, wx_token: cfgWxToken.value.trim(), wx_uid: cfgWxUID.value.trim() }); showToast('保存成功') } catch (e) { showToast(e.message, false) }
  busySave.value = false
}
async function doTest() {
  if (!cfgUrl.value || !cfgUser.value || !cfgPass.value) { showToast('请填写面板信息', false); return }
  busyTest.value = true
  try { await api.testQL({ ql_url: cfgUrl.value.trim(), ql_user: cfgUser.value.trim(), ql_pass: cfgPass.value.trim() }); showToast('连接成功') } catch (e) { showToast(e.message, false) }
  busyTest.value = false
}
async function doClearLog() {
  try { await api.clearLog(); logs.value = []; showToast('已清空') } catch (e) { showToast('清空失败', false) }
}
</script>

<template>
  <!-- Toast -->
  <div class="toast" :class="{ ok: toast.ok, err: !toast.ok, show: toast.show }">{{ toast.msg }}</div>

  <!-- App -->
  <div class="shell">
    <!-- Header -->
    <header class="bar">
      <div class="bar-left">
        <span class="dot"></span>
        <span class="bar-title">京东助手</span>
        <span class="bar-sub dim" v-if="!connected && connecting">connecting...</span>
        <span class="bar-sub dim" v-if="!connected && !connecting">offline</span>
      </div>
      <div class="bar-right" v-if="connected">
        <span class="bar-sub">已运行 {{ uptime }}</span>
      </div>
    </header>

    <!-- Content -->
    <div class="grid">
      <!-- Left -->
      <div class="panel">
        <!-- Cookie -->
        <section class="block">
          <div class="block-hd">Cookie</div>
          <div class="block-bd">
            <div v-if="connecting" class="loading"><span class="spinner"></span> connecting...</div>
            <div v-else-if="cookie && cookie.ok" class="kv-list">
              <div class="kv"><span class="kv-k">pt_key</span><span class="kv-v">{{ cookie.key }}</span></div>
              <div class="kv"><span class="kv-k">pt_pin</span><span class="kv-v">{{ cookie.pin }}</span></div>
            </div>
            <div v-else class="empty">读取按钮获取 Cookie</div>
          </div>
          <div class="block-info"><span>最后上传</span><span>{{ lastUpload || '从未上传' }}</span></div>
          <div class="block-ft">
            <div class="ft-left">
              <button class="btn primary" :disabled="!connected || busyRead" @click="doRead">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
                {{ busyRead ? '...' : '读取' }}
              </button>
            </div>
            <div class="ft-right">
              <button class="btn primary" :disabled="!connected || busyUploadOnly || !(cookie && cookie.ok) || !cfgUrl || !cfgUser || !cfgPass" @click="doUploadOnly">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
                {{ busyUploadOnly ? '...' : '上传' }}
              </button>
            </div>
          </div>
        </section>

        <!-- Config -->
        <section class="block">
          <div class="block-hd">青龙面板</div>
          <div class="block-bd form">
            <label>面板地址</label>
            <input v-model="cfgUrl" placeholder="http://192.168.1.100:5700">
            <div class="form-row">
              <div class="form-col"><label>用户名</label><input v-model="cfgUser" placeholder="admin"></div>
              <div class="form-col"><label>密码</label><input v-model="cfgPass" type="password" placeholder="密码"></div>
            </div>
            <label>变量名</label>
            <input v-model="cfgEnv" placeholder="JD_COOKIE">
            <label>强制重传周期（小时）</label>
            <input v-model="cfgForce" type="number" min="1" placeholder="0 = 默认 12 小时">
            <div class="form-hint">Cookie 超过该时长未变化时强制覆盖上传，规避 pt_key 过期却不重传</div>
            <div class="form-divider">Wxpusher 通知（可选）</div>
            <label>AppToken</label>
            <input v-model="cfgWxToken" placeholder="AT_xxxxxxxxxxxxx">
            <label>接收 UID（多个用逗号分隔）</label>
            <input v-model="cfgWxUID" placeholder="UID_xxxxxxxxxxxxx">
            <div class="form-hint">上传成功 / 失败 / 强制重传时推送消息</div>
          </div>
          <div class="block-ft">
            <div class="ft-left">
              <button class="btn" :disabled="!connected || busyTest" @click="doTest">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
                {{ busyTest ? '...' : '测试连接' }}
              </button>
            </div>
            <div class="ft-right">
              <button class="btn" :disabled="!connected || busySave" @click="doSave">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>
                {{ busySave ? '...' : '保存' }}
              </button>
            </div>
          </div>
        </section>
      </div>

      <!-- Right: Log -->
      <section class="block log-block">
        <div class="block-hd">日志<span class="count" v-if="logs.length">{{ logs.length }}</span>
          <span class="log-spacer"></span>
          <button class="log-clear-btn" @click="doClearLog">清除</button>
        </div>
        <div class="log-scroll">
          <table class="log-table">
            <tbody>
              <tr v-if="logs.length === 0"><td colspan="2" class="log-empty">暂无日志</td></tr>
              <tr v-for="(l, i) in logs" :key="i" class="log-tr">
                <td class="log-td-time">{{ l.t }}</td>
                <td class="log-td-msg">{{ l.m }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>

<style>
/* ===== Vars ===== */
:root {
  --bg:        #0b0d13;
  --bg2:       #11141c;
  --bg3:       #181b25;
  --brd:       #232733;
  --brd2:      #2e3344;
  --tx:        #e3e7ef;
  --tx2:       #8890a8;
  --tx3:       #565e79;
  --pri:       #6c6bf9;
  --pri2:      #8b8aff;
  --grn:       #2dd47c;
  --red:       #f54a4a;
  --amb:       #f09820;
  --rad:       10px;
}

/* ===== Reset ===== */
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
body {
  background: var(--bg); color: var(--tx); font-size:13px; line-height:1.5;
  font-family: 'PingFang SC', 'Microsoft YaHei', 'HarmonyOS Sans SC', system-ui, -apple-system, sans-serif;
  -webkit-font-smoothing: antialiased;
}

/* ===== Shell ===== */
.shell { max-width: 900px; margin: 0 auto; padding: 12px; }

/* ===== Bar ===== */
.bar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 16px; margin-bottom: 14px;
  background: var(--bg2); border: 1px solid var(--brd); border-radius: var(--rad);
}
.bar-left { display: flex; align-items: center; gap: 10px; }
.dot { width: 8px; height: 8px; border-radius: 50%; background: var(--grn); animation: breathe 2s ease-in-out infinite; }
@keyframes breathe { 0%,100% { opacity: 1; box-shadow: 0 0 4px var(--grn); } 50% { opacity: .4; box-shadow: 0 0 8px var(--grn); } }
.bar-title { font-weight: 700; font-size: 14px; letter-spacing: -.2px; }
.bar-sub { font-size: 11px; font-family: monospace; color: var(--tx2); }
.bar-sub.dim { color: var(--tx3); }
.bar-right { display: flex; align-items: center; }
.stat { font-size: 10px; font-family: monospace; color: var(--tx3); }

/* ===== Grid ===== */
.grid { display: grid; grid-template-columns: 1fr 1.2fr; gap: 14px; align-items: start; }
@media (max-width: 700px) { .grid { grid-template-columns: 1fr; } }

/* ===== Block ===== */
.block {
  background: var(--bg2); border: 1px solid var(--brd); border-radius: var(--rad);
  margin-bottom: 14px; overflow: hidden;
}
.block-hd {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 14px; font-size: 11px; font-weight: 700;
  text-transform: uppercase; letter-spacing: .6px; color: var(--tx3);
  border-bottom: 1px solid var(--brd);
}
.count { font-size: 10px; color: var(--tx3); background: var(--bg3); padding: 1px 7px; border-radius: 8px; }
.block-bd { padding: 14px; }
.block-ft {
  display: flex; align-items: center; gap: 10px; padding: 10px 14px;
  border-top: 1px solid var(--brd); justify-content: space-between;
  flex-wrap: wrap;
}

/* ===== KV ===== */
.kv-list { display: flex; flex-direction: column; gap: 10px; }
.kv { display: flex; flex-direction: column; gap: 3px; }
.kv-k {
  font-size: 10px; color: var(--pri2); font-weight: 600;
  text-transform: uppercase; letter-spacing: .4px; font-family: monospace;
}
.kv-v {
  font-size: 12px; color: var(--tx); background: var(--bg); padding: 7px 10px;
  border-radius: 6px; word-break: break-all; font-family: 'SF Mono', 'Fira Code', monospace;
  line-height: 1.5; border: 1px solid var(--brd);
}
.kv-v.sm { font-size: 10px; color: var(--tx2); }

/* ===== Button ===== */
.btn {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 7px 14px; border-radius: 7px; border: 1px solid var(--brd2);
  background: var(--bg3); color: var(--tx); font-size: 12px;
  cursor: pointer; font-family: inherit; transition: all .12s;
  white-space: nowrap;
}
.btn:hover { background: #202433; border-color: #3a4058; }
.btn:disabled { opacity: .35; cursor: default; }
.btn.primary { background: var(--pri); border-color: var(--pri); color: #fff; font-weight: 600; }
.btn.primary:hover { background: var(--pri2); border-color: var(--pri2); }
.btn svg { flex-shrink: 0; }
.ft-left { display: flex; gap: 8px; }
.ft-right { display: flex; gap: 8px; margin-left: auto; }
.block-info {
  display: flex; gap: 12px; padding: 8px 14px;
  border-top: 1px solid var(--brd); font-size: 11px;
}
.block-info span:first-child { color: var(--tx3); text-transform: uppercase; letter-spacing: 0.3px; font-weight: 600; }
.block-info span:last-child { color: var(--tx2); font-family: monospace; }

/* ===== Form ===== */
.form { display: flex; flex-direction: column; gap: 8px; }
.form label { font-size: 11px; color: var(--tx2); font-weight: 600; }
.form input, .form input:focus {
  width: 100%; padding: 9px 12px; border-radius: 7px; font-size: 13px;
  background: var(--bg); border: 1px solid var(--brd); color: var(--tx);
  outline: none; font-family: inherit;
}
.form input:focus { border-color: var(--pri); }
.form input::placeholder { color: var(--tx3); }
.form-row { display: flex; gap: 8px; }
.form-col { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.form-hint { font-size: 10px; color: var(--tx3); line-height: 1.4; margin-top: -4px; }
.form-divider {
  margin: 6px 0 2px; padding-top: 8px; border-top: 1px dashed var(--brd2);
  font-size: 11px; color: var(--tx2); font-weight: 700; text-transform: uppercase; letter-spacing: .4px;
}

/* ===== Loading / Empty ===== */
.loading { display: flex; align-items: center; gap: 8px; padding: 18px 0; color: var(--tx2); font-size: 12px; }
.spinner { width: 14px; height: 14px; border: 2px solid var(--brd); border-top-color: var(--pri); border-radius: 50%; animation: spin .6s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.empty { padding: 18px 0; color: var(--tx3); text-align: center; font-size: 13px; }

/* ===== Log ===== */
.log-block { display: flex; flex-direction: column; }
.log-block .block-bd { flex: 1; padding: 0; overflow: hidden; }
.log-scroll { height: calc(100vh - 200px); overflow: auto; }
.log-scroll::-webkit-scrollbar { width: 4px; height: 4px; }
.log-scroll::-webkit-scrollbar-thumb { background: var(--brd2); border-radius: 2px; }
.log-table {
  width: 100%; border-collapse: collapse;
  font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
  font-size: 12px; line-height: 1.6;
}
.log-tr td { padding: 1px 0; }
.log-tr:first-child td { padding-top: 8px; }
.log-tr:last-child td { padding-bottom: 8px; }
.log-table .log-td-time {
  width: 105px; white-space: nowrap; vertical-align: baseline;
  color: var(--tx3); font-size: 11px; text-align: right; font-variant-numeric: tabular-nums;
  padding-left: 5px; padding-right: 10px;
}
.log-td-msg { color: var(--tx2); font-size: 12px; white-space: nowrap; vertical-align: baseline; }
.log-empty { color: var(--tx3); padding: 30px 0; text-align: center; font-size: 12px; }

/* ===== Log Level ===== */
.log-spacer { flex: 1; }
.log-clear-btn {
  margin-left: 8px; padding: 1px 8px; border: 1px solid var(--brd2); border-radius: 4px;
  background: transparent; color: var(--tx3); font-size: 10px; cursor: pointer; font-family: inherit;
}
.log-clear-btn:hover { color: var(--red); border-color: var(--red); }

/* ===== Toast ===== */
.toast {
  position: fixed; top: 14px; left: 50%; transform: translateX(-50%);
  padding: 9px 22px; border-radius: 8px; color: #fff; font-size: 12px; font-weight: 600;
  z-index: 999; opacity: 0; pointer-events: none; transition: opacity .2s;
}
.toast.show { opacity: 1; }
.toast.ok { background: var(--grn); }
.toast.err { background: var(--red); }
</style>
