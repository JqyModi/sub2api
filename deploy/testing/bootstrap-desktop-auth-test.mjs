const baseURL = (process.env.SUB2API_URL || 'http://127.0.0.1:8080').replace(/\/$/, '')
const adminEmail = process.env.ADMIN_EMAIL || 'admin@sub2api.local'
const adminPassword = process.env.ADMIN_PASSWORD || 'desktop-auth-local-admin'

async function request(path, options = {}) {
  const response = await fetch(`${baseURL}/api/v1${path}`, {
    ...options,
    headers: {
      'content-type': 'application/json',
      ...(options.token ? { authorization: `Bearer ${options.token}` } : {}),
      ...options.headers,
    },
  })
  const body = await response.json().catch(() => null)
  if (!response.ok || body?.code !== 0) {
    throw new Error(`${options.method || 'GET'} ${path} failed (${response.status}): ${JSON.stringify(body)}`)
  }
  return body.data
}

const auth = await request('/auth/login', {
  method: 'POST',
  body: JSON.stringify({ email: adminEmail, password: adminPassword }),
})
const token = auth.access_token

await request('/admin/settings', {
  method: 'PUT', token,
  body: JSON.stringify({
    registration_enabled: true,
    purchase_subscription_enabled: true,
    purchase_subscription_url: `${baseURL}/purchase`,
  }),
})

await request('/admin/payment/config', {
  method: 'PUT', token,
  body: JSON.stringify({
    enabled: true,
    enabled_payment_types: ['alipay'],
    payment_visible_method_alipay_enabled: true,
    payment_visible_method_alipay_source: 'easypay',
  }),
})

const groups = await request('/admin/groups/all', { token })
let group = groups.find((item) => item.name === 'Desktop Local Test' && item.subscription_type === 'subscription')
if (!group) {
  group = await request('/admin/groups', {
    method: 'POST', token,
    body: JSON.stringify({
      name: 'Desktop Local Test',
      description: 'Local desktop authorization purchase test group',
      platform: 'openai',
      rate_multiplier: 1,
      subscription_type: 'subscription',
      monthly_limit_usd: 10,
      models_list_config: { enabled: true, models: ['gpt-5.4'] },
      supported_model_scopes: ['codex'],
    }),
  })
}

const providers = await request('/admin/payment/providers', { token })
const providerPayload = {
  name: 'Desktop Local Fake EasyPay',
  config: {
    pid: 'desktop-auth-local',
    pkey: 'desktop-auth-local-secret',
    apiBase: 'http://fake-easypay:8090',
    notifyUrl: 'http://sub2api:8080/api/v1/payment/webhook/easypay',
    returnUrl: 'http://127.0.0.1:8080/payment/result',
  },
  supported_types: ['alipay'],
  enabled: true,
  payment_mode: 'qrcode',
  sort_order: 0,
  limits: JSON.stringify({
    alipay: { currency: 'CNY', fee_rate: 0, daily_limit: 0, single_min: 0.01, single_max: 10000 },
  }),
  refund_enabled: false,
  allow_user_refund: false,
}
const existingProvider = providers.find((item) => item.name === providerPayload.name)
if (existingProvider) {
  await request(`/admin/payment/providers/${existingProvider.id}`, {
    method: 'PUT', token, body: JSON.stringify(providerPayload),
  })
} else {
  await request('/admin/payment/providers', {
    method: 'POST', token,
    body: JSON.stringify({ provider_key: 'easypay', ...providerPayload }),
  })
}

const plans = await request('/admin/payment/plans', { token })
const planPayload = {
  group_id: group.id,
  name: 'Desktop Local Monthly',
  description: 'Local end-to-end purchase verification plan',
  price: 0.01,
  currency: 'CNY',
  validity_days: 1,
  validity_unit: 'month',
  features: '桌面端一键接入\n本地购买闭环验收',
  product_name: 'Codex Multi Launcher Local Subscription',
  for_sale: true,
  sort_order: 0,
}
const existingPlan = plans.find((item) => item.name === planPayload.name)
if (existingPlan) {
  await request(`/admin/payment/plans/${existingPlan.id}`, {
    method: 'PUT', token, body: JSON.stringify(planPayload),
  })
} else {
  await request('/admin/payment/plans', {
    method: 'POST', token, body: JSON.stringify(planPayload),
  })
}

const settings = await request('/settings/public', {})
const checkout = await request('/payment/checkout-info', { token })
if (!settings.registration_enabled || !checkout.plans?.some((item) => item.name === planPayload.name) || !checkout.methods?.alipay) {
  throw new Error('desktop authorization purchase test configuration did not become visible')
}

console.log(JSON.stringify({
  ready: true,
  registration: settings.registration_enabled,
  group: group.name,
  plan: planPayload.name,
  paymentMethod: 'alipay',
  purchaseURL: `${baseURL}/purchase`,
}, null, 2))
