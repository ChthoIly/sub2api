import type {
  ActivityTimelineDay,
  ActivityTimelineEntity,
  ActivityTimelineSegment
} from '@/api/admin/activityMonitor'

export const SECONDS_PER_DAY = 24 * 60 * 60

export interface FilledActivityDay extends ActivityTimelineDay {
  has_activity: boolean
}

export interface SegmentGeometry {
  leftPercent: number
  widthPercent: number
  isDot: boolean
  startSeconds: number
  endSeconds: number
  durationSeconds: number
}

export interface ActivityTimelineSummary {
  entityCount: number
  totalRequests: number
  highestSingleDayCoverageSeconds: number
  longestContinuousSegmentSeconds: number
}

function parseDateParts(date: string): [number, number, number] | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date.trim())
  if (!match) return null

  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const check = new Date(Date.UTC(year, month - 1, day))
  if (
    check.getUTCFullYear() !== year ||
    check.getUTCMonth() !== month - 1 ||
    check.getUTCDate() !== day
  ) {
    return null
  }
  return [year, month, day]
}

/**
 * Returns every calendar date in the inclusive range. Date arithmetic uses UTC
 * solely to keep calendar dates stable across DST and browser time zones.
 */
export function enumerateDateRange(startDate: string, endDate: string): string[] {
  const start = parseDateParts(startDate)
  const end = parseDateParts(endDate)
  if (!start || !end) return []

  const startMs = Date.UTC(start[0], start[1] - 1, start[2])
  const endMs = Date.UTC(end[0], end[1] - 1, end[2])
  if (startMs > endMs) return []

  const dates: string[] = []
  for (let current = startMs; current <= endMs; current += SECONDS_PER_DAY * 1000) {
    dates.push(new Date(current).toISOString().slice(0, 10))
  }
  return dates
}

function emptyDay(date: string): FilledActivityDay {
  return {
    date,
    requests: 0,
    active_hours: 0,
    segment_count: 0,
    coverage_seconds: 0,
    longest_segment_seconds: 0,
    segments: [],
    has_activity: false
  }
}

/**
 * Expands sparse API day entries to one row per selected calendar date.
 */
export function fillActivityDays(
  startDate: string,
  endDate: string,
  days: ActivityTimelineDay[] | null | undefined
): FilledActivityDay[] {
  const byDate = new Map<string, ActivityTimelineDay>()
  for (const day of days ?? []) {
    if (day?.date) byDate.set(day.date, day)
  }

  return enumerateDateRange(startDate, endDate).map((date) => {
    const day = byDate.get(date)
    if (!day) return emptyDay(date)

    const segments = Array.isArray(day.segments) ? day.segments : []
    return {
      date,
      requests: day.requests ?? 0,
      active_hours: day.active_hours ?? 0,
      segment_count: day.segment_count ?? segments.length,
      coverage_seconds: day.coverage_seconds ?? 0,
      longest_segment_seconds: day.longest_segment_seconds ?? 0,
      segments,
      has_activity:
        (day.requests ?? 0) > 0 ||
        (day.coverage_seconds ?? 0) > 0 ||
        (day.segment_count ?? 0) > 0 ||
        segments.length > 0
    }
  })
}

/**
 * Parses HH:mm or HH:mm:ss as seconds from local midnight. 24:00 is accepted as
 * the end-of-day boundary.
 */
export function parseClockTime(value: string): number | null {
  const match = /^(\d{1,2}):(\d{2})(?::(\d{2}))?$/.exec(value.trim())
  if (!match) return null

  const hours = Number(match[1])
  const minutes = Number(match[2])
  const seconds = Number(match[3] ?? 0)
  if (minutes > 59 || seconds > 59) return null
  if (hours === 24) {
    return minutes === 0 && seconds === 0 ? SECONDS_PER_DAY : null
  }
  if (hours > 23) return null
  return hours * 3600 + minutes * 60 + seconds
}

export function getSegmentGeometry(segment: ActivityTimelineSegment): SegmentGeometry {
  const parsedStart = parseClockTime(segment.start)
  const parsedEnd = parseClockTime(segment.end)
  const startSeconds = parsedStart ?? 0
  const endSeconds = parsedEnd ?? startSeconds
  const durationSeconds =
    Number.isFinite(segment.duration_seconds) && segment.duration_seconds >= 0
      ? segment.duration_seconds
      : Math.max(0, endSeconds - startSeconds)

  const isDot = segment.requests <= 1 || endSeconds <= startSeconds
  const leftPercent = Math.min(100, Math.max(0, (startSeconds / SECONDS_PER_DAY) * 100))
  const rawWidthPercent = Math.max(0, ((endSeconds - startSeconds) / SECONDS_PER_DAY) * 100)
  const widthPercent = isDot ? 0 : Math.min(100 - leftPercent, rawWidthPercent)

  return {
    leftPercent,
    widthPercent,
    isDot,
    startSeconds,
    endSeconds,
    durationSeconds
  }
}

/** Compact duration formatting used in KPIs, entity cards and the daily table. */
export function formatDuration(totalSeconds: number): string {
  const seconds = Math.max(0, Math.round(Number.isFinite(totalSeconds) ? totalSeconds : 0))
  if (seconds < 60) return `${seconds}s`

  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainder = seconds % 60

  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`
  }
  return remainder > 0 ? `${minutes}m ${remainder}s` : `${minutes}m`
}

/**
 * Aggregates headline values without summing the repeated daily/longest fields.
 */
export function summarizeActivityTimeline(
  entities: ActivityTimelineEntity[] | null | undefined
): ActivityTimelineSummary {
  let totalRequests = 0
  let highestSingleDayCoverageSeconds = 0
  let longestContinuousSegmentSeconds = 0

  for (const entity of entities ?? []) {
    totalRequests += Number.isFinite(entity.total_requests) ? entity.total_requests : 0
    longestContinuousSegmentSeconds = Math.max(
      longestContinuousSegmentSeconds,
      entity.longest_segment_seconds ?? 0
    )

    for (const day of entity.days ?? []) {
      highestSingleDayCoverageSeconds = Math.max(
        highestSingleDayCoverageSeconds,
        day.coverage_seconds ?? 0
      )
      longestContinuousSegmentSeconds = Math.max(
        longestContinuousSegmentSeconds,
        day.longest_segment_seconds ?? 0
      )
      for (const segment of day.segments ?? []) {
        longestContinuousSegmentSeconds = Math.max(
          longestContinuousSegmentSeconds,
          segment.duration_seconds ?? 0
        )
      }
    }
  }

  return {
    entityCount: entities?.length ?? 0,
    totalRequests,
    highestSingleDayCoverageSeconds,
    longestContinuousSegmentSeconds
  }
}
