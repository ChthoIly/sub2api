export default {
  subscriptionLimits: {
    title: 'Claude Subscription Dynamic Limits',
    description: 'Automatically derive per-user subscription group limits from each group\'s uniquely bound upstream source account when passive 5h / 7d utilization reaches 50%, 75%, and 85%.',
    rules: {
      title: 'Engine Rules & Resolved Source Accounts',
      thresholds: 'Thresholds: {thresholds} (5h and 7d run independently; crossing 50% takes effect immediately and carries over until the next cycle hits 50%, with refreshes at 75% and 85%)',
      sourceBinding: 'Source binding: each subscription group automatically resolves its own bound upstream account. A group must bind exactly one Anthropic OAuth / Setup Token account; multi-account bindings trigger a warning and block or pause auto calculation.',
      initialDefault: 'Initial defaults: admin-configured 5h / 7d full-quota defaults apply until a window records its first >= 50% self-driving estimate, which then overrides the default.',
      noResolvedSources: 'No subscription group currently resolves to a unique Anthropic OAuth / Setup Token source account.'
    },
    source: {
      boundGroups: 'Groups: {groups}',
      resetAt: 'Resets at {time}',
      appliedEstimate: 'Effective full estimate',
      currentCycle: 'Current cycle',
      previousCycle: 'Previous cycle',
      defaultEstimate: 'Initial default',
      sampledAt: 'Sampled: {time}',
      waitingFirstSample: 'Waiting for first 50% sample',
      noSnapshot: 'No passive snapshot yet',
      uniqueSource: 'Source account',
      statusMissing: 'No bound account',
      statusMultiple: 'Multiple bound accounts ({count})',
      statusUnsupported: 'Unsupported account type',
      pausedBadge: 'Auto-refresh paused',
      conflictBannerTitle: '{count} subscription group(s) have invalid source account bindings',
      conflictBannerHint: 'Dynamic limits cannot be enabled for these groups. Previously enabled groups are paused automatically without changing current limits until bound to a single Anthropic OAuth / Setup Token account.'
    },
    config: {
      title: 'Subscription Group Dynamic Limit Configuration',
      hint: 'Each subscription group manages its own initial full-quota defaults and formula (effective full estimate × cap % ÷ shares + manual offset). Saving while disabled leaves current group limits unchanged.',
      subscriptions: '{count} active subscriptions',
      window: 'Window',
      currentLimit: 'Current limit',
      appliedEstimate: 'Effective full estimate',
      initialFull: 'Initial default full (USD)',
      initialActive: 'Active fallback',
      initialOverridden: 'Overridden by dynamic data',
      cap: 'Cap %',
      shares: 'Shares',
      offset: 'Offset (USD)',
      previewLimit: 'Expected limit',
      autoEnabled: 'Auto calculation',
      save: 'Save config',
      saved: 'Group configuration saved',
      empty: 'No subscription groups',
      unlimited: 'Not set',
      cannotEnableHint: 'Fix the group\'s upstream account binding before enabling dynamic limits',
      pausedSaveHint: 'Source account binding is invalid. Disable auto calculation to save, or fix the group binding first.'
    },
    history: {
      title: 'History',
      estimatesTab: 'Account Full-Quota Estimates',
      adjustmentsTab: 'User Limit Adjustments',
      filters: {
        window: 'Window',
        allWindows: 'All windows',
        source: 'Source account',
        allSources: 'All source accounts',
        group: 'Group',
        allGroups: 'All groups',
        trigger: 'Trigger',
        allTriggers: 'All triggers',
        userKeyword: 'User email',
        userKeywordPlaceholder: 'Search user email',
        onlyChanged: 'Changed only',
        refresh: 'Refresh'
      },
      estimates: {
        time: 'Time',
        source: 'Source account',
        window: 'Window',
        threshold: 'Threshold',
        utilization: 'Utilization',
        windowCost: 'Window cost',
        estimatedFull: 'Full estimate',
        cycleRange: 'Usage cycle',
        empty: 'No full-quota estimates'
      },
      adjustments: {
        time: 'Time',
        source: 'Source account',
        group: 'Group',
        user: 'User',
        window: 'Window',
        trigger: 'Trigger',
        utilization: 'Source utilization',
        appliedEstimate: 'Applied full estimate',
        sampledEstimate: 'Sample full estimate',
        rule: 'Cap / shares / offset',
        oldLimit: 'Before',
        newLimit: 'After',
        delta: 'Change',
        empty: 'No limit adjustments',
        groupLevel: 'Group level (no active subscriptions)',
        changed: 'Changed',
        unchanged: 'Unchanged'
      },
      trigger: {
        threshold: 'Usage threshold',
        config_update: 'Config update',
        source_update: 'Source update'
      }
    },
    common: {
      refresh: 'Refresh',
      loadFailed: 'Failed to load',
      operationFailed: 'Operation failed',
      saved: 'Saved'
    }
  }
}
