import { describe, expect, it } from 'vitest'
import {
  buildDesktopSubscriptionPurchaseRedirect,
  resolveAuthRedirect,
  resolveDesktopAuthorizationRedirect,
  shouldUseSameWindowForPayment,
} from '../authRedirect'

describe('resolveAuthRedirect', () => {
  it('preserves a desktop authorization path and query', () => {
    expect(resolveAuthRedirect('/desktop/authorize?session=dsa_test')).toBe(
      '/desktop/authorize?session=dsa_test'
    )
  })

  it.each([
    'https://evil.example/steal',
    '//evil.example/steal',
    '/\\evil.example/steal',
    '',
  ])('rejects unsafe redirect %s', (value) => {
    expect(resolveAuthRedirect(value)).toBe('/dashboard')
  })

  it('uses the first route query value', () => {
    expect(resolveAuthRedirect(['/profile', '/dashboard'])).toBe('/profile')
  })

  it('recognizes a valid desktop authorization return path', () => {
    expect(resolveDesktopAuthorizationRedirect('/desktop/authorize?session=dsa_test')).toBe(
      '/desktop/authorize?session=dsa_test'
    )
  })

  it.each([
    '/desktop/authorize',
    '/desktop/authorize?session=',
    '/dashboard?session=dsa_test',
    'https://evil.example/desktop/authorize?session=dsa_test',
  ])('rejects a non-desktop authorization return %s', (value) => {
    expect(resolveDesktopAuthorizationRedirect(value)).toBeNull()
  })

  it('builds a subscription-first purchase path that retains the authorization session', () => {
    expect(buildDesktopSubscriptionPurchaseRedirect('/desktop/authorize?session=dsa_test')).toBe(
      '/purchase?tab=subscription&redirect=%2Fdesktop%2Fauthorize%3Fsession%3Ddsa_test'
    )
  })

  it('keeps desktop authorization and mobile payments in the current window', () => {
    expect(shouldUseSameWindowForPayment('/desktop/authorize?session=dsa_test', false)).toBe(true)
    expect(shouldUseSameWindowForPayment(undefined, true)).toBe(true)
    expect(shouldUseSameWindowForPayment(undefined, false)).toBe(false)
    expect(shouldUseSameWindowForPayment('https://evil.example/steal', false)).toBe(false)
  })
})
