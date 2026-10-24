package desktop

// trayPanelHTML 托盘速览面板：magpie 式「点击即答」的单页卡。
// 只做「看」和「去」——配置与操作留给教师台全功能页。
//
// 关键约束（别再退回 <a target="_blank">）：
// WebView2 对任何 target=_blank / window.open 都会走 NewWindowRequested，
// 而 Wails v3 beta.24 不接管它，于是系统默认浏览器（Edge）会弹出一个
// 带地址栏、带 Edge 图标的「假控制台」。桌面外壳内（?native=1）所有
// 「去」动作必须走 POST /ui/command，由 Go 侧打开原生窗口。
// 只有非外壳形态（浏览器里直接开 /tray）才允许 window.open 开新标签。
const trayPanelHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Forge · 监考速览</title>
<style>
  * { box-sizing: border-box; }
  body { margin: 0; background: #0b1520; color: #e6edf5;
         font-family: "Microsoft YaHei", sans-serif; font-size: 13px; }
  header { display: flex; align-items: center; justify-content: space-between;
           padding: 12px 14px; border-bottom: 1px solid rgba(255,255,255,.08); }
  header .brand { font-weight: 800; }
  header .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%;
                background: #34d399; margin-right: 6px; }
  main { padding: 12px 14px 64px; }
  .contest { border: 1px solid rgba(255,255,255,.1); border-radius: 14px; padding: 12px 14px;
             background: rgba(255,255,255,.04); margin-bottom: 12px; }
  .contest .name { font-weight: 700; font-size: 14px; }
  .contest .meta { color: #9db1c5; font-size: 12px; margin-top: 4px; }
  .countdown { font-family: Consolas, monospace; font-size: 22px; font-weight: 800; color: #7fb2ff; }
  .countdown.warn { color: #fbbf24; }
  .stats { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 12px; }
  .stat { border: 1px solid rgba(255,255,255,.1); border-radius: 12px; padding: 10px 12px;
          background: rgba(255,255,255,.04); }
  .stat b { display: block; font-size: 20px; font-family: Consolas, monospace; }
  .stat span { color: #9db1c5; font-size: 11px; }
  .alert { border-radius: 10px; padding: 8px 12px; margin-bottom: 12px; font-weight: 700;
           background: rgba(244,63,94,.12); color: #fda4af; border: 1px solid rgba(244,63,94,.3); }
  .actions { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 12px; }
  .actions button { display: block; width: 100%; text-align: center; padding: 9px 6px; font-size: 12px;
           font-weight: 700; color: #e6edf5; text-decoration: none; border-radius: 10px; cursor: pointer;
           font-family: inherit; background: rgba(255,255,255,.06); border: 1px solid rgba(255,255,255,.1); }
  .actions button:hover { background: rgba(255,255,255,.12); }
  .actions .primary { background: #14283d; border-color: #26415f; }
  .feed { border: 1px solid rgba(255,255,255,.1); border-radius: 12px;
          background: rgba(255,255,255,.04); }
  .feed .row { display: flex; justify-content: space-between; gap: 8px;
               padding: 7px 12px; border-bottom: 1px solid rgba(255,255,255,.05); font-size: 12px; }
  .feed .row:last-child { border-bottom: none; }
  .feed .who { font-weight: 700; }
  .v-accepted { color: #34d399; } .v-wrong_answer, .v-compile_error, .v-runtime_error { color: #fb7185; }
  .v-queued, .v-judging, .v-submitted { color: #7fb2ff; } .v-time_limit, .v-output_limit { color: #fbbf24; }
  .muted { color: #9db1c5; }
  footer { position: fixed; bottom: 0; left: 0; right: 0; display: flex; gap: 8px;
           justify-content: space-between; padding: 10px 14px;
           background: #0b1520; border-top: 1px solid rgba(255,255,255,.08); }
  footer button { color: #9db1c5; font-size: 12px; text-decoration: none; background: none;
           border: none; cursor: pointer; font-family: inherit; }
  footer button.danger:hover { color: #fb7185; }
  .login { text-align: center; padding: 40px 16px; }
  .login button { color: #7fb2ff; background: none; border: none; padding: 0; cursor: pointer;
                  font: inherit; text-decoration: underline; }
  .toast { position: fixed; left: 14px; right: 14px; bottom: 58px; padding: 8px 12px;
           border-radius: 10px; background: rgba(244,63,94,.16); color: #fecdd3;
           border: 1px solid rgba(244,63,94,.35); font-size: 12px; font-weight: 700;
           opacity: 0; transition: opacity .18s; pointer-events: none; }
  .toast.show { opacity: 1; }
</style>
</head>
<body>
<header>
  <span class="brand"><span class="dot" id="liveDot"></span>Forge 监考速览</span>
  <span class="muted" id="updatedAt"></span>
</header>
<main id="main">
  <div class="login" id="loginHint" hidden>
    尚未登录教师账号。<button type="button" data-act="show-main">打开教师台登录</button> 后此面板自动出现考场数据。
  </div>
  <div id="content" hidden>
    <div class="contest" id="contestCard"></div>
    <div id="alertBox"></div>
    <div class="stats">
      <div class="stat"><b id="stOnline">-</b><span>在线 / 参赛</span></div>
      <div class="stat"><b id="stSubs">-</b><span>累计提交</span></div>
      <div class="stat"><b id="stQueue">-</b><span>待评测</span></div>
      <div class="stat"><b id="stConflicts">-</b><span>机位冲突</span></div>
    </div>
    <div class="actions">
      <button type="button" class="primary" data-act="open-bigscreen">监考大屏</button>
      <button type="button" data-act="open-join">学生扫码加入</button>
      <button type="button" data-act="show-main">打开教师台</button>
      <button type="button" data-act="open-student">学生大厅</button>
    </div>
    <div class="feed" id="feed"></div>
  </div>
</main>
<footer>
  <span class="muted" id="serverAddr"></span>
  <span>
    <button type="button" data-act="show-main">打开教师台</button>
    <button type="button" class="danger" id="quitBtn">退出服务</button>
  </span>
</footer>
<div class="toast" id="toast" role="status"></div>
<script>
const $ = (id) => document.getElementById(id)
// 桌面外壳加载 /tray?native=1；浏览器里直接开 /tray 则是普通网页形态。
const NATIVE = /(?:^|[?&])native=1(?:&|$)/.test(location.search)
const ACTION_URL = {
  'show-main': '/teacher',
  'open-bigscreen': '/teacher?tab=monitor',
  'open-join': '/join',
  'open-student': '/app',
}
let toastTimer = null
function toast(msg) {
  const el = $('toast')
  if (!el) return
  el.textContent = msg
  el.classList.add('show')
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => el.classList.remove('show'), 2800)
}
function actionForURL(href) {
  let u
  try { u = new URL(href, location.href) } catch (e) { return 'show-main' }
  if (u.pathname === '/join') return 'open-join'
  if (u.searchParams.get('tab') === 'monitor') return 'open-bigscreen'
  if (u.pathname === '/app' || u.pathname === '/app/' || u.pathname.indexOf('/contest/') === 0) return 'open-student'
  return 'show-main'
}
async function act(action) {
  const url = ACTION_URL[action] || '/teacher'
  if (!NATIVE) { window.open(url, '_blank'); return }
  try {
    const r = await fetch('/ui/command', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: action }),
    })
    if (r.ok) return
    if (r.status === 401 || r.status === 403) { toast('请先在教师台登录教师账号'); return }
    if (r.status === 503) { toast('桌面外壳未就绪，请重启 Forge'); return }
    toast('窗口指令未生效（HTTP ' + r.status + '）')
  } catch (e) {
    toast('窗口指令发送失败：' + (e && e.message ? e.message : e))
  }
}
document.addEventListener('click', (e) => {
  const el = e.target && e.target.closest ? e.target.closest('[data-act]') : null
  if (!el) return
  e.preventDefault()
  act(el.getAttribute('data-act'))
})
if (NATIVE) {
  // 桌面外壳内的总闸：任何遗留的 window.open / <a target=_blank> 一律改走
  // 原生窗口指令，绝不让 WebView2 把页面交给系统浏览器。
  window.open = function (href) { act(actionForURL(String(href || ''))); return null }
  document.addEventListener('click', (e) => {
    const a = e.target && e.target.closest ? e.target.closest('a[href]') : null
    if (!a) return
    e.preventDefault()
    act(actionForURL(a.getAttribute('href')))
  })
}
const verdictLabel = {
  accepted: '通过', wrong_answer: '答案错误', compile_error: '编译错误',
  runtime_error: '运行错误', time_limit: '超时', memory_limit: '超内存',
  output_limit: '输出超限', checker_error: 'SPJ 判错', queued: '排队中',
  judging: '评测中', submitted: '已提交', skipped: '跳过',
}
function relTime(iso) {
  const d = Date.now() - new Date(iso).getTime()
  if (d < 30_000) return '刚刚'
  if (d < 3_600_000) return Math.round(d / 60_000) + ' 分钟前'
  return new Date(iso).toLocaleTimeString()
}
function fmtCountdown(ms) {
  if (ms < 0) ms = 0
  const h = Math.floor(ms / 3_600_000), m = Math.floor(ms % 3_600_000 / 60_000), s = Math.floor(ms % 60_000 / 1000)
  return (h > 0 ? h + ':' : '') + String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0')
}
let lastEndsAt = null
function render(d) {
  const authed = !d.error
  $('loginHint').hidden = authed
  $('content').hidden = !authed
  if (!authed) return
  const c = d.contest
  if (!c) {
    $('contestCard').innerHTML = '<div class="name">当前没有进行中的比赛</div>' +
      '<div class="meta">在教师台创建并启动一场比赛后，这里会实时显示考场状态。</div>'
    $('feed').innerHTML = ''
    $('alertBox').innerHTML = ''
    lastEndsAt = null
  } else {
    const modeText = { online: '在线提交', noi: 'NOI 全真模拟', gesp: 'GESP 机考' }[c.mode] || c.mode
    const statusText = { running: '进行中', draft: '未开始', finished: '已结束' }[c.status] || c.status
    $('contestCard').innerHTML =
      '<div class="name">' + c.name + ' <span class="muted">· ' + modeText + ' · ' + statusText + '</span></div>' +
      '<div class="meta">剩余时间：<span class="countdown" id="cd">—</span></div>'
    lastEndsAt = (c.status === 'running' && c.windowEndsAt) ? new Date(c.windowEndsAt).getTime() : null
    tick()
  }
  $('stOnline').textContent = d.online + ' / ' + d.participants
  $('stSubs').textContent = d.submissions
  $('stQueue').textContent = d.queued
  $('stConflicts').textContent = d.conflicts
  const alerts = []
  if (d.conflicts > 0) alerts.push(d.conflicts + ' 个机位 IP 冲突——请核对监考大屏')
  if (d.queued > 20) alerts.push('评测队列积压 ' + d.queued + ' 题')
  $('alertBox').innerHTML = alerts.map((a) => '<div class="alert">⚠ ' + a + '</div>').join('')
  $('feed').innerHTML = (d.recent || []).map((e) =>
    '<div class="row"><span><span class="who">' + (e.userName || '考生') + '</span> · ' +
    (e.problemCode || '—') + '</span>' +
    '<span class="v-' + e.verdict + '">' + (verdictLabel[e.verdict] || e.verdict) +
    ' <span class="muted">' + relTime(e.submittedAt) + '</span></span></div>'
  ).join('') || '<div class="row muted">暂无提交动态</div>'
  $('updatedAt').textContent = '更新于 ' + new Date().toLocaleTimeString()
}
function tick() {
  const el = document.getElementById('cd')
  if (!el || lastEndsAt == null) return
  const ms = lastEndsAt - Date.now()
  el.textContent = fmtCountdown(ms)
  el.className = 'countdown' + (ms < 5 * 60_000 ? ' warn' : '')
}
async function refresh() {
  try {
    const r = await fetch('/api/v1/tray-summary')
    if (r.status === 401) { render({ error: 'unauthorized' }); return }
    render(await r.json())
  } catch (e) {
    $('liveDot').style.background = '#fb7185'
  }
}
$('quitBtn').addEventListener('click', async () => {
  if (!confirm('确定退出 Forge 服务？考生将无法继续作答。')) return
  await fetch('/api/v1/shutdown', { method: 'POST' })
  document.body.innerHTML = '<div class="login"><h1>服务已停止</h1><p class="muted">可以关闭本窗口。重新启动：forge-desktop.exe</p></div>'
})
$('serverAddr').textContent = '服务地址 ' + location.host
setInterval(tick, 1000)
setInterval(refresh, 5000)
refresh()
</script>
</body>
</html>`
