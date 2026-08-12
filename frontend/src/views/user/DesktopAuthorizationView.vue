<template>
  <div class="mx-auto flex min-h-[70vh] max-w-xl items-center justify-center px-4 py-10">
    <section class="w-full rounded-2xl border border-gray-200 bg-white p-7 shadow-sm dark:border-dark-700 dark:bg-dark-900">
      <div class="mb-6 flex items-start gap-4">
        <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
          <Icon name="infoCircle" size="lg" />
        </div>
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ copy.title }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ copy.subtitle }}</p>
        </div>
      </div>

      <div v-if="isLoading" class="rounded-xl bg-gray-50 p-4 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-200">
        {{ copy.loading }}
      </div>
      <div v-else-if="state === 'authorized'" class="rounded-xl border border-green-200 bg-green-50 p-4 text-sm text-green-700 dark:border-green-800/50 dark:bg-green-900/20 dark:text-green-300">
        <p class="font-medium">{{ copy.success }}</p>
        <p class="mt-1">{{ copy.successDetail }}</p>
      </div>
      <div v-else-if="state === 'payment_required'" class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-700 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-300">
        <p class="font-medium">{{ copy.paymentRequired }}</p>
        <p class="mt-1">{{ copy.paymentDetail }}</p>
      </div>
      <div v-else-if="state === 'selection_required'" class="space-y-3">
        <div class="rounded-xl bg-gray-50 p-4 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-200">
          <p class="font-medium text-gray-900 dark:text-white">{{ copy.selectPlan }}</p>
          <p class="mt-1">{{ copy.selectPlanDetail }}</p>
        </div>
        <button
          v-for="subscription in subscriptions"
          :key="subscription.id"
          type="button"
          class="flex w-full items-center justify-between gap-4 rounded-xl border px-4 py-3 text-left transition-colors"
          :class="selectedSubscriptionId === subscription.id
            ? 'border-primary-500 bg-primary-50 text-primary-800 dark:bg-primary-900/20 dark:text-primary-200'
            : 'border-gray-200 bg-white text-gray-700 hover:border-primary-300 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200'"
          @click="selectedSubscriptionId = subscription.id"
        >
          <span class="flex items-center gap-3">
            <span class="flex h-5 w-5 items-center justify-center rounded-full border" :class="selectedSubscriptionId === subscription.id ? 'border-primary-500' : 'border-gray-300 dark:border-dark-500'">
              <span v-if="selectedSubscriptionId === subscription.id" class="h-2.5 w-2.5 rounded-full bg-primary-500"></span>
            </span>
            <span>
              <strong class="block font-medium">{{ subscriptionDisplayName(subscription) }}</strong>
              <small class="mt-0.5 block opacity-75">{{ copy.expires }} {{ formatExpiry(subscription.expires_at) }}</small>
            </span>
          </span>
        </button>
      </div>
      <div v-else-if="errorMessage" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800/50 dark:bg-red-900/20 dark:text-red-300">
        {{ errorMessage }}
      </div>
      <div v-else class="rounded-xl bg-gray-50 p-4 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-200">
        {{ copy.confirmDetail }}
      </div>

      <div class="mt-6 flex flex-wrap gap-3">
        <button v-if="state === 'selection_required'" class="btn btn-primary" type="button" :disabled="isLoading || !selectedSubscriptionId" @click="approve(selectedSubscriptionId)">
          {{ copy.confirm }}
        </button>
        <button v-if="state === 'payment_required'" class="btn btn-primary" type="button" @click="goPurchase">
          {{ copy.purchase }}
        </button>
        <button v-if="state === 'error' || state === 'expired'" class="btn btn-primary" type="button" :disabled="isLoading || !sessionId" @click="approve()">
          {{ copy.retry }}
        </button>
        <button v-if="state !== 'authorized'" class="btn btn-secondary" type="button" @click="closePage">
          {{ copy.cancel }}
        </button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiClient } from '@/api'
import { paymentAPI } from '@/api/payment'
import { useI18n } from 'vue-i18n'
import { buildDesktopSubscriptionPurchaseRedirect } from '@/utils/authRedirect'
import type { SubscriptionPlanCatalogItem } from '@/types/payment'
import { localizedPlanContentForGroup } from '@/components/payment/localizedPlanContent'

type AuthorizationState = 'pending' | 'payment_required' | 'selection_required' | 'authorized' | 'denied' | 'expired' | 'error'
type AuthorizationSubscription = { id: number; group_id: number; group_name: string; expires_at: string }

