<template>
  <AppLayout>
    <div class="space-y-6 pb-10">
      <header class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div class="min-w-0">
          <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white">
            {{ t('admin.activityMonitor.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.activityMonitor.description') }}
          </p>
        </div>

        <div
          class="inline-flex w-fit rounded-xl border border-gray-200 bg-gray-100/80 p-1 dark:border-dark-700 dark:bg-dark-900/70"
          role="group"
          :aria-label="t('admin.activityMonitor.scope.label')"
        >
          <button
            v-for="tab in scopeTabs"
            :key="tab.value"
            type="button"
            class="rounded-lg px-4 py-2 text-sm font-medium transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
            :class="
              scope === tab.value
                ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
            "
            :aria-pressed="scope === tab.value"
            @click="setScope(tab.value)"
          >
            {{ t(tab.labelKey) }}
          </button>
        </div>
      </header>

      <section class="card p-4 md:p-5">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div class="flex flex-wrap items-end gap-4">
            <div>
              <span class="input-label">{{ t('admin.activityMonitor.controls.dateRange') }}</span>
              <DateRangePicker
                v-model:start-date="startDate"
                v-model:end-date="endDate"
                @change="onDateRangeChange"
              />
            </div>

            <label for="activity-gap-select" class="block">
              <span class="input-label">{{ t('admin.activityMonitor.controls.gap') }}</span>
              <select
                id="activity-gap-select"
                v-model.number="gapMinutes"
                class="input w-28 py-2"
                :aria-label="t('admin.activityMonitor.controls.gap')"
                @change="onGapChange"
              >
                <option v-for="minutes in gapOptions" :key="minutes" :value="minutes">
                  {{ t('admin.activityMonitor.controls.minutesValue', { minutes }) }}
                </option>
              </select>
            </label>

            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loading"
              :aria-label="t('admin.activityMonitor.controls.refresh')"
              @click="refreshTimeline"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
              {{ t('admin.activityMonitor.controls.refresh') }}
            </button>
          </div>

          <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span class="inline-flex items-center gap-1.5 rounded-lg bg-gray-50 px-2.5 py-1.5 dark:bg-dark-900/60">
              <Icon name="clock" size="xs" />
              {{ t('admin.activityMonitor.controls.timezone') }}:
              <span class="font-medium text-gray-700 dark:text-gray-300">{{ displayTimezone }}</span>
            </span>
          </div>
        </div>
      </section>

      <div id="activity-monitor-results" class="space-y-6">
        <p class="sr-only" aria-live="polite">
          {{ loading ? t('admin.activityMonitor.state.loading') : '' }}
        </p>

        <div
          v-if="loadError && timeline"
          class="flex flex-col gap-3 rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm dark:border-red-900/50 dark:bg-red-950/20 sm:flex-row sm:items-center sm:justify-between"
          role="alert"
        >
          <div class="flex items-start gap-2.5">
            <Icon name="x" size="sm" class="mt-0.5 flex-shrink-0 text-red-500" />
            <div>
              <p class="font-medium text-red-700 dark:text-red-300">{{ t('admin.activityMonitor.state.loadFailed') }}</p>
              <p v-if="loadError" class="mt-0.5 break-all text-xs text-red-600/80 dark:text-red-400/80">{{ loadError }}</p>
            </div>
          </div>
          <button type="button" class="btn btn-secondary flex-shrink-0" @click="refreshTimeline">
            {{ t('admin.activityMonitor.state.retry') }}
          </button>
        </div>

        <template v-if="loading && !timeline">
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4" aria-hidden="true">
            <div v-for="index in 4" :key="index" class="card animate-pulse p-4">
              <div class="flex items-start justify-between gap-4">
                <div class="flex-1 space-y-3">
                  <div class="h-3 w-24 rounded bg-gray-200 dark:bg-dark-700"></div>
                  <div class="h-7 w-20 rounded bg-gray-200 dark:bg-dark-700"></div>
                  <div class="h-2.5 w-full rounded bg-gray-100 dark:bg-dark-800"></div>
                </div>
                <div class="h-10 w-10 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
              </div>
            </div>
          </div>
          <div class="card animate-pulse p-5" aria-hidden="true">
            <div class="mb-6 h-5 w-48 rounded bg-gray-200 dark:bg-dark-700"></div>
            <div class="space-y-3">
              <div v-for="row in 7" :key="row" class="flex items-center gap-4">
                <div class="h-3 w-16 rounded bg-gray-100 dark:bg-dark-800"></div>
                <div class="h-8 flex-1 rounded-lg bg-gray-100 dark:bg-dark-800"></div>
              </div>
            </div>
          </div>
        </template>

        <template v-else-if="!timeline">
          <div class="card px-6 py-12 text-center">
            <span class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-red-50 text-red-500 dark:bg-red-950/30 dark:text-red-400">
              <Icon name="x" size="lg" />
            </span>
            <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.activityMonitor.state.loadFailed') }}
            </h2>
            <p v-if="loadError" class="mx-auto mt-2 max-w-xl break-all text-sm text-gray-500 dark:text-gray-400">
              {{ loadError }}
            </p>
            <button type="button" class="btn btn-primary mt-5" @click="refreshTimeline">
              {{ t('admin.activityMonitor.state.retry') }}
            </button>
          </div>
        </template>

        <template v-else>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <div class="card p-4">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.activityMonitor.kpi.entities') }}
                  </p>
                  <p class="mt-2 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">
                    {{ formatNumber(summary.entityCount) }}
                  </p>
                </div>
                <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400">
                  <Icon name="users" size="md" />
                </span>
              </div>
              <p class="mt-2 text-[11px] leading-relaxed text-gray-400 dark:text-gray-500">
                {{ t('admin.activityMonitor.kpi.entitiesHint') }}
              </p>
            </div>

            <div class="card p-4">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.activityMonitor.kpi.requests') }}
                  </p>
                  <p class="mt-2 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">
                    {{ formatNumber(summary.totalRequests) }}
                  </p>
                </div>
                <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-violet-50 text-violet-600 dark:bg-violet-900/30 dark:text-violet-400">
                  <Icon name="chart" size="md" />
                </span>
              </div>
              <p class="mt-2 text-[11px] leading-relaxed text-gray-400 dark:text-gray-500">
                {{ t('admin.activityMonitor.kpi.requestsHint') }}
              </p>
            </div>

            <div class="card p-4">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.activityMonitor.kpi.highestCoverage') }}
                  </p>
                  <p class="mt-2 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">
                    {{ formatDuration(summary.highestSingleDayCoverageSeconds) }}
                  </p>
                </div>
                <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400">
                  <Icon name="clock" size="md" />
                </span>
              </div>
              <p class="mt-2 text-[11px] leading-relaxed text-gray-400 dark:text-gray-500">
                {{ t('admin.activityMonitor.kpi.highestCoverageHint') }}
              </p>
            </div>

            <div class="card p-4">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0">
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.activityMonitor.kpi.longestSegment') }}
                  </p>
                  <p class="mt-2 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">
                    {{ formatDuration(summary.longestContinuousSegmentSeconds) }}
                  </p>
                </div>
                <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400">
                  <Icon name="chart" size="md" />
                </span>
              </div>
              <p class="mt-2 text-[11px] leading-relaxed text-gray-400 dark:text-gray-500">
                {{ t('admin.activityMonitor.kpi.longestSegmentHint') }}
              </p>
            </div>
          </div>

          <div v-if="entities.length === 0" class="card px-6 py-12 text-center">
            <span class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-dark-800 dark:text-gray-500">
              <Icon name="chart" size="lg" />
            </span>
            <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.activityMonitor.state.emptyTitle') }}
            </h2>
            <p class="mx-auto mt-2 max-w-lg text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.activityMonitor.state.emptyDescription') }}
            </p>
            <button type="button" class="btn btn-secondary mt-5" @click="refreshTimeline">
              {{ t('admin.activityMonitor.controls.refresh') }}
            </button>
          </div>

          <div v-else class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
            <div class="min-w-0 space-y-6">
              <section v-if="selectedEntity" class="card overflow-hidden">
                <div class="p-5 md:p-6">
                  <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                    <div class="min-w-0">
                      <div class="flex flex-wrap items-center gap-2">
                        <h2 class="truncate text-lg font-semibold text-gray-900 dark:text-white">
                          {{ selectedEntity.name || t('admin.activityMonitor.state.unknownEntity') }}
                        </h2>
                        <span class="rounded-full bg-primary-50 px-2 py-0.5 text-[11px] font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                          {{ scope === 'user' ? t('admin.activityMonitor.scope.users') : t('admin.activityMonitor.scope.accounts') }}
                        </span>
                      </div>
                      <p class="mt-1 truncate text-sm text-gray-500 dark:text-gray-400">
                        {{ selectedEntity.detail || t('admin.activityMonitor.state.noDetail') }}
                      </p>
                    </div>
                    <span class="inline-flex w-fit flex-shrink-0 items-center gap-1.5 rounded-lg bg-gray-50 px-2.5 py-1.5 text-xs font-medium text-gray-600 dark:bg-dark-900/60 dark:text-gray-300">
                      <Icon name="calendar" size="xs" />
                      {{ t('admin.activityMonitor.units.daysValue', { count: formatNumber(selectedEntity.active_days) }) }}
                    </span>
                  </div>

                  <div class="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
                    <div class="min-w-0 rounded-xl bg-gray-50 p-3 dark:bg-dark-900/40">
                      <p class="text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ t('admin.activityMonitor.summary.requests') }}</p>
                      <p class="mt-1 truncate text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatNumber(selectedEntity.total_requests) }}</p>
                    </div>
                    <div class="min-w-0 rounded-xl bg-gray-50 p-3 dark:bg-dark-900/40">
                      <p class="text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ t('admin.activityMonitor.summary.activeDays') }}</p>
                      <p class="mt-1 truncate text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatNumber(selectedEntity.active_days) }}</p>
                    </div>
                    <div class="min-w-0 rounded-xl bg-gray-50 p-3 dark:bg-dark-900/40">
                      <p class="text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ t('admin.activityMonitor.summary.activeHours') }}</p>
                      <p class="mt-1 truncate text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatHoursValue(selectedEntity.total_active_hours) }}</p>
                    </div>
                    <div class="min-w-0 rounded-xl bg-gray-50 p-3 dark:bg-dark-900/40">
                      <p class="text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ t('admin.activityMonitor.summary.coverage') }}</p>
                      <p class="mt-1 truncate text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatDuration(selectedEntity.total_coverage_seconds) }}</p>
                    </div>
                    <div class="min-w-0 rounded-xl bg-gray-50 p-3 dark:bg-dark-900/40">
                      <p class="text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ t('admin.activityMonitor.summary.longestSegment') }}</p>
                      <p class="mt-1 truncate text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatDuration(selectedEntity.longest_segment_seconds) }}</p>
                    </div>
                  </div>
                </div>

                <div class="border-t border-gray-100 p-5 dark:border-dark-700/60 md:p-6">
                  <div class="mb-5">
                    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                      {{ t('admin.activityMonitor.chart.title') }}
                    </h3>
                    <p class="mt-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
                      {{ t('admin.activityMonitor.chart.description') }}
                    </p>
                  </div>
                  <ActivityTimelineChart
                    :days="filledDays"
                    :gap-minutes="timeline.gap_minutes ?? gapMinutes"
                    :timezone="displayTimezone"
                  />
                </div>
              </section>

              <section class="card overflow-hidden">
                <div class="flex flex-col gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700/60 sm:flex-row sm:items-center sm:justify-between md:px-6">
                  <div>
                    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                      {{ t('admin.activityMonitor.table.title') }}
                    </h3>
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.activityMonitor.table.description') }}
                    </p>
                  </div>
                  <span class="text-xs text-gray-400 dark:text-gray-500">
                    {{ formatDateRangeLabel(loadedStartDate, loadedEndDate) }}
                  </span>
                </div>

                <div class="overflow-x-auto">
                  <table class="w-full min-w-[980px] border-collapse">
                    <thead class="bg-gray-50/80 dark:bg-dark-900/50">
                      <tr>
                        <th scope="col" class="px-4 py-3 text-left text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {{ t('admin.activityMonitor.table.date') }}
                        </th>
                        <th scope="col" class="px-4 py-3 text-right text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {{ t('admin.activityMonitor.table.requests') }}
                        </th>
                        <th scope="col" class="px-4 py-3 text-right text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {{ t('admin.activityMonitor.table.activeHours') }}
                        </th>
                        <th scope="col" class="px-4 py-3 text-right text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {{ t('admin.activityMonitor.table.coverage') }}
                        </th>
                        <th scope="col" class="px-4 py-3 text-right text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {{ t('admin.activityMonitor.table.longestSegment') }}
                        </th>
                        <th scope="col" class="px-4 py-3 text-left text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {{ t('admin.activityMonitor.table.windows') }}
                        </th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-100 dark:divide-dark-700/60">
                      <tr
                        v-for="day in filledDays"
                        :key="day.date"
                        class="transition-colors hover:bg-gray-50/70 dark:hover:bg-dark-900/30"
                        :class="{ 'bg-gray-50/30 dark:bg-dark-900/10': !day.has_activity }"
                      >
                        <td class="whitespace-nowrap px-4 py-3.5">
                          <span class="block text-sm font-medium text-gray-800 dark:text-gray-200">{{ formatTableDate(day.date) }}</span>
                          <span class="block text-[11px] text-gray-400 dark:text-gray-500">{{ formatTableWeekday(day.date) }}</span>
                        </td>
                        <td class="px-4 py-3.5 text-right text-sm tabular-nums" :class="day.requests > 0 ? 'font-medium text-gray-800 dark:text-gray-200' : 'text-gray-400 dark:text-gray-600'">
                          {{ formatNumber(day.requests) }}
                        </td>
                        <td class="px-4 py-3.5 text-right text-sm tabular-nums" :class="day.active_hours > 0 ? 'text-gray-700 dark:text-gray-300' : 'text-gray-400 dark:text-gray-600'">
                          {{ formatHoursValue(day.active_hours) }}
                        </td>
                        <td class="px-4 py-3.5 text-right text-sm tabular-nums" :class="day.coverage_seconds > 0 ? 'text-gray-700 dark:text-gray-300' : 'text-gray-400 dark:text-gray-600'">
                          {{ formatDuration(day.coverage_seconds) }}
                        </td>
                        <td class="px-4 py-3.5 text-right text-sm tabular-nums" :class="day.longest_segment_seconds > 0 ? 'text-gray-700 dark:text-gray-300' : 'text-gray-400 dark:text-gray-600'">
                          {{ formatDuration(day.longest_segment_seconds) }}
                        </td>
                        <td class="px-4 py-3.5">
                          <div v-if="day.segments.length > 0" class="flex max-h-24 max-w-[360px] flex-wrap gap-1.5 overflow-y-auto pr-1">
                            <span
                              v-for="(segment, segmentIndex) in day.segments"
                              :key="`${day.date}-${segmentIndex}-${segment.start}`"
                              class="inline-flex items-center gap-1.5 rounded-md border border-gray-200 bg-white px-2 py-1 text-[11px] text-gray-600 shadow-sm dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
                              :title="segmentTooltip(segment)"
                            >
                              <span class="font-medium tabular-nums">{{ segment.start }}–{{ segment.end }}</span>
                              <span class="text-gray-400 dark:text-gray-500">
                                {{ t('admin.activityMonitor.units.requestsValue', { count: formatNumber(segment.requests) }) }}
                              </span>
                            </span>
                          </div>
                          <span v-else class="text-xs text-gray-400 dark:text-gray-600">
                            {{ t('admin.activityMonitor.table.noWindows') }}
                          </span>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </section>
            </div>

            <aside class="order-first min-w-0 xl:order-last xl:sticky xl:top-24 xl:self-start">
              <section class="card overflow-hidden">
                <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-4 py-4 dark:border-dark-700/60">
                  <div class="min-w-0">
                    <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                      {{ t('admin.activityMonitor.entityList.title') }}
                    </h2>
                    <p class="mt-0.5 text-[11px] text-gray-400 dark:text-gray-500">
                      {{ t('admin.activityMonitor.entityList.loadedCount', { count: entities.length }) }}
                    </p>
                  </div>
                  <Icon v-if="loading" name="refresh" size="sm" class="animate-spin text-primary-500" />
                </div>

                <div class="border-b border-gray-100 p-3 dark:border-dark-700/60">
                  <div class="relative">
                    <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                    <input
                      v-model="searchQuery"
                      type="search"
                      class="input py-2 pl-9 pr-9"
                      :placeholder="t('admin.activityMonitor.entityList.searchPlaceholder')"
                      :aria-label="t('admin.activityMonitor.entityList.searchLabel')"
                    />
                    <button
                      v-if="searchQuery"
                      type="button"
                      class="absolute right-2.5 top-1/2 flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded text-gray-400 transition-colors hover:text-gray-700 dark:hover:text-gray-200"
                      :aria-label="t('admin.activityMonitor.entityList.clearSearch')"
                      @click="searchQuery = ''"
                    >
                      <Icon name="x" size="xs" />
                    </button>
                  </div>
                </div>

                <div v-if="filteredEntities.length === 0" class="px-4 py-8 text-center">
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t('admin.activityMonitor.entityList.noMatches') }}
                  </p>
                </div>

                <ul v-else class="max-h-[32rem] divide-y divide-gray-100 overflow-y-auto dark:divide-dark-700/60 xl:max-h-[calc(100vh-15rem)]">
                  <li v-for="entity in filteredEntities" :key="entity.entity_id">
                    <button
                      type="button"
                      class="w-full px-4 py-3.5 text-left transition-colors focus:z-10 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-primary-500/40"
                      :class="
                        isEntitySelected(entity)
                          ? 'bg-primary-50/80 dark:bg-primary-900/20'
                          : 'hover:bg-gray-50 dark:hover:bg-dark-900/40'
                      "
                      :aria-pressed="isEntitySelected(entity)"
                      @click="selectedEntityId = entity.entity_id"
                    >
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <p class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">
                            {{ entity.name || t('admin.activityMonitor.state.unknownEntity') }}
                          </p>
                          <p class="mt-0.5 truncate text-[11px] text-gray-400 dark:text-gray-500">
                            {{ entity.detail || t('admin.activityMonitor.state.noDetail') }}
                          </p>
                        </div>
                        <Icon
                          name="chevronRight"
                          size="sm"
                          class="mt-0.5 flex-shrink-0"
                          :class="isEntitySelected(entity) ? 'text-primary-500' : 'text-gray-300 dark:text-dark-500'"
                        />
                      </div>

                      <div class="mt-3 grid grid-cols-3 gap-2">
                        <div class="min-w-0">
                          <p class="text-[10px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">{{ t('admin.activityMonitor.entityList.requests') }}</p>
                          <p class="mt-0.5 truncate text-xs font-semibold tabular-nums text-gray-700 dark:text-gray-300">{{ formatNumber(entity.total_requests) }}</p>
                        </div>
                        <div class="min-w-0">
                          <p class="text-[10px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">{{ t('admin.activityMonitor.entityList.coverage') }}</p>
                          <p class="mt-0.5 truncate text-xs font-semibold tabular-nums text-gray-700 dark:text-gray-300">{{ formatDuration(entity.total_coverage_seconds) }}</p>
                        </div>
                        <div class="min-w-0">
                          <p class="text-[10px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">{{ t('admin.activityMonitor.entityList.activeDays') }}</p>
                          <p class="mt-0.5 truncate text-xs font-semibold tabular-nums text-gray-700 dark:text-gray-300">{{ formatNumber(entity.active_days) }}</p>
                        </div>
                      </div>
                    </button>
                  </li>
                </ul>
              </section>
            </aside>
          </div>
        </template>
      </div>

      <div class="rounded-2xl border border-dashed border-gray-200 bg-white/50 px-4 py-3.5 dark:border-dark-700 dark:bg-dark-900/20">
        <div class="flex items-start gap-2.5 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
          <Icon name="lightbulb" size="sm" class="mt-0.5 flex-shrink-0 text-amber-500" />
          <div class="space-y-1">
            <p>
              <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('admin.activityMonitor.footnote.title') }}</span>
              {{ t('admin.activityMonitor.footnote.coverage') }}
            </p>
            <p>{{ t('admin.activityMonitor.footnote.observational') }}</p>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Icon from '@/components/icons/Icon.vue'
