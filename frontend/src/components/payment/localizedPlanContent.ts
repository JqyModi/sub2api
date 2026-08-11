import type { SubscriptionPlan } from '@/types/payment'

export interface LocalizedPlanContent {
  name: string
  description: string
  features: string[]
}

function nonEmpty(value: string | undefined): string {
  return value?.trim() || ''
}

export function localizedPlanContent(plan: SubscriptionPlan, locale: string): LocalizedPlanContent {
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
