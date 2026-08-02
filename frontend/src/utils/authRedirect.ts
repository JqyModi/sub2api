export function resolveAuthRedirect(value: unknown, fallback = '/dashboard'): string {
  const candidate = Array.isArray(value) ? value[0] : value
  if (typeof candidate !== 'string') return fallback

  const normalized = candidate.trim()
  if (!normalized.startsWith('/') || normalized.startsWith('//') || normalized.includes('\\')) {
    return fallback
  }

  return normalized
}

export function resolveDesktopAuthorizationRedirect(value: unknown): string | null {
  const redirect = resolveAuthRedirect(value, '')
  if (!redirect) return null

  const url = new URL(redirect, 'https://desktop-auth.local')
  if (url.pathname !== '/desktop/authorize' || !url.searchParams.get('session')?.trim()) {
    return null
  }

  return `${url.pathname}${url.search}${url.hash}`
}

export function buildDesktopSubscriptionPurchaseRedirect(value: unknown): string | null {
  const authorizationRedirect = resolveDesktopAuthorizationRedirect(value)
  if (!authorizationRedirect) return null

  const query = new URLSearchParams({
    tab: 'subscription',
    redirect: authorizationRedirect,
  })
  return `/purchase?${query.toString()}`
}