import ActivityTimelineChart from '@/components/admin/activity/ActivityTimelineChart.vue'
import { adminAPI } from '@/api/admin'
import type {
  ActivityMonitorScope,
  ActivityTimelineResponse,
  ActivityTimelineSegment
} from '@/api/admin/activityMonitor'
import {
  fillActivityDays,
  formatDuration,
  summarizeActivityTimeline
} from '@/components/admin/activity/timeline'

const { t, locale } = useI18n()

const scopeTabs: Array<{ value: ActivityMonitorScope; labelKey: string }> = [
  { value: 'user', labelKey: 'admin.activityMonitor.scope.users' },
  { value: 'account', labelKey: 'admin.activityMonitor.scope.accounts' }
]

const gapOptions = [5, 15, 30, 60]
const defaultRange = getDefaultDateRange()
const scope = ref<ActivityMonitorScope>('user')
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const gapMinutes = ref(30)
const timeline = ref<ActivityTimelineResponse | null>(null)
const loading = ref(false)
const loadError = ref('')
const searchQuery = ref('')
const selectedEntityId = ref<number | null>(null)
const loadedStartDate = ref(defaultRange.start)
const loadedEndDate = ref(defaultRange.end)

let loadSequence = 0

const entities = computed(() => timeline.value?.entities ?? [])
const filteredEntities = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase()
  if (!query) return entities.value

  return entities.value.filter((entity) => {
    return (
      String(entity.entity_id).includes(query) ||
      entity.name.toLocaleLowerCase().includes(query) ||
      entity.detail.toLocaleLowerCase().includes(query)
    )
  })
})
const selectedEntity = computed(() => {
  return (
    entities.value.find((entity) => entity.entity_id === selectedEntityId.value) ??
    entities.value[0] ??
    null
  )
})
const filledDays = computed(() => {
  if (!selectedEntity.value) return []
  return fillActivityDays(
    loadedStartDate.value,
    loadedEndDate.value,
    selectedEntity.value.days
  )
})
const summary = computed(() => summarizeActivityTimeline(entities.value))
const displayTimezone = computed(() => {
  if (timeline.value?.timezone) return timeline.value.timezone
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    return 'UTC'
  }
})