const route = useRoute()
const router = useRouter()
const { locale } = useI18n()
const sessionId = computed(() => String(route.query.session || '').trim())
const state = ref<AuthorizationState>('pending')
const isLoading = ref(false)
const errorMessage = ref('')
const subscriptions = ref<AuthorizationSubscription[]>([])
const planCatalog = ref<SubscriptionPlanCatalogItem[]>([])
const selectedSubscriptionId = ref<number | null>(null)
let paymentPollTimer: number | null = null

const copy = computed(() => locale.value.startsWith('zh') ? {
  title: '连接桌面订阅服务', subtitle: '正在为 Codex 多开助手连接当前账号。', loading: '正在检查账号和订阅状态...', confirmDetail: '正在安全连接当前设备，不会在页面显示 API Key。', confirm: '确认接入', selectPlan: '选择此 Profile 使用的套餐', selectPlanDetail: '检测到多个有效套餐。后续请求将使用所选套餐的额度和限制。', expires: '有效期至', paymentRequired: '当前账号还没有有效订阅。', paymentDetail: '正在前往套餐页面，支付完成后会自动继续接入。', purchase: '购买套餐', retry: '重新检查', success: '接入成功', successDetail: '正在返回 Codex 多开助手并完成 Profile 创建。', close: '关闭页面', cancel: '取消'
} : {
  title: 'Connect desktop subscription', subtitle: 'Connecting this account to Codex Multi Launcher.', loading: 'Checking your account and subscription...', confirmDetail: 'Securely connecting this device. The API key will not be shown.', confirm: 'Authorize', selectPlan: 'Choose a plan for this profile', selectPlanDetail: 'Multiple active plans were found. Requests will use the selected plan limits.', expires: 'Expires', paymentRequired: 'No active subscription was found.', paymentDetail: 'Opening subscription plans. Authorization continues automatically after payment.', purchase: 'Buy a plan', retry: 'Check again', success: 'Connected', successDetail: 'Returning to Codex Multi Launcher to finish profile creation.', close: 'Close page', cancel: 'Cancel'
})

function subscriptionDisplayName(subscription: AuthorizationSubscription): string {
  return localizedPlanContentForGroup(planCatalog.value, subscription.group_id, locale.value)?.name
    || subscription.group_name
}

async function loadPlanCatalog(): Promise<void> {
  if (planCatalog.value.length > 0) return
  try {
    planCatalog.value = (await paymentAPI.getPlanCatalog()).data
  } catch {
    planCatalog.value = []
  }
}

async function approve(subscriptionId?: number | null): Promise<void> {
  if (isLoading.value) return
  if (!sessionId.value) {
    errorMessage.value = copy.value.confirmDetail
    return
  }
  isLoading.value = true
  errorMessage.value = ''
  try {
    const { data } = await apiClient.post<{ state: AuthorizationState; subscriptions?: AuthorizationSubscription[] }>(
      `/desktop-auth/sessions/${encodeURIComponent(sessionId.value)}/approve`,
      subscriptionId ? { subscription_id: subscriptionId } : {}
    )
    state.value = data.state
    subscriptions.value = data.subscriptions || []
    if (data.state === 'selection_required') {
      await loadPlanCatalog()
    }
    if (data.state === 'selection_required' && subscriptions.value.length > 0 && !subscriptions.value.some(item => item.id === selectedSubscriptionId.value)) {
      selectedSubscriptionId.value = subscriptions.value[0].id
    }
    if (data.state === 'payment_required') {
      await goPurchase()
    }
  } catch (error) {
    const candidate = error as { status?: number; message?: string }
    if (candidate.status === 402) {
      state.value = 'payment_required'
      await goPurchase()
    } else {
      state.value = 'error'
      errorMessage.value = candidate.message || copy.value.paymentDetail
    }
  } finally {
    isLoading.value = false
  }
}

function formatExpiry(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString(locale.value)
}

async function goPurchase(): Promise<void> {
  const purchasePath = buildDesktopSubscriptionPurchaseRedirect(route.fullPath)
  await router.replace(purchasePath || { path: '/purchase', query: { tab: 'subscription' } })
}

function closePage(): void {
  window.close()
}

onMounted(() => {
  if (!sessionId.value) {
    state.value = 'error'
    errorMessage.value = 'Missing authorization session.'
    return
  }
  void approve()
})

watch(state, (nextState) => {
  if (paymentPollTimer !== null) {
    window.clearInterval(paymentPollTimer)
    paymentPollTimer = null
  }
  if (nextState === 'payment_required') {
    paymentPollTimer = window.setInterval(() => void approve(), 5000)
  }
})

onBeforeUnmount(() => {
  if (paymentPollTimer !== null) {
    window.clearInterval(paymentPollTimer)
  }
})
</script>
