import { describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

const getMySubscriptions = vi.hoisted(() => vi.fn())
const getPlanCatalog = vi.hoisted(() => vi.fn())

vi.mock('@/api/subscriptions', () => ({
  default: { getMySubscriptions },
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: { getPlanCatalog },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null, showError: vi.fn() }),
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return { ...actual, useRouter: () => ({ push: vi.fn() }) }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'en' },
      t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

import SubscriptionsView from '../SubscriptionsView.vue'

describe('SubscriptionsView plan localization', () => {
  it('shows English plan metadata instead of internal group content', async () => {
    getMySubscriptions.mockResolvedValue([{
      id: 8,
      user_id: 1,
      group_id: 4,
      status: 'expired',
      starts_at: '2026-07-01T00:00:00Z',
      expires_at: '2026-08-01T00:00:00Z',
      daily_usage_usd: 0,
      weekly_usage_usd: 0,
      monthly_usage_usd: 0,
      daily_window_start: null,
      weekly_window_start: null,
      monthly_window_start: null,
      created_at: '2026-07-01T00:00:00Z',
      updated_at: '2026-08-01T00:00:00Z',
      group: {
        id: 4,
        name: 'codex-standard',
        description: '适合持续开发和日常项目使用',
        platform: 'openai',
        rate_multiplier: 1,
        daily_limit_usd: 30,
        weekly_limit_usd: 65,
        monthly_limit_usd: 65,
      },
    }])
    getPlanCatalog.mockResolvedValue({ data: [{
      id: 3,
      group_id: 4,
      name: 'Codex 标准版',
      name_en: 'Codex Standard',
      description: '适合持续开发和日常项目使用',
      description_en: 'For continuous development and everyday projects.',
      features: [],
      features_en: [],
    }] })

    const wrapper = shallowMount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Codex Standard')
    expect(wrapper.text()).toContain('For continuous development and everyday projects.')
    expect(wrapper.text()).not.toContain('codex-standard')
    expect(wrapper.text()).not.toContain('适合持续开发和日常项目使用')
  })
})
