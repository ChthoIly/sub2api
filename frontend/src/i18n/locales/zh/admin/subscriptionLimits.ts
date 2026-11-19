export default {
  subscriptionLimits: {
    title: 'Claude 订阅动态限额',
    description: '按各订阅分组绑定的唯一上游源账号被动 5h / 7d 用量窗口，在使用率达到 50% / 75% / 85% 时自动计算并立即刷新分组每人限额。',
    rules: {
      title: '运行规则与已关联源账号',
      thresholds: '触发阈值：{thresholds}（5h 与 7d 窗口各自独立，跨过 50% 即刻生效并延续到下期，75% / 85% 各刷新一次）',
      sourceBinding: '源账号归属：由各订阅分组实际绑定的上游账号自动决定，每个分组只能绑定 1 个 Anthropic OAuth / Setup Token 账号，多账号绑定将告警并拒绝开启或暂停刷新。',
      initialDefault: '默认初始值：窗口首次跨过 50% 获得自我驱动数据前，按管理员填写的 5h / 7d 初始满额默认值计算；有有效估算后自动覆盖默认值。',
      noResolvedSources: '当前暂无满足唯一 Anthropic OAuth / Setup Token 绑定条件的订阅分组。'
    },
    source: {
      boundGroups: '关联分组：{groups}',
      resetAt: '重置时间：{time}',
      appliedEstimate: '有效满额估算',
      currentCycle: '本周期',
      previousCycle: '历史周期',
      defaultEstimate: '默认初始值',
      sampledAt: '采样时间：{time}',
      waitingFirstSample: '待首次达到 50%',
      noSnapshot: '暂无被动采样数据',
      uniqueSource: '唯一源账号',
      statusMissing: '未绑定上游账号',
      statusMultiple: '绑定了多个账号（{count} 个）',
      statusUnsupported: '非 Anthropic OAuth / Setup Token',
      pausedBadge: '自动刷新已暂停',
      conflictBannerTitle: '检测到 {count} 个订阅分组的源账号绑定异常',
      conflictBannerHint: '这些分组无法开启动态限额；若此前已开启，后台已自动暂停刷新并保持现有额度不变，请在分组管理调整为单一 Anthropic OAuth / Setup Token 账号。'
    },
    config: {
      title: '订阅分组动态限额配置',
      hint: '每个订阅类型分组独立维护初始满额默认值与换算参数（生效满额估算 × 上限% ÷ 平分份数 + 手动偏移）。关闭自动计算时保存参数不会改动当前限额。',
      subscriptions: '{count} 个活跃订阅',
      window: '窗口',
      currentLimit: '当前限额',
      appliedEstimate: '生效满额估算',
      initialFull: '默认初始满额 (USD)',
      initialActive: '当前生效中',
      initialOverridden: '已被动态数据覆盖',
      cap: '上限 %',
      shares: '平分份数',
      offset: '手动偏移 (USD)',
      previewLimit: '预计限额',
      autoEnabled: '自动计算',
      save: '保存配置',
      saved: '分组配置已保存',
      empty: '暂无订阅类型分组',
      unlimited: '未设置',
      cannotEnableHint: '请先修正该分组的源账号绑定关系后再开启动态限额',
      pausedSaveHint: '源账号绑定异常，请先关闭自动计算后再保存，或先在分组管理修正绑定关系'
    },
    history: {
      title: '历史记录',
      estimatesTab: '账号满额预估历史',
      adjustmentsTab: '用户限额调整历史',
      filters: {
        window: '窗口',
        allWindows: '全部窗口',
        source: '源账号',
        allSources: '全部源账号',
        group: '分组',
        allGroups: '全部分组',
        trigger: '触发来源',
        allTriggers: '全部触发来源',
        userKeyword: '用户邮箱',
        userKeywordPlaceholder: '搜索用户邮箱',
        onlyChanged: '仅看额度变化',
        refresh: '刷新'
      },
      estimates: {
        time: '时间',
        source: '源账号',
        window: '窗口',
        threshold: '阈值',
        utilization: '使用率',
        windowCost: '窗口费用',
        estimatedFull: '满额估算',
        cycleRange: '用量周期',
        empty: '暂无满额预估记录'
      },
      adjustments: {
        time: '时间',
        source: '源账号',
        group: '分组',
        user: '用户',
        window: '窗口',
        trigger: '触发',
        utilization: '源使用率',
        appliedEstimate: '采用的满额估算',
        sampledEstimate: '本次采样估算',
        rule: '比例 / 份数 / 偏移',
        oldLimit: '调整前',
        newLimit: '调整后',
        delta: '浮动',
        empty: '暂无限额调整记录',
        groupLevel: '分组级（无活跃订阅）',
        changed: '已变化',
        unchanged: '未变化'
      },
      trigger: {
        threshold: '用量阈值',
        config_update: '配置更新',
        source_update: '源账号更新'
      }
    },
    common: {
      refresh: '刷新',
      loadFailed: '加载失败',
      operationFailed: '操作失败',
      saved: '保存成功'
    }
  }
}
