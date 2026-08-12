import type { SubscriptionPlanCatalogItem } from '@/types/payment'

export interface LocalizedPlanContent {
  name: string
  description: string
  features: string[]
}

function nonEmpty(value: string | undefined): string {
  return value?.trim() || ''
}

export function localizedPlanContent(plan: SubscriptionPlanCatalogItem, locale: string): LocalizedPlanContent {
  const useEnglish = !locale.toLowerCase().startsWith('zh')
  const englishFeatures = plan.features_en?.filter(feature => feature.trim()) ?? []

  if (useEnglish) {
    return {
      name: nonEmpty(plan.name_en) || plan.name,
      description: nonEmpty(plan.description_en) || plan.description,
      features: englishFeatures.length > 0 ? englishFeatures : plan.features,
    }
  }

  return {
    name: plan.name || nonEmpty(plan.name_en),
    description: plan.description || nonEmpty(plan.description_en),
    features: plan.features.length > 0 ? plan.features : englishFeatures,
  }
}

export function localizedPlanContentForGroup(
  plans: SubscriptionPlanCatalogItem[],
  groupId: number,
  locale: string
): LocalizedPlanContent | null {
  const plan = plans.find(candidate => candidate.group_id === groupId)
  return plan ? localizedPlanContent(plan, locale) : null
}
