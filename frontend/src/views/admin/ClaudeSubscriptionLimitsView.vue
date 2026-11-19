<template>
  <AppLayout>
    <div class="space-y-6 pb-10">
      <header class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div class="min-w-0">
          <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white">
            {{ t('admin.subscriptionLimits.title') }}
          </h1>
          <p class="mt-1 max-w-4xl text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.subscriptionLimits.description') }}
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="refreshAll">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          <span class="ml-2">{{ t('admin.subscriptionLimits.common.refresh') }}</span>
        </button>
      </header>

      <div
        v-if="loadError"
        class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-300"
      >
        {{ loadError }}
      </div>

      <div
        v-if="conflictGroups.length > 0"
        class="rounded-xl border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800/70 dark:bg-amber-950/30 dark:text-amber-200"
      >
        <div class="font-semibold">
          {{ t('admin.subscriptionLimits.source.conflictBannerTitle', { count: conflictGroups.length }) }}
        </div>
        <p class="mt-1 text-xs leading-relaxed text-amber-800 dark:text-amber-300">
          {{ t('admin.subscriptionLimits.source.conflictBannerHint') }}
        </p>
        <ul class="mt-2 space-y-1 text-xs">
          <li v-for="group in conflictGroups" :key="group.group_id" class="flex flex-wrap items-center gap-2">
            <span class="font-semibold">{{ group.group_name }} (#{{ group.group_id }})</span>
            <span
              v-if="group.auto_enabled"
              class="rounded bg-red-100 px-1.5 py-0.5 text-[11px] font-medium text-red-700 dark:bg-red-900/50 dark:text-red-300"
            >
              {{ t('admin.subscriptionLimits.source.pausedBadge') }}
            </span>
            <span>{{ group.source_warning || sourceStatusLabel(group) }}</span>
          </li>
        </ul>
      </div>

      <section class="card p-5 space-y-4">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.subscriptionLimits.rules.title') }}
          </h2>
          <ul class="mt-2 space-y-1 text-xs text-gray-600 dark:text-gray-300">
            <li>• {{ t('admin.subscriptionLimits.rules.thresholds', { thresholds: thresholdText }) }}</li>
            <li>• {{ t('admin.subscriptionLimits.rules.sourceBinding') }}</li>
            <li>• {{ t('admin.subscriptionLimits.rules.initialDefault') }}</li>
          </ul>
        </div>

        <div
          v-if="resolvedSourceSummaries.length === 0"
          class="rounded-xl border border-dashed border-gray-200 px-4 py-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400"
        >
          {{ t('admin.subscriptionLimits.rules.noResolvedSources') }}
        </div>

        <div v-else class="grid gap-4 lg:grid-cols-2">
          <div
            v-for="summary in resolvedSourceSummaries"
            :key="summary.account.id"
            class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"
          >
            <div class="flex flex-wrap items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="font-semibold text-gray-900 dark:text-white">
                  {{ summary.account.name }}
                  <span class="ml-1 text-xs font-normal text-gray-500 dark:text-gray-400">
                    #{{ summary.account.id }} · {{ summary.account.type }}
                  </span>
                </div>
                <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ summary.account.email || '—' }} · {{ summary.account.status }}
                </div>
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.subscriptionLimits.source.boundGroups', { groups: summary.groupNames.join('、') }) }}
              </div>
            </div>

            <div class="mt-3 grid gap-3 sm:grid-cols-2">
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900/60">
                <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                  <span class="font-mono font-semibold uppercase">5h</span>
                  <span v-if="summary.estimate5h && summary.estimate5h.source !== 'default'" class="rounded bg-primary-50 px-1.5 py-0.5 text-[11px] font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                    {{ estimateSourceBadge(summary.estimate5h) }}
                  </span>
                  <span v-else class="text-[11px]">{{ t('admin.subscriptionLimits.source.waitingFirstSample') }}</span>
                </div>
                <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
                  {{ formatPercent(summary.account.five_hour_utilization_percent) }}
                </div>
                <div class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">
                  {{ summary.account.five_hour_resets_at ? t('admin.subscriptionLimits.source.resetAt', { time: formatTime(summary.account.five_hour_resets_at) }) : t('admin.subscriptionLimits.source.noSnapshot') }}
                </div>
                <div
                  v-if="summary.estimate5h && summary.estimate5h.source !== 'default'"
                  class="mt-2 border-t border-gray-200/80 pt-2 text-xs dark:border-dark-700"
                >
                  <span class="text-gray-500 dark:text-gray-400">{{ t('admin.subscriptionLimits.source.appliedEstimate') }}：</span>
                  <span class="font-semibold text-gray-900 dark:text-white">{{ formatMoney(summary.estimate5h.value_usd) }}</span>
                </div>
              </div>

              <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900/60">
                <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                  <span class="font-mono font-semibold uppercase">7d</span>
                  <span v-if="summary.estimate7d && summary.estimate7d.source !== 'default'" class="rounded bg-primary-50 px-1.5 py-0.5 text-[11px] font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                    {{ estimateSourceBadge(summary.estimate7d) }}
                  </span>
                  <span v-else class="text-[11px]">{{ t('admin.subscriptionLimits.source.waitingFirstSample') }}</span>
                </div>
                <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
                  {{ formatPercent(summary.account.seven_day_utilization_percent) }}
                </div>
                <div class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">
                  {{ summary.account.seven_day_resets_at ? t('admin.subscriptionLimits.source.resetAt', { time: formatTime(summary.account.seven_day_resets_at) }) : t('admin.subscriptionLimits.source.noSnapshot') }}
                </div>
                <div
                  v-if="summary.estimate7d && summary.estimate7d.source !== 'default'"
                  class="mt-2 border-t border-gray-200/80 pt-2 text-xs dark:border-dark-700"
                >
                  <span class="text-gray-500 dark:text-gray-400">{{ t('admin.subscriptionLimits.source.appliedEstimate') }}：</span>
                  <span class="font-semibold text-gray-900 dark:text-white">{{ formatMoney(summary.estimate7d.value_usd) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="space-y-4">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.subscriptionLimits.config.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.subscriptionLimits.config.hint') }}
          </p>
        </div>

        <div
          v-if="groupDrafts.length === 0"
          class="card px-6 py-12 text-center text-sm text-gray-500 dark:text-gray-400"
        >
          {{ t('admin.subscriptionLimits.config.empty') }}
        </div>

        <div v-else class="space-y-4">
          <div
            v-for="draft in groupDrafts"
            :key="draft.group_id"
            class="card overflow-hidden"
            :class="draft.source_status !== 'ok' ? 'border-amber-300 dark:border-amber-800/70' : ''"
          >
            <div class="flex flex-col gap-4 border-b border-gray-200 p-5 dark:border-dark-700 lg:flex-row lg:items-start lg:justify-between">
              <div class="min-w-0 flex-1 space-y-2">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                    {{ draft.group_name }}
                  </h3>
                  <span class="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    #{{ draft.group_id }} · {{ draft.group_platform }} · {{ draft.group_status }}
                  </span>
                  <span class="rounded bg-primary-50 px-2 py-0.5 text-xs text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                    {{ t('admin.subscriptionLimits.config.subscriptions', { count: draft.subscription_count }) }}
                  </span>
                  <span
                    v-if="draft.auto_enabled && draft.source_status !== 'ok'"
                    class="rounded bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-900/40 dark:text-red-300"
                  >
                    {{ t('admin.subscriptionLimits.source.pausedBadge') }}
                  </span>
                </div>

                <div
                  v-if="draft.source_status === 'ok' && draft.source_account"
                  class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-600 dark:text-gray-300"
                >
                  <span class="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-0.5 font-medium text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300">
                    {{ t('admin.subscriptionLimits.source.uniqueSource') }}：{{ draft.source_account.name }}
                  </span>
                  <span>{{ draft.source_account.email || `#${draft.source_account.id}` }}</span>
                  <span>5h: {{ formatPercent(draft.source_account.five_hour_utilization_percent) }}</span>
                  <span>7d: {{ formatPercent(draft.source_account.seven_day_utilization_percent) }}</span>
                </div>

                <div
                  v-else
                  class="rounded-lg border border-amber-200 bg-amber-50/80 px-3 py-2 text-xs text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200"
                >
                  <div class="font-medium">
                    {{ draft.source_warning || sourceStatusLabel(draft) }}
                  </div>
                  <div v-if="draft.bound_accounts.length > 0" class="mt-1.5 flex flex-wrap gap-1.5">
                    <span
                      v-for="acc in draft.bound_accounts"
                      :key="acc.id"
                      class="rounded bg-white/80 px-2 py-0.5 text-[11px] text-amber-900 ring-1 ring-amber-200 dark:bg-dark-800 dark:text-amber-200 dark:ring-amber-800/60"
                    >
                      {{ acc.name }} (#{{ acc.id }} · {{ acc.platform }}/{{ acc.type }} · {{ acc.status }})
                    </span>
                  </div>
                </div>
              </div>

              <div class="flex flex-wrap items-center gap-4 lg:flex-nowrap">
                <div class="flex items-center gap-2">
                  <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
                    {{ t('admin.subscriptionLimits.config.autoEnabled') }}
                  </span>
                  <Toggle
                    v-model="draft.auto_enabled"
                    :disabled="draft.source_status !== 'ok' && !draft.auto_enabled"
                  />
                </div>
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="savingGroupId === draft.group_id || (draft.auto_enabled && draft.source_status !== 'ok')"
                  @click="saveGroup(draft)"
                >
                  <Icon v-if="savingGroupId === draft.group_id" name="refresh" size="sm" class="animate-spin" />
                  <span :class="{ 'ml-2': savingGroupId === draft.group_id }">
                    {{ t('admin.subscriptionLimits.config.save') }}
                  </span>
                </button>
              </div>
            </div>

            <div
              v-if="draft.source_status !== 'ok'"
              class="border-b border-amber-100 bg-amber-50/40 px-5 py-2 text-xs text-amber-800 dark:border-amber-900/40 dark:bg-amber-950/15 dark:text-amber-300"
            >
              {{ draft.auto_enabled ? t('admin.subscriptionLimits.config.pausedSaveHint') : t('admin.subscriptionLimits.config.cannotEnableHint') }}
            </div>

            <div class="overflow-x-auto">
              <table class="min-w-[1040px] w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:bg-dark-900/60 dark:text-gray-400">
                  <tr>
                    <th class="px-4 py-2.5">{{ t('admin.subscriptionLimits.config.window') }}</th>
                    <th class="px-4 py-2.5">{{ t('admin.subscriptionLimits.config.currentLimit') }}</th>
                    <th class="px-4 py-2.5">{{ t('admin.subscriptionLimits.config.appliedEstimate') }}</th>
                    <th class="px-4 py-2.5">{{ t('admin.subscriptionLimits.config.initialFull') }}</th>
                    <th class="px-4 py-2.5">{{ t('admin.subscriptionLimits.config.cap') }}</th>
                    <th class="px-4 py-2.5">{{ t('admin.subscriptionLimits.config.shares') }}</th>
                    <th class="px-4 py-2.5">{{ t('admin.subscriptionLimits.config.offset') }}</th>
                    <th class="px-4 py-2.5">{{ t('admin.subscriptionLimits.config.previewLimit') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                  <tr>
                    <td class="px-4 py-3">
                      <span class="rounded bg-gray-100 px-2 py-1 font-mono text-xs font-semibold dark:bg-dark-700">5h</span>
                    </td>
                    <td class="px-4 py-3 text-gray-700 dark:text-gray-300">
                      {{ formatNullableMoney(draft.current_five_hour_limit_usd) }}
                    </td>
                    <td class="px-4 py-3">
                      <div class="font-medium text-gray-900 dark:text-white">
                        {{ formatMoney(effectiveEstimateForPreview(draft, '5h')) }}
                      </div>
                      <div class="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs">
                        <span
                          class="rounded px-1.5 py-0.5 text-[11px]"
                          :class="estimateBadgeClass(draft.applied_estimates?.['5h']?.source)"
                        >
                          {{ estimateSourceBadge(draft.applied_estimates?.['5h']) }}
                        </span>
                        <span
                          v-if="draft.applied_estimates?.['5h']?.sampled_at"
                          class="text-gray-500 dark:text-gray-400"
                        >
                          {{ formatTime(draft.applied_estimates['5h'].sampled_at) }}
                        </span>
                      </div>
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="draft.initial_five_hour_full_usd"
                        type="number"
                        min="0.01"
                        step="1"
                        class="input w-32"
                      />
                      <div
                        class="mt-1 text-[11px]"
                        :class="draft.applied_estimates?.['5h']?.source === 'default' ? 'text-primary-600 dark:text-primary-300' : 'text-gray-400 dark:text-gray-500'"
                      >
                        {{ draft.applied_estimates?.['5h']?.source === 'default' ? t('admin.subscriptionLimits.config.initialActive') : t('admin.subscriptionLimits.config.initialOverridden') }}
                      </div>
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="draft.five_hour_cap_percent"
                        type="number"
                        min="0"
                        max="100"
                        step="0.01"
                        class="input w-24"
                      />
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="draft.five_hour_share_count"
                        type="number"
                        min="1"
                        step="1"
                        class="input w-20"
                      />
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="draft.five_hour_offset_usd"
                        type="number"
                        step="0.01"
                        class="input w-28"
                      />
                    </td>
                    <td class="px-4 py-3 font-semibold text-primary-600 dark:text-primary-300">
                      {{ formatMoney(previewLimit(draft, '5h')) }}
                    </td>
                  </tr>

                  <tr>
                    <td class="px-4 py-3">
                      <span class="rounded bg-gray-100 px-2 py-1 font-mono text-xs font-semibold dark:bg-dark-700">7d</span>
                    </td>
                    <td class="px-4 py-3 text-gray-700 dark:text-gray-300">
                      {{ formatNullableMoney(draft.current_seven_day_limit_usd) }}
                    </td>
                    <td class="px-4 py-3">
                      <div class="font-medium text-gray-900 dark:text-white">
                        {{ formatMoney(effectiveEstimateForPreview(draft, '7d')) }}
                      </div>
                      <div class="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs">
                        <span
                          class="rounded px-1.5 py-0.5 text-[11px]"
                          :class="estimateBadgeClass(draft.applied_estimates?.['7d']?.source)"
                        >
                          {{ estimateSourceBadge(draft.applied_estimates?.['7d']) }}
                        </span>
                        <span
                          v-if="draft.applied_estimates?.['7d']?.sampled_at"
                          class="text-gray-500 dark:text-gray-400"
                        >
                          {{ formatTime(draft.applied_estimates['7d'].sampled_at) }}
                        </span>
                      </div>
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="draft.initial_seven_day_full_usd"
                        type="number"
                        min="0.01"
                        step="10"
                        class="input w-32"
                      />
                      <div
                        class="mt-1 text-[11px]"
                        :class="draft.applied_estimates?.['7d']?.source === 'default' ? 'text-primary-600 dark:text-primary-300' : 'text-gray-400 dark:text-gray-500'"
                      >
                        {{ draft.applied_estimates?.['7d']?.source === 'default' ? t('admin.subscriptionLimits.config.initialActive') : t('admin.subscriptionLimits.config.initialOverridden') }}
                      </div>
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="draft.seven_day_cap_percent"
                        type="number"
                        min="0"
                        max="100"
                        step="0.01"
                        class="input w-24"
                      />
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="draft.seven_day_share_count"
                        type="number"
                        min="1"
                        step="1"
                        class="input w-20"
                      />
                    </td>
                    <td class="px-4 py-3">
                      <input
                        v-model.number="draft.seven_day_offset_usd"
                        type="number"
                        step="0.01"
                        class="input w-28"
                      />
                    </td>
                    <td class="px-4 py-3 font-semibold text-primary-600 dark:text-primary-300">
                      {{ formatMoney(previewLimit(draft, '7d')) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-200 p-5 dark:border-dark-700">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.subscriptionLimits.history.title') }}</h2>
            <div class="inline-flex w-fit rounded-xl border border-gray-200 bg-gray-100/80 p-1 dark:border-dark-700 dark:bg-dark-900/70">
              <button
                type="button"
                class="rounded-lg px-3 py-1.5 text-sm font-medium"
                :class="historyTab === 'estimates' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300' : 'text-gray-500 dark:text-gray-400'"
                @click="historyTab = 'estimates'"
              >
                {{ t('admin.subscriptionLimits.history.estimatesTab') }}
              </button>
              <button
                type="button"
                class="rounded-lg px-3 py-1.5 text-sm font-medium"
                :class="historyTab === 'adjustments' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300' : 'text-gray-500 dark:text-gray-400'"
                @click="historyTab = 'adjustments'"
              >
                {{ t('admin.subscriptionLimits.history.adjustmentsTab') }}
              </button>
            </div>
          </div>
        </div>

        <template v-if="historyTab === 'estimates'">
          <div class="flex flex-wrap items-end gap-3 border-b border-gray-200 p-4 dark:border-dark-700">
            <label class="block">
              <span class="input-label">{{ t('admin.subscriptionLimits.history.filters.window') }}</span>
              <select v-model="estimateFilters.window_type" class="input w-36">
                <option value="">{{ t('admin.subscriptionLimits.history.filters.allWindows') }}</option>
                <option value="5h">5h</option>
                <option value="7d">7d</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.subscriptionLimits.history.filters.source') }}</span>
              <select v-model.number="estimateFilters.source_account_id" class="input w-64">
                <option :value="0">{{ t('admin.subscriptionLimits.history.filters.allSources') }}</option>
                <option v-for="option in sourceAccounts" :key="option.id" :value="option.id">{{ option.name }} · {{ option.email || `#${option.id}` }}</option>
              </select>
            </label>
            <button type="button" class="btn btn-secondary" :disabled="estimateLoading" @click="loadEstimates(1)">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': estimateLoading }" />
              <span class="ml-2">{{ t('admin.subscriptionLimits.history.filters.refresh') }}</span>
            </button>
          </div>

          <div v-if="estimateLoading" class="px-6 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
          <div v-else-if="estimates.length === 0" class="px-6 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.subscriptionLimits.history.estimates.empty') }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="min-w-[1200px] w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
              <thead class="bg-gray-50 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:bg-dark-900/60 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.estimates.time') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.estimates.source') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.estimates.window') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.estimates.threshold') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.estimates.utilization') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.estimates.windowCost') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.estimates.estimatedFull') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.estimates.cycleRange') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                <tr v-for="item in estimates" :key="item.id" class="hover:bg-gray-50/70 dark:hover:bg-dark-800/40">
                  <td class="whitespace-nowrap px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatTime(item.sampled_at) }}</td>
                  <td class="px-4 py-3">
                    <div class="font-medium text-gray-900 dark:text-white">{{ item.source_account_name }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.source_account_email }} · #{{ item.source_account_id }}</div>
                  </td>
                  <td class="px-4 py-3"><span class="rounded bg-gray-100 px-2 py-1 font-mono text-xs dark:bg-dark-700">{{ item.window_type }}</span></td>
                  <td class="px-4 py-3">{{ item.threshold_percent }}%</td>
                  <td class="px-4 py-3">{{ formatPercent(item.utilization_percent) }}</td>
                  <td class="px-4 py-3">{{ formatMoney(item.window_cost_usd) }}</td>
                  <td class="px-4 py-3 font-semibold text-primary-600 dark:text-primary-300">{{ formatMoney(item.estimated_full_usd) }}</td>
                  <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-500 dark:text-gray-400">{{ formatTime(item.window_start) }} → {{ formatTime(item.window_end) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            v-model:page="estimatePage"
            v-model:page-size="estimatePageSize"
            :total="estimateTotal"
            @update:page="loadEstimates(estimatePage)"
            @update:page-size="loadEstimates(1)"
          />
        </template>

        <template v-else>
          <div class="flex flex-wrap items-end gap-3 border-b border-gray-200 p-4 dark:border-dark-700">
            <label class="block">
              <span class="input-label">{{ t('admin.subscriptionLimits.history.filters.group') }}</span>
              <select v-model.number="adjustmentFilters.group_id" class="input w-48">
                <option :value="0">{{ t('admin.subscriptionLimits.history.filters.allGroups') }}</option>
                <option v-for="group in groupDrafts" :key="group.group_id" :value="group.group_id">{{ group.group_name }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.subscriptionLimits.history.filters.window') }}</span>
              <select v-model="adjustmentFilters.window_type" class="input w-32">
                <option value="">{{ t('admin.subscriptionLimits.history.filters.allWindows') }}</option>
                <option value="5h">5h</option>
                <option value="7d">7d</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.subscriptionLimits.history.filters.trigger') }}</span>
              <select v-model="adjustmentFilters.trigger_type" class="input w-48">
                <option value="">{{ t('admin.subscriptionLimits.history.filters.allTriggers') }}</option>
                <option value="threshold">{{ t('admin.subscriptionLimits.history.trigger.threshold') }}</option>
                <option value="config_update">{{ t('admin.subscriptionLimits.history.trigger.config_update') }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.subscriptionLimits.history.filters.userKeyword') }}</span>
              <input v-model.trim="adjustmentFilters.user_keyword" type="text" class="input w-56" :placeholder="t('admin.subscriptionLimits.history.filters.userKeywordPlaceholder')" />
            </label>
            <label class="flex items-center gap-2 pb-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="adjustmentFilters.only_changed" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              <span>{{ t('admin.subscriptionLimits.history.filters.onlyChanged') }}</span>
            </label>
            <button type="button" class="btn btn-secondary" :disabled="adjustmentLoading" @click="loadAdjustments(1)">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': adjustmentLoading }" />
              <span class="ml-2">{{ t('admin.subscriptionLimits.history.filters.refresh') }}</span>
            </button>
          </div>

          <div v-if="adjustmentLoading" class="px-6 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
          <div v-else-if="adjustments.length === 0" class="px-6 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.subscriptionLimits.history.adjustments.empty') }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="min-w-[1850px] w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
              <thead class="bg-gray-50 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:bg-dark-900/60 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.time') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.source') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.group') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.user') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.window') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.trigger') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.utilization') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.appliedEstimate') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.sampledEstimate') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.rule') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.oldLimit') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.newLimit') }}</th>
                  <th class="px-4 py-3">{{ t('admin.subscriptionLimits.history.adjustments.delta') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                <tr v-for="item in adjustments" :key="item.id" class="hover:bg-gray-50/70 dark:hover:bg-dark-800/40">
                  <td class="whitespace-nowrap px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatTime(item.created_at) }}</td>
                  <td class="px-4 py-3">{{ item.source_account_name || '—' }}<span class="ml-1 text-xs text-gray-400">#{{ item.source_account_id || '—' }}</span></td>
                  <td class="px-4 py-3">
                    <div class="font-medium text-gray-900 dark:text-white">{{ item.group_name_snapshot }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">#{{ item.group_id || '—' }}</div>
                  </td>
                  <td class="px-4 py-3">
                    <div v-if="item.user_id" class="text-gray-900 dark:text-white">{{ item.user_email || `#${item.user_id}` }}</div>
                    <div v-else class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.subscriptionLimits.history.adjustments.groupLevel') }}</div>
                    <div v-if="item.subscription_id" class="text-xs text-gray-500 dark:text-gray-400">sub #{{ item.subscription_id }}</div>
                  </td>
                  <td class="px-4 py-3"><span class="rounded bg-gray-100 px-2 py-1 font-mono text-xs dark:bg-dark-700">{{ item.window_type }}</span></td>
                  <td class="px-4 py-3">
                    <div>{{ triggerLabel(item.trigger_type) }}</div>
                    <div v-if="item.threshold_percent" class="text-xs text-gray-500 dark:text-gray-400">{{ item.threshold_percent }}%</div>
                  </td>
                  <td class="px-4 py-3">{{ formatNullablePercent(item.source_utilization_percent) }}</td>
                  <td class="px-4 py-3">
                    <div class="font-medium text-primary-600 dark:text-primary-300">{{ formatMoney(item.applied_estimate_usd) }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">{{ estimateSourceLabel(item.applied_estimate_source) }}</div>
                  </td>
                  <td class="px-4 py-3">{{ formatNullableMoney(item.source_estimated_full_usd) }}</td>
                  <td class="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">{{ formatRule(item) }}</td>
                  <td class="px-4 py-3">{{ formatNullableMoney(item.old_limit_usd) }}</td>
                  <td class="px-4 py-3 font-semibold text-gray-900 dark:text-white">{{ formatMoney(item.new_limit_usd) }}</td>
                  <td class="px-4 py-3" :class="item.changed ? 'text-primary-600 dark:text-primary-300' : 'text-gray-400'">
                    <div>{{ formatDelta(item.delta_usd) }}</div>
                    <div class="text-xs">{{ item.delta_percent == null ? '—' : `${item.delta_percent.toFixed(2)}%` }}</div>
                    <span class="mt-1 inline-block rounded px-1.5 py-0.5 text-[11px]" :class="item.changed ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'">
                      {{ item.changed ? t('admin.subscriptionLimits.history.adjustments.changed') : t('admin.subscriptionLimits.history.adjustments.unchanged') }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            v-model:page="adjustmentPage"
            v-model:page-size="adjustmentPageSize"
            :total="adjustmentTotal"
            @update:page="loadAdjustments(adjustmentPage)"
            @update:page-size="loadAdjustments(1)"
          />
        </template>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores'
import {
  getOverview,
  listLimitAdjustments,
  listQuotaEstimates,
  updateGroupConfig,
  type ClaudeHistoryFilters,
  type ClaudeSubscriptionAppliedEstimate,
  type ClaudeSubscriptionEstimateSource,
  type ClaudeSubscriptionGroupConfig,
  type ClaudeSubscriptionLimitAdjustmentHistory,
  type ClaudeSubscriptionLimitOverview,
  type ClaudeSubscriptionQuotaEstimateHistory,
  type ClaudeSubscriptionSourceAccount,
  type ClaudeSubscriptionTriggerType,
  type ClaudeSubscriptionWindowType
} from '@/api/admin/claudeSubscriptionLimits'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const loadError = ref('')
const overview = ref<ClaudeSubscriptionLimitOverview | null>(null)
const savingGroupId = ref<number | null>(null)
const groupDrafts = ref<ClaudeSubscriptionGroupConfig[]>([])

const sourceAccounts = computed(() => overview.value?.source_accounts ?? [])
const thresholdText = computed(() => {
  const values = overview.value?.thresholds?.length ? overview.value.thresholds : [50, 75, 85]
  return values.map((v) => `${v}%`).join(' / ')
})
const conflictGroups = computed(() => groupDrafts.value.filter((group) => group.source_status !== 'ok'))

interface ResolvedSourceSummary {
  account: ClaudeSubscriptionSourceAccount
  groupNames: string[]
  estimate5h?: ClaudeSubscriptionAppliedEstimate
  estimate7d?: ClaudeSubscriptionAppliedEstimate
}

const resolvedSourceSummaries = computed<ResolvedSourceSummary[]>(() => {
  const byId = new Map<number, ResolvedSourceSummary>()
  for (const group of groupDrafts.value) {
    if (group.source_status !== 'ok' || !group.source_account) continue
    const existing = byId.get(group.source_account.id)
    if (existing) {
      existing.groupNames.push(group.group_name)
    } else {
      byId.set(group.source_account.id, {
        account: group.source_account,
        groupNames: [group.group_name],
        estimate5h: group.applied_estimates?.['5h'],
        estimate7d: group.applied_estimates?.['7d']
      })
    }
  }
  return Array.from(byId.values())
})

const historyTab = ref<'estimates' | 'adjustments'>('estimates')
const estimates = ref<ClaudeSubscriptionQuotaEstimateHistory[]>([])
const estimateTotal = ref(0)
const estimatePage = ref(1)
const estimatePageSize = ref(20)
const estimateLoading = ref(false)
const estimateFilters = reactive({
  window_type: '' as '' | ClaudeSubscriptionWindowType,
  source_account_id: 0
})

const adjustments = ref<ClaudeSubscriptionLimitAdjustmentHistory[]>([])
const adjustmentTotal = ref(0)
const adjustmentPage = ref(1)
const adjustmentPageSize = ref(20)
const adjustmentLoading = ref(false)
const adjustmentFilters = reactive({
  group_id: 0,
  window_type: '' as '' | ClaudeSubscriptionWindowType,
  trigger_type: '' as '' | ClaudeSubscriptionTriggerType,
  user_keyword: '',
  only_changed: false
})

function cloneGroups(groups: ClaudeSubscriptionGroupConfig[]): ClaudeSubscriptionGroupConfig[] {
  return groups.map((group) => ({
    ...group,
    bound_accounts: [...(group.bound_accounts || [])],
    applied_estimates: { ...group.applied_estimates }
  }))
}

async function loadOverview() {
  loading.value = true
  loadError.value = ''
  try {
    const data = await getOverview()
    overview.value = data
    groupDrafts.value = cloneGroups(data.groups || [])
  } catch (error) {
    loadError.value = errorMessage(error)
  } finally {
    loading.value = false
  }
}

async function loadEstimates(page = estimatePage.value) {
  estimateLoading.value = true
  try {
    const filters: ClaudeHistoryFilters = {
      page,
      page_size: estimatePageSize.value,
      window_type: estimateFilters.window_type || undefined,
      source_account_id: estimateFilters.source_account_id || undefined
    }
    const data = await listQuotaEstimates(filters)
    estimates.value = data.items
    estimateTotal.value = data.total
    estimatePage.value = data.page
    estimatePageSize.value = data.page_size
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    estimateLoading.value = false
  }
}

async function loadAdjustments(page = adjustmentPage.value) {
  adjustmentLoading.value = true
  try {
    const filters: ClaudeHistoryFilters = {
      page,
      page_size: adjustmentPageSize.value,
      group_id: adjustmentFilters.group_id || undefined,
      window_type: adjustmentFilters.window_type || undefined,
      trigger_type: adjustmentFilters.trigger_type || undefined,
      user_keyword: adjustmentFilters.user_keyword || undefined,
      only_changed: adjustmentFilters.only_changed || undefined
    }
    const data = await listLimitAdjustments(filters)
    adjustments.value = data.items
    adjustmentTotal.value = data.total
    adjustmentPage.value = data.page
    adjustmentPageSize.value = data.page_size
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    adjustmentLoading.value = false
  }
}

async function refreshAll() {
  await loadOverview()
  await Promise.all([loadEstimates(1), loadAdjustments(1)])
}

async function saveGroup(group: ClaudeSubscriptionGroupConfig) {
  savingGroupId.value = group.group_id
  try {
    await updateGroupConfig(group.group_id, {
      auto_enabled: group.auto_enabled,
      initial_five_hour_full_usd: Number(group.initial_five_hour_full_usd),
      initial_seven_day_full_usd: Number(group.initial_seven_day_full_usd),
      five_hour_cap_percent: Number(group.five_hour_cap_percent),
      seven_day_cap_percent: Number(group.seven_day_cap_percent),
      five_hour_share_count: Number(group.five_hour_share_count),
      seven_day_share_count: Number(group.seven_day_share_count),
      five_hour_offset_usd: Number(group.five_hour_offset_usd),
      seven_day_offset_usd: Number(group.seven_day_offset_usd)
    })
    appStore.showSuccess(t('admin.subscriptionLimits.config.saved'))
    await loadOverview()
    await loadAdjustments(1)
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    savingGroupId.value = null
  }
}

function effectiveEstimateForPreview(
  group: ClaudeSubscriptionGroupConfig,
  windowType: ClaudeSubscriptionWindowType
): number {
  const applied = group.applied_estimates?.[windowType]
  if (applied && applied.source !== 'default' && Number.isFinite(applied.value_usd) && applied.value_usd > 0) {
    return applied.value_usd
  }
  const initial = windowType === '5h' ? Number(group.initial_five_hour_full_usd) : Number(group.initial_seven_day_full_usd)
  if (Number.isFinite(initial) && initial > 0) return initial
  return applied?.value_usd ?? 0
}

function previewLimit(group: ClaudeSubscriptionGroupConfig, windowType: ClaudeSubscriptionWindowType): number {
  const estimate = effectiveEstimateForPreview(group, windowType)
  const cap = windowType === '5h' ? Number(group.five_hour_cap_percent) : Number(group.seven_day_cap_percent)
  const shares = windowType === '5h' ? Number(group.five_hour_share_count) : Number(group.seven_day_share_count)
  const offset = windowType === '5h' ? Number(group.five_hour_offset_usd) : Number(group.seven_day_offset_usd)
  if (!Number.isFinite(estimate) || estimate <= 0 || !Number.isFinite(cap) || !Number.isFinite(shares) || shares <= 0) {
    return 0
  }
  return Math.max(1e-8, Math.round((estimate * cap / 100 / shares + (Number.isFinite(offset) ? offset : 0)) * 1e8) / 1e8)
}

function sourceStatusLabel(group: ClaudeSubscriptionGroupConfig): string {
  if (group.source_status === 'multiple') {
    return t('admin.subscriptionLimits.source.statusMultiple', { count: group.bound_accounts.length })
  }
  if (group.source_status === 'unsupported') {
    return t('admin.subscriptionLimits.source.statusUnsupported')
  }
  return t('admin.subscriptionLimits.source.statusMissing')
}

function estimateSourceLabel(source: ClaudeSubscriptionEstimateSource | undefined): string {
  if (source === 'current_cycle') return t('admin.subscriptionLimits.source.currentCycle')
  if (source === 'previous_cycle') return t('admin.subscriptionLimits.source.previousCycle')
  return t('admin.subscriptionLimits.source.defaultEstimate')
}

function estimateSourceBadge(estimate: ClaudeSubscriptionAppliedEstimate | undefined): string {
  const base = estimateSourceLabel(estimate?.source)
  if (estimate?.source && estimate.source !== 'default' && estimate.threshold_percent) {
    return `${base} ${estimate.threshold_percent}%`
  }
  return base
}

function estimateBadgeClass(source: ClaudeSubscriptionEstimateSource | undefined): string {
  if (source === 'current_cycle') {
    return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
  }
  if (source === 'previous_cycle') {
    return 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
  }
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function formatMoney(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return `$${Number(value).toFixed(4)}`
}

function formatNullableMoney(value: number | null | undefined): string {
  return value == null ? t('admin.subscriptionLimits.config.unlimited') : formatMoney(value)
}

function formatPercent(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return `${Number(value).toFixed(2)}%`
}

function formatNullablePercent(value: number | null | undefined): string {
  return value == null ? '—' : formatPercent(value)
}

function formatDelta(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return `${value >= 0 ? '+' : ''}${Number(value).toFixed(4)}`
}

function formatTime(value: string | null | undefined): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function formatRule(item: ClaudeSubscriptionLimitAdjustmentHistory): string {
  return `${item.cap_percent}% / ${item.share_count} / ${item.manual_offset_usd >= 0 ? '+' : ''}${item.manual_offset_usd}`
}

function triggerLabel(trigger: string): string {
  if (trigger === 'threshold') return t('admin.subscriptionLimits.history.trigger.threshold')
  if (trigger === 'config_update') return t('admin.subscriptionLimits.history.trigger.config_update')
  if (trigger === 'source_update') return t('admin.subscriptionLimits.history.trigger.source_update')
  return trigger
}

function errorMessage(error: unknown): string {
  if (typeof error === 'object' && error !== null) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string' && message.trim()) return message
  }
  return t('admin.subscriptionLimits.common.loadFailed')
}

onMounted(async () => {
  await refreshAll()
})
</script>
