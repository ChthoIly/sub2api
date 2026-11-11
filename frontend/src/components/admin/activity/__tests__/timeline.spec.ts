import { describe, expect, it } from 'vitest'
import type { ActivityTimelineDay, ActivityTimelineEntity } from '@/api/admin/activityMonitor'
import {
  fillActivityDays,
  formatDuration,
  getSegmentGeometry,
  parseClockTime,
  summarizeActivityTimeline
} from '../timeline'

const activeDay: ActivityTimelineDay = {
  date: '2026-11-07',
  requests: 322,
  active_hours: 8,
  segment_count: 1,
  coverage_seconds: 9612,
  longest_segment_seconds: 1513,
  segments: [
    {
      start: '04:26:57',
      end: '04:52:10',
      requests: 84,
      duration_seconds: 1513
    }
  ]
}

describe('activity timeline helpers', () => {
  it('fills every date in the selected range and leaves missing days empty', () => {
    const days = fillActivityDays('2026-11-06', '2026-11-10', [activeDay])

    expect(days.map((day) => day.date)).toEqual([
      '2026-11-06',
      '2026-11-07',
      '2026-11-08',
      '2026-11-09',
      '2026-11-10'
    ])
    expect(days[0]).toMatchObject({
      date: '2026-11-06',
      requests: 0,
      coverage_seconds: 0,
      segments: [],
      has_activity: false
    })
    expect(days[1]).toMatchObject({
      date: '2026-11-07',
      requests: 322,
      coverage_seconds: 9612,
      has_activity: true
    })
    expect(days[1].segments).toEqual(activeDay.segments)
    expect(days[4].has_activity).toBe(false)
  })

  it('positions segments from local wall-clock times', () => {
    const geometry = getSegmentGeometry(activeDay.segments[0])

    expect(geometry.startSeconds).toBe(16017)
    expect(geometry.endSeconds).toBe(17530)
    expect(geometry.durationSeconds).toBe(1513)
    expect(geometry.isDot).toBe(false)
    expect(geometry.leftPercent).toBeCloseTo(18.5382, 3)
    expect(geometry.widthPercent).toBeCloseTo(1.7512, 3)
  })

  it('renders single-request and zero-length windows as dots', () => {
    const singleRequest = getSegmentGeometry({
      start: '12:00:00',
      end: '12:10:00',
      requests: 1,
      duration_seconds: 600
    })
    const zeroLength = getSegmentGeometry({
      start: '13:00:00',
      end: '13:00:00',
      requests: 5,
      duration_seconds: 0
    })

    expect(singleRequest.isDot).toBe(true)
    expect(singleRequest.widthPercent).toBe(0)
    expect(zeroLength.isDot).toBe(true)
    expect(zeroLength.widthPercent).toBe(0)
    expect(parseClockTime('24:00:00')).toBe(86400)
  })

  it('formats compact durations for KPIs and tables', () => {
    expect(formatDuration(0)).toBe('0s')
    expect(formatDuration(45)).toBe('45s')
    expect(formatDuration(65)).toBe('1m 5s')
    expect(formatDuration(600)).toBe('10m')
    expect(formatDuration(3600)).toBe('1h')
    expect(formatDuration(33120)).toBe('9h 12m')
    expect(formatDuration(143928)).toBe('39h 58m')
  })

  it('sums entity totals without double-counting repeated daily fields', () => {
    const entities: ActivityTimelineEntity[] = [
      {
        entity_id: 1,
        name: 'Claude Ultra',
        detail: 'anthropic / oauth',
        total_requests: 5559,
        active_days: 5,
        total_active_hours: 65,
        total_coverage_seconds: 143928,
        longest_segment_seconds: 33120,
        days: [activeDay]
      },
      {
        entity_id: 2,
        name: 'Claude Pro',
        detail: 'anthropic / oauth',
        total_requests: 100,
        active_days: 1,
        total_active_hours: 2,
        total_coverage_seconds: 7200,
        longest_segment_seconds: 5000,
        days: []
      }
    ]

    expect(summarizeActivityTimeline(entities)).toEqual({
      entityCount: 2,
      totalRequests: 5659,
      highestSingleDayCoverageSeconds: 9612,
      longestContinuousSegmentSeconds: 33120
    })
  })
})
