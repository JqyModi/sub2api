import { createHash, randomBytes } from 'node:crypto'

const baseURL = (process.env.SUB2API_URL || 'http://127.0.0.1:8080').replace(/\/$/, '')

async function rawRequest(path, options = {}) {
  return fetch(path.startsWith('http') ? path : `${baseURL}/api/v1${path}`, {
    ...options,
    headers: {
      ...(options.body && typeof options.body === 'string' ? { 'content-type': 'application/json' } : {}),
      ...(options.token ? { authorization: `Bearer ${options.token}` } : {}),
      ...options.headers,
    },
  })
}

async function request(path, options = {}) {
  const response = await rawRequest(path, options)
  const body = await response.json().catch(() => null)
  if (!response.ok || body?.code !== 0) {
    throw new Error(`${options.method || 'GET'} ${path} failed (${response.status}): ${JSON.stringify(body)}`)
  }
  return body.data
}

const suffix = `${Date.now()}-${randomBytes(2).toString('hex')}`
const email = `desktop-purchase-${suffix}@example.test`
const password = `Local-${randomBytes(8).toString('hex')}!`
const auth = await request('/auth/register', {
  method: 'POST',
  body: JSON.stringify({ email, password }),
})
const token = auth.access_token

const verifier = randomBytes(32).toString('base64url')
const challenge = createHash('sha256').update(verifier).digest('base64url')
const session = await request('/desktop-auth/sessions', {
  method: 'POST',
  body: JSON.stringify({
    client_id: 'codex-multi-launcher',
    code_challenge: challenge,
    device_name: 'Automated Purchase Verification',
  }),
})

const firstApprove = await rawRequest(`/desktop-auth/sessions/${session.session_id}/approve`, {
  method: 'POST', token, body: '{}',
})
const firstApproveBody = await firstApprove.json()
if (!firstApprove.ok || firstApproveBody?.data?.state !== 'payment_required') {
  throw new Error(`new user should require payment: ${firstApprove.status} ${JSON.stringify(firstApproveBody)}`)
}

const checkout = await request('/payment/checkout-info', { token })
const plan = checkout.plans.find((item) => item.name === 'Desktop Local Monthly')
if (!plan) throw new Error('local desktop subscription plan is not available')

const order = await request('/payment/orders', {
  method: 'POST', token,
  body: JSON.stringify({
    amount: plan.price,
    payment_type: 'alipay',
    order_type: 'subscription',
    plan_id: plan.id,
    payment_source: 'hosted_redirect',
    return_url: `${baseURL}/payment/result`,
    is_mobile: false,
  }),
})
if (!order.pay_url || !order.out_trade_no) throw new Error(`payment order has no redirect URL: ${JSON.stringify(order)}`)

const invalidWebhook = await fetch(`${baseURL}/api/v1/payment/webhook/easypay`, {
  method: 'POST',
  headers: { 'content-type': 'application/x-www-form-urlencoded' },
  body: new URLSearchParams({
    pid: 'desktop-auth-local',
    trade_no: `INVALID-${suffix}`,
    out_trade_no: order.out_trade_no,
    money: String(order.pay_amount),
    trade_status: 'TRADE_SUCCESS',
    sign: 'invalid-signature',
    sign_type: 'MD5',
  }),
})
if (invalidWebhook.status !== 400) {
  throw new Error(`invalid EasyPay signature should be rejected, got ${invalidWebhook.status}`)
}

const payURL = new URL(order.pay_url)
const paymentResult = await fetch(`${payURL.origin}/pay`, {
  method: 'POST',
  headers: { 'content-type': 'application/x-www-form-urlencoded' },
  body: new URLSearchParams({ out_trade_no: order.out_trade_no }),
})
const paymentResultBody = await paymentResult.text()
if (!paymentResult.ok || !paymentResultBody.includes('支付已完成')) {
  throw new Error(`fake payment did not complete (${paymentResult.status})`)
}

let authorized
for (let attempt = 0; attempt < 10; attempt += 1) {
  const response = await rawRequest(`/desktop-auth/sessions/${session.session_id}/approve`, {
    method: 'POST', token, body: '{}',
  })
  const body = await response.json()
  if (response.ok && body?.data?.state === 'authorized') {
    authorized = body.data
    break
  }
  await new Promise((resolve) => setTimeout(resolve, 250))
}
if (!authorized) throw new Error('desktop authorization did not continue after payment fulfillment')

const desktopToken = await request('/desktop-auth/token', {
  method: 'POST',
  body: JSON.stringify({ session_id: session.session_id, code_verifier: verifier }),
})
if (!desktopToken.access_token || desktopToken.base_url !== `${baseURL}/v1`) {
  throw new Error(`desktop token response is incomplete: ${JSON.stringify(desktopToken)}`)
}

const secondExchange = await rawRequest('/desktop-auth/token', {
  method: 'POST', body: JSON.stringify({ session_id: session.session_id, code_verifier: verifier }),
})
if (secondExchange.ok) throw new Error('desktop authorization session was exchanged more than once')

const orders = await request('/payment/orders/my?page=1&page_size=10', { token })
const completedOrder = orders.items?.find((item) => item.out_trade_no === order.out_trade_no)
if (!completedOrder || completedOrder.status.toLowerCase() !== 'completed') {
  throw new Error(`subscription order is not completed: ${JSON.stringify(completedOrder)}`)
}

const subscriptions = await request('/subscriptions/active', { token })
if (!Array.isArray(subscriptions) || subscriptions.length === 0) {
  throw new Error('payment fulfillment did not create an active subscription')
}
const keys = await request('/keys?page=1&page_size=100', { token })
const desktopKey = keys.items?.find((item) => String(item.name || '').startsWith('Codex Multi Launcher -'))
if (!desktopKey) throw new Error('desktop authorization did not create a device API key')

console.log(JSON.stringify({
  ok: true,
  user: email,
  order: order.out_trade_no,
  orderStatus: completedOrder.status,
  activeSubscription: subscriptions[0].id,
  desktopKey: desktopKey.name,
  invalidSignatureRejected: true,
  authorizationState: authorized.state,
  baseURL: desktopToken.base_url,
  provider: desktopToken.provider_name,
  singleUseExchange: true,
}, null, 2))
