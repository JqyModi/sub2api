import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { ref } from 'vue'

const approveSession = vi.hoisted(() => vi.fn())
const routerReplace = vi.hoisted(() => vi.fn())
const routeState = vi.hoisted(() => ({
  query: { session: 'dsa_component_test' } as Record<string, unknown>,
  fullPath: '/desktop/authorize?session=dsa_component_test',
}))

vi.mock('@/api', () => ({
  apiClient: {
    post: approveSession,
  },
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeState,
    useRouter: () => ({ replace: routerReplace }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ locale: ref('zh-CN') }),
  }
})

import DesktopAuthorizationView from '../DesktopAuthorizationView.vue'

describe('DesktopAuthorizationView', () => {
  beforeEach(() => {
    approveSession.mockReset()
    routerReplace.mockReset().mockResolvedValue(undefined)
  })

  it('checks authorization automatically and sends unsubscribed users straight to subscription plans', async () => {
    approveSession.mockResolvedValue({ data: { state: 'payment_required' } })

    const wrapper = shallowMount(DesktopAuthorizationView, {
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    expect(approveSession).toHaveBeenCalledWith(
      '/desktop-auth/sessions/dsa_component_test/approve',
      {}
    )
    expect(routerReplace).toHaveBeenCalledWith(
      '/purchase?tab=subscription&redirect=%2Fdesktop%2Fauthorize%3Fsession%3Ddsa_component_test'
    )
    wrapper.unmount()
  })

  it('asks the user to choose when multiple active subscriptions exist', async () => {
    approveSession
      .mockResolvedValueOnce({ data: { state: 'selection_required', subscriptions: [
        { id: 20, group_id: 4, group_name: 'Codex 标准版', expires_at: '2026-09-10T00:00:00Z' },
        { id: 10, group_id: 3, group_name: 'Codex 轻量版', expires_at: '2026-09-01T00:00:00Z' },
      ] } })
      .mockResolvedValueOnce({ data: { state: 'authorized' } })

    const wrapper = shallowMount(DesktopAuthorizationView, {
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('选择此 Profile 使用的套餐')
    expect(wrapper.text()).toContain('Codex 标准版')
    const confirm = wrapper.findAll('button').find(button => button.text().includes('确认接入'))
    expect(confirm).toBeTruthy()
    await confirm!.trigger('click')
    await flushPromises()

    expect(approveSession).toHaveBeenLastCalledWith(
      '/desktop-auth/sessions/dsa_component_test/approve',
      { subscription_id: 20 }
    )
    expect(wrapper.text()).toContain('接入成功')
    wrapper.unmount()
  })

  it('completes an active subscription without showing the purchase page', async () => {
    approveSession.mockResolvedValue({ data: { state: 'authorized' } })

    const wrapper = shallowMount(DesktopAuthorizationView, {
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('接入成功')
    expect(routerReplace).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