const numberFormatter = computed(
  () => new Intl.NumberFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US')
)

function toLocalDateString(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function getDefaultDateRange(): { start: string; end: string } {
  const end = new Date()
  const start = new Date(end)
  start.setDate(start.getDate() - 6)
  return { start: toLocalDateString(start), end: toLocalDateString(end) }
}

function formatNumber(value: number | null | undefined): string {
  return numberFormatter.value.format(Number.isFinite(value ?? NaN) ? Number(value) : 0)
}

function formatDecimal(value: number): string {
  return new Intl.NumberFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    maximumFractionDigits: 2
  }).format(Number.isFinite(value) ? value : 0)
}

function formatHoursValue(value: number): string {
  return t('admin.activityMonitor.units.hoursValue', { value: formatDecimal(value) })
}

function formatTableDate(date: string): string {
  return new Date(`${date}T00:00:00`).toLocaleDateString(
    locale.value === 'zh' ? 'zh-CN' : 'en-US',
    { year: 'numeric', month: 'short', day: 'numeric' }
  )
}

function formatTableWeekday(date: string): string {
  return new Date(`${date}T00:00:00`).toLocaleDateString(
    locale.value === 'zh' ? 'zh-CN' : 'en-US',
    { weekday: 'long' }
  )
}

function formatDateRangeLabel(start: string, end: string): string {
  const formatter = new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    month: 'short',
    day: 'numeric'
  })
  return `${formatter.format(new Date(`${start}T00:00:00`))} – ${formatter.format(new Date(`${end}T00:00:00`))}`
}

