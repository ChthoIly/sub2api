export default {
  activityMonitor: {
    title: '活跃时段监控',
    description: '查看用户与上游账号在所选日期范围内的请求覆盖时段。',
    scope: {
      label: '监控范围',
      users: '用户',
      accounts: '上游账号'
    },
    controls: {
      dateRange: '日期范围',
      gap: '间隔阈值',
      minutesValue: '{minutes} 分钟',
      refresh: '刷新',
      timezone: '时区'
    },
    kpi: {
      entities: '返回实体数',
      entitiesHint: '本次加载结果中的实体数量，不代表全部实体总数。',
      requests: '请求总数',
      requestsHint: '按返回实体的总计求和，不重复累加每日字段。',
      highestCoverage: '单日最高覆盖',
      highestCoverageHint: '所有返回实体中，单日活跃窗口覆盖率的最大值。',
      longestSegment: '最长连续时段',
      longestSegmentHint: '应用间隔阈值后，持续时间最长的连续活跃窗口。'
    },
    state: {
      loading: '正在加载活跃时间线…',
      loadFailed: '加载活跃时间线失败',
      retry: '重试',
      emptyTitle: '所选范围内暂无活跃记录',
      emptyDescription: '当前监控范围、日期范围和间隔阈值下未找到请求覆盖记录。',
      unknownEntity: '未知实体',
      noDetail: '暂无详情'
    },
    entityList: {
      title: '实体列表',
      loadedCount: '已加载 {count} 个',
      searchLabel: '搜索已加载实体',
      searchPlaceholder: '搜索名称或详情…',
      clearSearch: '清除搜索',
      requests: '请求',
      coverage: '覆盖时长',
      activeDays: '活跃天数',
      noMatches: '没有已加载实体匹配当前搜索。'
    },
    summary: {
      requests: '请求数',
      activeDays: '活跃天数',
      activeHours: '活跃时长',
      coverage: '窗口覆盖',
      longestSegment: '最长时段'
    },
    chart: {
      title: '24 小时活跃窗口',
      description: '按本地时间轴展示请求覆盖窗口；无活跃记录的日期仍会在时间线中保留为空行。',
      activeWindow: '活跃窗口',
      singleRequest: '单次请求 / 零时长',
      overnight: '夜间时段',
      localTime: '本地时间 · {timezone}',
      noActivity: '无活跃记录',
      gapHint: '请求间隔超过 {minutes} 分钟时，服务端会将其拆分为不同窗口。',
      segmentTooltip: '{start}–{end} · {requests} 次请求 · {duration}',
      dayAriaLabel: '{date}：{requests} 次请求，{segments} 个活跃窗口，覆盖 {coverage}'
    },
    table: {
      title: '每日明细',
      description: '列出所选范围内的每一天，包括无活跃记录的日期。',
      date: '日期',
      requests: '请求数',
      activeHours: '活跃时长',
      coverage: '窗口覆盖',
      longestSegment: '最长时段',
      windows: '活跃窗口',
      noWindows: '无活跃窗口'
    },
    units: {
      hoursValue: '{value} 小时',
      daysValue: '{count} 天',
      requestsValue: '{count} 次'
    },
    footnote: {
      title: '说明：',
      coverage: '活跃窗口根据 usage_logs 请求时间戳推断，仅表示请求覆盖，不代表键盘操作、在线时长或会话时长。',
      observational: '本页面仅作观察之用。用户或账号之间的窗口重叠本身并不表示存在共享账号行为。'
    }
  }
}
