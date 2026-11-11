export default {
  activityMonitor: {
    title: 'Activity Monitor',
    description: 'Observe when users and upstream accounts have request coverage across a selected date range.',
    scope: {
      label: 'Monitor scope',
      users: 'Users',
      accounts: 'Accounts'
    },
    controls: {
      dateRange: 'Date range',
      gap: 'Gap threshold',
      minutesValue: '{minutes} min',
      refresh: 'Refresh',
      timezone: 'Timezone'
    },
    kpi: {
      entities: 'Entities returned',
      entitiesHint: 'Number of entities in the loaded result, not the total number available.',
      requests: 'Total requests',
      requestsHint: 'Sum of each returned entity total; daily values are not added again.',
      highestCoverage: 'Highest one-day coverage',
      highestCoverageHint: 'Largest single-day active-window coverage among all returned entities.',
      longestSegment: 'Longest continuous segment',
      longestSegmentHint: 'Longest uninterrupted active window after applying the gap threshold.'
    },
    state: {
      loading: 'Loading activity timeline…',
      loadFailed: 'Failed to load the activity timeline',
      retry: 'Try again',
      emptyTitle: 'No activity in this range',
      emptyDescription: 'No request coverage was found for the selected scope, date range, and gap threshold.',
      unknownEntity: 'Unknown entity',
      noDetail: 'No detail available'
    },
    entityList: {
      title: 'Entities',
      loadedCount: '{count} loaded',
      searchLabel: 'Search loaded entities',
      searchPlaceholder: 'Search name or detail…',
      clearSearch: 'Clear search',
      requests: 'Requests',
      coverage: 'Coverage',
      activeDays: 'Active days',
      noMatches: 'No loaded entities match your search.'
    },
    summary: {
      requests: 'Requests',
      activeDays: 'Active days',
      activeHours: 'Active time',
      coverage: 'Window coverage',
      longestSegment: 'Longest segment'
    },
    chart: {
      title: '24-hour activity windows',
      description: 'Request-coverage windows on a local-time axis. Inactive days remain in the timeline as empty rows.',
      activeWindow: 'Active window',
      singleRequest: 'Single request / zero-length',
      overnight: 'Overnight',
      localTime: 'Local time · {timezone}',
      noActivity: 'No activity',
      gapHint: 'Windows are separated by request gaps longer than {minutes} minutes, as calculated by the server.',
      segmentTooltip: '{start}–{end} · {requests} requests · {duration}',
      dayAriaLabel: '{date}: {requests} requests, {segments} active windows, {coverage} coverage'
    },
    table: {
      title: 'Daily details',
      description: 'Every date in the selected range is listed, including days without activity.',
      date: 'Date',
      requests: 'Requests',
      activeHours: 'Active time',
      coverage: 'Window coverage',
      longestSegment: 'Longest segment',
      windows: 'Active windows',
      noWindows: 'No active windows'
    },
    units: {
      hoursValue: '{value} h',
      daysValue: '{count} days',
      requestsValue: '{count} req'
    },
    footnote: {
      title: 'Interpretation:',
      coverage: 'Active windows are inferred from usage-log request timestamps and represent request coverage only, not keyboard, online, or session time.',
      observational: 'This page is observational. Overlapping windows across users or accounts do not by themselves imply account sharing.'
    }
  }
}
