import { describe, expect, it } from 'vitest'

import type { SubscriptionPlan } from '@/types/payment'
import { localizedPlanContent, localizedPlanContentForGroup } from '../localizedPlanContent'

const plan: SubscriptionPlan = {
  id: 1,
  group_id: 10,
  name: 'Codex 轻量版',
  name_en: 'Codex Starter',
  description: '适合轻度使用和初次体验',
  description_en: 'For light usage and first-time users',
  price: 7.99,
  validity_days: 30,
  validity_unit: 'days',
  features: ['$35 标准用量额度', '每日最高 $20'],
  features_en: ['$35 standard usage quota', 'Up to $20 per day'],
  for_sale: true,
  max_sales: 5,
  per_user_limit: 1,
  sort_order: 1,
}

describe('localizedPlanContent', () => {
  it('uses English plan content on non-Chinese pages', () => {
    expect(localizedPlanContent(plan, 'en')).toEqual({
      name: 'Codex Starter',
      description: 'For light usage and first-time users',
      features: ['$35 standard usage quota', 'Up to $20 per day'],
    })
  })

  it('uses the default Chinese content on Chinese pages', () => {
    expect(localizedPlanContent(plan, 'zh-CN')).toEqual({
      name: 'Codex 轻量版',
      description: '适合轻度使用和初次体验',
      features: ['$35 标准用量额度', '每日最高 $20'],
    })
  })

  it('falls back to the default content when translations are empty', () => {
    expect(localizedPlanContent({ ...plan, name_en: '', description_en: '', features_en: [] }, 'en')).toEqual({
      name: plan.name,
      description: plan.description,
      features: plan.features,
    })
  })

  it('finds localized content by group for historical subscription display', () => {
    expect(localizedPlanContentForGroup([{ ...plan, for_sale: false }], 10, 'en')).toEqual({
      name: 'Codex Starter',
      description: 'For light usage and first-time users',
      features: ['$35 standard usage quota', 'Up to $20 per day'],
    })
    expect(localizedPlanContentForGroup([plan], 999, 'en')).toBeNull()
  })
})
