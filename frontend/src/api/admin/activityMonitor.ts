/**
 * Admin activity monitor API endpoints
 * Activity windows are derived from request coverage in usage logs.
 */

import { apiClient } from '../client'

export type ActivityMonitorScope = 'user' | 'account'

export interface ActivityTimelineParams {
  scope: ActivityMonitorScope
  start_date: string
  end_date: string
  entity_id?: number
  limit?: number
  gap_minutes?: number
}

export interface ActivityTimelineSegment {
  start: string
  end: string
  requests: number
  duration_seconds: number
}

export interface ActivityTimelineDay {
  date: string
  requests: number
  active_hours: number
  segment_count: number
  coverage_seconds: number
  longest_segment_seconds: number
  segments: ActivityTimelineSegment[]
}

export interface ActivityTimelineEntity {
  entity_id: number
  name: string
  detail: string
  total_requests: number
  active_days: number
  total_active_hours: number
  total_coverage_seconds: number
  longest_segment_seconds: number
  days: ActivityTimelineDay[]
}

export interface ActivityTimelineResponse {
  scope: ActivityMonitorScope
  start_date: string
  end_date: string
  timezone: string
  gap_minutes: number
  entities: ActivityTimelineEntity[]
}

/**
 * Get daily active-time windows for users or upstream accounts.
 */
export async function getActivityTimeline(
  params: ActivityTimelineParams
): Promise<ActivityTimelineResponse> {
  const { data } = await apiClient.get<ActivityTimelineResponse>(
    '/admin/dashboard/activity-timeline',
    { params }
  )
  return data
}

const activityMonitorAPI = {
  getActivityTimeline
}

export default activityMonitorAPI
