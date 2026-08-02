export function resolveAuthRedirect(value: unknown, fallback = '/dashboard'): string {
  const candidate = Array.isArray(value) ? value[0] : value
  if (typeof candidate !== 'string') return fallback

  const normalized = candidate.trim()
  if (!normalized.startsWith('/') || normalized.startsWith('//') || normalized.includes('\\')) {
    return fallback
  }

  return normalized
}
