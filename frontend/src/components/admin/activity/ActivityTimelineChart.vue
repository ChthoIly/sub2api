<template>
  <div class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 text-xs text-gray-500 dark:text-gray-400">
      <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span class="inline-flex items-center gap-1.5">
          <span class="h-2 w-5 rounded-full bg-gradient-to-r from-primary-400 to-primary-500"></span>
          {{ t('admin.activityMonitor.chart.activeWindow') }}
        </span>
        <span class="inline-flex items-center gap-1.5">
          <span class="h-2 w-2 rounded-full bg-primary-500"></span>
          {{ t('admin.activityMonitor.chart.singleRequest') }}
        </span>
        <span class="inline-flex items-center gap-1.5">
          <span class="h-3 w-4 rounded-sm bg-gray-500/10 ring-1 ring-inset ring-gray-400/10 dark:bg-gray-300/10 dark:ring-gray-300/10"></span>
          {{ t('admin.activityMonitor.chart.overnight') }}
        </span>
      </div>
      <span class="inline-flex items-center gap-1.5">
        <Icon name="clock" size="xs" />
        {{ t('admin.activityMonitor.chart.localTime', { timezone }) }}
      </span>
    </div>

    <div class="overflow-x-auto pb-1">
      <div class="min-w-[640px]">
        <div
          class="grid items-center gap-x-3"
          style="grid-template-columns: 88px minmax(0, 1fr)"
        >
          <div aria-hidden="true"></div>
          <div class="relative h-5" aria-hidden="true">
            <span
              v-for="tick in hourTicks"
              :key="tick.value"
              class="absolute top-0 text-[10px] font-medium tabular-nums text-gray-400 dark:text-gray-500"
              :style="axisLabelStyle(tick.value)"
            >
              {{ tick.label }}
            </span>
          </div>

          <template v-for="day in days" :key="day.date">
            <div class="flex flex-col py-1.5 text-right">
              <span
                class="text-[11px] font-semibold tabular-nums"
                :class="day.has_activity ? 'text-gray-700 dark:text-gray-200' : 'text-gray-400 dark:text-gray-500'"
              >
                {{ formatDayLabel(day.date) }}
              </span>
              <span class="text-[10px] text-gray-400 dark:text-gray-500">
                {{ formatWeekday(day.date) }}
              </span>
            </div>

            <div
              class="relative my-1.5 h-10 overflow-hidden rounded-lg border border-gray-200/80 bg-gray-50/80 dark:border-dark-700/80 dark:bg-dark-900/50"
              role="img"
              :aria-label="dayAriaLabel(day)"
            >
              <!-- Overnight shading: 00:00-06:00 and 22:00-24:00. -->
              <span class="pointer-events-none absolute inset-y-0 left-0 w-1/4 bg-gray-500/[0.055] dark:bg-gray-300/[0.035]" aria-hidden="true"></span>
              <span class="pointer-events-none absolute inset-y-0 right-0 w-[8.333%] bg-gray-500/[0.055] dark:bg-gray-300/[0.035]" aria-hidden="true"></span>

              <span
                class="pointer-events-none absolute inset-y-0 left-1/4 border-l border-dashed border-gray-300/70 dark:border-dark-600/70"
                aria-hidden="true"
              ></span>
              <span
                class="pointer-events-none absolute inset-y-0 left-1/2 border-l border-dashed border-gray-300/70 dark:border-dark-600/70"
                aria-hidden="true"
              ></span>
              <span
                class="pointer-events-none absolute inset-y-0 left-3/4 border-l border-dashed border-gray-300/70 dark:border-dark-600/70"
                aria-hidden="true"
              ></span>

              <template v-for="(segment, index) in day.segments" :key="`${day.date}-${index}-${segment.start}`">
                <span
                  v-if="geometryFor(segment).isDot"
                  class="absolute top-1/2 h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-500 shadow-sm ring-2 ring-white/80 dark:bg-primary-400 dark:ring-dark-900/80"
                  :style="{ left: `${geometryFor(segment).leftPercent}%` }"
                  :title="segmentTitle(segment)"
                  :aria-label="segmentTitle(segment)"
                ></span>
                <span
                  v-else
                  class="absolute top-1/2 h-4 min-w-[3px] -translate-y-1/2 rounded-full bg-gradient-to-r from-primary-400 to-primary-500 shadow-sm ring-1 ring-primary-600/10 dark:from-primary-500 dark:to-primary-400"
                  :style="{
                    left: `${geometryFor(segment).leftPercent}%`,
                    width: `${geometryFor(segment).widthPercent}%`
                  }"
                  :title="segmentTitle(segment)"
                  :aria-label="segmentTitle(segment)"
                ></span>
              </template>

              <span
                v-if="!day.has_activity"
                class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-400 dark:text-gray-600"
                aria-hidden="true"
              >
                {{ t('admin.activityMonitor.chart.noActivity') }}
              </span>
            </div>
          </template>
        </div>
      </div>
    </div>

    <p class="text-[11px] leading-relaxed text-gray-400 dark:text-gray-500">
      {{ t('admin.activityMonitor.chart.gapHint', { minutes: gapMinutes }) }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { ActivityTimelineSegment } from '@/api/admin/activityMonitor'
import {
  formatDuration,
  getSegmentGeometry,
  type FilledActivityDay,
  type SegmentGeometry
} from './timeline'

defineProps<{
  days: FilledActivityDay[]
  gapMinutes: number
  timezone: string
}>()

const { t, locale } = useI18n()

const hourTicks = Array.from({ length: 9 }, (_, index) => {
  const hours = index * 3
  return {
    value: hours,
    label: hours === 24 ? '24' : String(hours).padStart(2, '0')
  }
})

const axisLabelStyle = (hour: number): Record<string, string> => {
  const percent = (hour / 24) * 100
  if (percent === 0) return { left: '0%' }
  if (percent === 100) return { right: '0%' }
  return { left: `${percent}%`, transform: 'translateX(-50%)' }
}

const formatDayLabel = (date: string): string =>
  new Date(`${date}T00:00:00`).toLocaleDateString(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    month: 'short',
    day: 'numeric'
  })

const formatWeekday = (date: string): string =>
  new Date(`${date}T00:00:00`).toLocaleDateString(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    weekday: 'short'
  })

const geometryFor = (segment: ActivityTimelineSegment): SegmentGeometry =>
  getSegmentGeometry(segment)

const segmentTitle = (segment: ActivityTimelineSegment): string =>
  t('admin.activityMonitor.chart.segmentTooltip', {
    start: segment.start,
    end: segment.end,
    requests: segment.requests.toLocaleString(),
    duration: formatDuration(segment.duration_seconds)
  })

const dayAriaLabel = (day: FilledActivityDay): string =>
  t('admin.activityMonitor.chart.dayAriaLabel', {
    date: day.date,
    requests: day.requests.toLocaleString(),
    segments: day.segment_count,
    coverage: formatDuration(day.coverage_seconds)
  })

</script>
