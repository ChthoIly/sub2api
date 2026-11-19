import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type ClaudeSubscriptionWindowType = '5h' | '7d'
export type ClaudeSubscriptionTriggerType = 'threshold' | 'config_update' | 'source_update'
export type ClaudeSubscriptionEstimateSource = 'current_cycle' | 'previous_cycle' | 'default'
export type ClaudeSubscriptionSourceStatus = 'ok' | 'missing' | 'multiple' | 'unsupported'

export interface ClaudeSubscriptionSourceAccountOption {
  id: number
  name: string
  email: string
  type: string
  status: string
}

export interface ClaudeSubscriptionSourceAccount extends ClaudeSubscriptionSourceAccountOption {
  five_hour_utilization_percent: number | null
  seven_day_utilization_percent: number | null
  five_hour_resets_at: string | null
  seven_day_resets_at: string | null
}

export interface ClaudeSubscriptionBoundAccount {
  id: number
  name: string
  email: string
  platform: string
  type: string
  status: string
}

export interface ClaudeSubscriptionQuotaCycle {
  id: number
  source_account_id: number
  window_type: ClaudeSubscriptionWindowType
  window_start: string
  window_end: string
  status: string
  last_threshold_percent: number
  last_utilization_percent: number | null
  last_window_cost_usd: number | null
  latest_estimated_full_usd: number | null
  last_sampled_at: string | null
  completed_at: string | null
}

export interface ClaudeSubscriptionAppliedEstimate {
  value_usd: number
  source: ClaudeSubscriptionEstimateSource
  threshold_percent?: number
  window_start?: string
  window_end?: string
  sampled_at?: string
}

export interface ClaudeSubscriptionGroupConfig {
  group_id: number
  group_name: string
  group_platform: string
  group_status: string
  subscription_count: number
  current_five_hour_limit_usd: number | null
  current_seven_day_limit_usd: number | null
  auto_enabled: boolean
  source_status: ClaudeSubscriptionSourceStatus
  source_warning?: string
  source_account_id: number | null
  source_account?: ClaudeSubscriptionSourceAccount | null
  bound_accounts: ClaudeSubscriptionBoundAccount[]
  applied_estimates: Record<ClaudeSubscriptionWindowType, ClaudeSubscriptionAppliedEstimate>
  current_cycles?: Partial<Record<ClaudeSubscriptionWindowType, ClaudeSubscriptionQuotaCycle>>
  initial_five_hour_full_usd: number
  initial_seven_day_full_usd: number
  five_hour_cap_percent: number
  seven_day_cap_percent: number
  five_hour_share_count: number
  seven_day_share_count: number
  five_hour_offset_usd: number
  seven_day_offset_usd: number
}

export interface ClaudeSubscriptionLimitOverview {
  thresholds: number[]
  source_accounts: ClaudeSubscriptionSourceAccountOption[]
  groups: ClaudeSubscriptionGroupConfig[]
}

export interface ClaudeSubscriptionGroupConfigUpdate {
  auto_enabled: boolean
  initial_five_hour_full_usd: number
  initial_seven_day_full_usd: number
  five_hour_cap_percent: number
  seven_day_cap_percent: number
  five_hour_share_count: number
  seven_day_share_count: number
  five_hour_offset_usd: number
  seven_day_offset_usd: number
}

export interface ClaudeSubscriptionQuotaEstimateHistory {
  id: number
  source_account_id: number
  source_account_name: string
  source_account_email: string
  window_type: ClaudeSubscriptionWindowType
  window_start: string
  window_end: string
  threshold_percent: number
  utilization_percent: number
  window_cost_usd: number
  estimated_full_usd: number
  sampled_at: string
  created_at: string
}

export interface ClaudeSubscriptionLimitAdjustmentHistory {
  id: number
  source_account_id: number | null
  source_account_name: string
  group_id: number | null
  group_name_snapshot: string
  user_id: number | null
  user_email: string
  subscription_id: number | null
  window_type: ClaudeSubscriptionWindowType
  trigger_type: ClaudeSubscriptionTriggerType
  threshold_percent: number | null
  source_window_start: string | null
  source_window_end: string | null
  source_utilization_percent: number | null
  source_window_cost_usd: number | null
  source_estimated_full_usd: number | null
  applied_estimate_usd: number
  applied_estimate_source: ClaudeSubscriptionEstimateSource
  cap_percent: number
  share_count: number
  manual_offset_usd: number
  old_limit_usd: number | null
  new_limit_usd: number
  delta_usd: number | null
  delta_percent: number | null
  changed: boolean
  created_at: string
}

export interface ClaudeHistoryFilters {
  page?: number
  page_size?: number
  source_account_id?: number
  group_id?: number
  window_type?: ClaudeSubscriptionWindowType | ''
  trigger_type?: ClaudeSubscriptionTriggerType | ''
  user_keyword?: string
  only_changed?: boolean
  start_time?: string
  end_time?: string
}

export async function getOverview(): Promise<ClaudeSubscriptionLimitOverview> {
  const { data } = await apiClient.get<ClaudeSubscriptionLimitOverview>('/admin/claude-subscription-limits')
  return data
}

export async function updateGroupConfig(
  groupId: number,
  payload: ClaudeSubscriptionGroupConfigUpdate
): Promise<void> {
  await apiClient.put(`/admin/claude-subscription-limits/groups/${groupId}`, payload)
}

export async function listQuotaEstimates(
  filters: ClaudeHistoryFilters
): Promise<PaginatedResponse<ClaudeSubscriptionQuotaEstimateHistory>> {
  const { data } = await apiClient.get<PaginatedResponse<ClaudeSubscriptionQuotaEstimateHistory>>(
    '/admin/claude-subscription-limits/history/estimates',
    { params: filters }
  )
  return data
}

export async function listLimitAdjustments(
  filters: ClaudeHistoryFilters
): Promise<PaginatedResponse<ClaudeSubscriptionLimitAdjustmentHistory>> {
  const { data } = await apiClient.get<PaginatedResponse<ClaudeSubscriptionLimitAdjustmentHistory>>(
    '/admin/claude-subscription-limits/history/adjustments',
    { params: filters }
  )
  return data
}