function isEntitySelected(entity: { entity_id: number }): boolean {
  return selectedEntity.value?.entity_id === entity.entity_id
}

function segmentTooltip(segment: ActivityTimelineSegment): string {
  return t('admin.activityMonitor.chart.segmentTooltip', {
    start: segment.start,
    end: segment.end,
    requests: formatNumber(segment.requests),
    duration: formatDuration(segment.duration_seconds)
  })
}

function extractErrorMessage(error: unknown): string {
  if (typeof error === 'string') return error
  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string' && message.trim()) return message
  }
  return ''
}

async function loadTimeline(clearBeforeLoad = false): Promise<void> {
  const sequence = ++loadSequence
  const requestStart = startDate.value
  const requestEnd = endDate.value

  if (clearBeforeLoad) {
    timeline.value = null
    selectedEntityId.value = null
  }
  loading.value = true
  loadError.value = ''

  try {
    const data = await adminAPI.activityMonitor.getActivityTimeline({
      scope: scope.value,
      start_date: requestStart,
      end_date: requestEnd,
      gap_minutes: gapMinutes.value
    })
    if (sequence !== loadSequence) return

    const normalized: ActivityTimelineResponse = {
      ...data,
      entities: Array.isArray(data.entities) ? data.entities : []
    }
    timeline.value = normalized
    loadedStartDate.value = normalized.start_date || requestStart
    loadedEndDate.value = normalized.end_date || requestEnd
    const selectedStillExists = normalized.entities.some(
      (entity) => entity.entity_id === selectedEntityId.value
    )
    if (!selectedStillExists) {
      selectedEntityId.value = normalized.entities[0]?.entity_id ?? null
    }
  } catch (error) {
    if (sequence !== loadSequence) return
    loadError.value = extractErrorMessage(error)
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

function setScope(nextScope: ActivityMonitorScope): void {
  if (scope.value === nextScope) return
  scope.value = nextScope
  searchQuery.value = ''
  void loadTimeline(true)
}

function onDateRangeChange(range: { startDate: string; endDate: string }): void {
  startDate.value = range.startDate
  endDate.value = range.endDate
  void loadTimeline(true)
}

function onGapChange(): void {
  void loadTimeline(false)
}

function refreshTimeline(): void {
  void loadTimeline(false)
}

onMounted(() => {
  void loadTimeline(true)
})
</script>
