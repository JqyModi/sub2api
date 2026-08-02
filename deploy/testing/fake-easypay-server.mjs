import http from 'node:http'
import { randomBytes } from 'node:crypto'
import { URL, URLSearchParams } from 'node:url'
import { buildPaymentNotification, verifyEasyPaySign } from './fake-easypay-lib.mjs'

const host = process.env.HOST || '0.0.0.0'
const port = Number(process.env.PORT || 8090)
const merchantID = process.env.EASYPAY_PID || 'desktop-auth-local'
const merchantSecret = process.env.EASYPAY_PKEY || 'desktop-auth-local-secret'
const publicOrigin = process.env.PUBLIC_ORIGIN || `http://127.0.0.1:${port}`
const orders = new Map()

function send(res, status, body, contentType = 'text/plain; charset=utf-8') {
  res.writeHead(status, { 'content-type': contentType, 'cache-control': 'no-store' })
  res.end(body)
}

function sendJSON(res, status, value) {
  send(res, status, JSON.stringify(value), 'application/json; charset=utf-8')
}

function escapeHTML(value) {
  return String(value || '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

async function parseForm(req) {
  const chunks = []
  for await (const chunk of req) chunks.push(chunk)
  return Object.fromEntries(new URLSearchParams(Buffer.concat(chunks).toString('utf8')))
}

function renderPaymentPage(order, state = 'pending', detail = '') {
  const success = state === 'paid'
  return `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>本地模拟支付</title><style>
body{margin:0;background:#f5f7fa;color:#18202a;font:15px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
main{max-width:520px;margin:10vh auto;padding:32px;background:#fff;border:1px solid #e2e7ec;border-radius:12px;box-shadow:0 12px 32px rgba(25,38,52,.08)}
h1{margin:0 0 8px;font-size:22px}dl{display:grid;grid-template-columns:90px 1fr;gap:8px;margin:24px 0}dt{color:#6a7480}dd{margin:0;word-break:break-all}
button{width:100%;border:0;border-radius:8px;padding:12px;background:#1677ff;color:#fff;font-size:15px;cursor:pointer}button:disabled{background:#9bbce8}.note{color:#68727e;font-size:13px}.ok{color:#16834b}
</style></head><body><main><h1>${success ? '支付已完成' : '本地模拟支付'}</h1>
<p class="${success ? 'ok' : 'note'}">${success ? '真实 Webhook 已通过验签并完成订阅履约。' : '仅用于本地验收，不会发生真实扣款。'}</p>
<dl><dt>商品</dt><dd>${escapeHTML(order.name)}</dd><dt>金额</dt><dd>¥${escapeHTML(order.money)}</dd><dt>订单号</dt><dd>${escapeHTML(order.out_trade_no)}</dd></dl>
${success ? `<p>${escapeHTML(detail)}</p><script>setTimeout(()=>window.close(),1800)</script>` : `<form method="post" action="/pay"><input type="hidden" name="out_trade_no" value="${escapeHTML(order.out_trade_no)}"><button type="submit">模拟支付成功</button></form>`}
</main></body></html>`
}

async function createPayment(req, res) {
  const params = await parseForm(req)
  if (params.pid !== merchantID || !verifyEasyPaySign(params, merchantSecret)) {
    sendJSON(res, 400, { code: -1, msg: 'merchant or signature verification failed' })
    return
  }
  if (!params.out_trade_no || !params.notify_url || !params.money) {
    sendJSON(res, 400, { code: -1, msg: 'missing required payment fields' })
    return
  }

  const tradeNo = `FAKE${Date.now()}${randomBytes(3).toString('hex')}`
  orders.set(params.out_trade_no, { ...params, tradeNo, paid: false })
  sendJSON(res, 200, {
    code: 1,
    msg: 'success',
    trade_no: tradeNo,
    payurl: `${publicOrigin}/pay?out_trade_no=${encodeURIComponent(params.out_trade_no)}`,
  })
}

async function completePayment(req, res) {
  const params = await parseForm(req)
  const order = orders.get(params.out_trade_no)
  if (!order) {
    send(res, 404, 'order not found')
    return
  }
  if (order.paid) {
    send(res, 200, renderPaymentPage(order, 'paid', '该订单已经完成。'), 'text/html; charset=utf-8')
    return
  }

  const notification = buildPaymentNotification(order, order.tradeNo, merchantSecret)
  const callback = await fetch(order.notify_url, {
    method: 'POST',
    headers: { 'content-type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams(notification),
  })
  const callbackBody = await callback.text()
  if (!callback.ok || callbackBody.trim().toLowerCase() !== 'success') {
    send(res, 502, renderPaymentPage(order, 'failed', `Webhook failed: ${callback.status} ${callbackBody}`), 'text/html; charset=utf-8')
    return
  }

  order.paid = true
  send(res, 200, renderPaymentPage(order, 'paid', '可以返回授权页面，桌面端将自动继续。'), 'text/html; charset=utf-8')
}

const server = http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url, publicOrigin)
    if (req.method === 'GET' && url.pathname === '/health') return send(res, 200, 'ok')
    if (req.method === 'POST' && url.pathname === '/mapi.php') return await createPayment(req, res)
    if (req.method === 'GET' && url.pathname === '/pay') {
      const order = orders.get(url.searchParams.get('out_trade_no'))
      return order
        ? send(res, 200, renderPaymentPage(order, order.paid ? 'paid' : 'pending'), 'text/html; charset=utf-8')
        : send(res, 404, 'order not found')
    }
    if (req.method === 'POST' && url.pathname === '/pay') return await completePayment(req, res)
    return send(res, 404, 'not found')
  } catch (error) {
    console.error(error)
    return send(res, 500, 'internal error')
  }
})

server.listen(port, host, () => console.log(`fake-easypay listening on ${host}:${port}`))
