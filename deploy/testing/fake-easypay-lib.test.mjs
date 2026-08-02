import test from 'node:test'
import assert from 'node:assert/strict'
import { buildPaymentNotification, easyPaySign, verifyEasyPaySign } from './fake-easypay-lib.mjs'

test('EasyPay signature is stable and excludes signature fields', () => {
  const params = { money: '9.90', pid: 'demo', out_trade_no: 'order-1', sign_type: 'MD5' }
  assert.equal(easyPaySign(params, 'secret'), '05304e2a6eb38ea154b2a94a1f054012')
  assert.equal(verifyEasyPaySign({ ...params, sign: easyPaySign(params, 'secret') }, 'secret'), true)
})

test('tampered payment notification is rejected', () => {
  const notification = buildPaymentNotification({
    pid: 'demo', type: 'alipay', name: 'plan', money: '9.90', out_trade_no: 'order-1',
  }, 'trade-1', 'secret')
  assert.equal(verifyEasyPaySign(notification, 'secret'), true)
  assert.equal(verifyEasyPaySign({ ...notification, money: '0.01' }, 'secret'), false)
})
