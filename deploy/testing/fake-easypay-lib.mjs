import { createHash, timingSafeEqual } from 'node:crypto'

export function easyPaySign(params, secret) {
  const body = Object.entries(params)
    .filter(([key, value]) => key !== 'sign' && key !== 'sign_type' && value !== '')
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join('&')

  return createHash('md5').update(body + secret).digest('hex')
}

export function verifyEasyPaySign(params, secret) {
  const actual = Buffer.from(String(params.sign || ''))
  const expected = Buffer.from(easyPaySign(params, secret))
  return actual.length === expected.length && timingSafeEqual(actual, expected)
}

export function buildPaymentNotification(order, tradeNo, secret) {
  const notification = {
    pid: order.pid,
    trade_no: tradeNo,
    out_trade_no: order.out_trade_no,
    type: order.type,
    name: order.name,
    money: order.money,
    trade_status: 'TRADE_SUCCESS',
  }
  notification.sign = easyPaySign(notification, secret)
  notification.sign_type = 'MD5'
  return notification
}
