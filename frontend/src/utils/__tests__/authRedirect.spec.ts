import { describe, expect, it } from 'vitest'
import { resolveAuthRedirect } from '../authRedirect'

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
})
