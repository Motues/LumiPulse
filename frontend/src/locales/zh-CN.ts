import { registerMessages } from '../composables/useI18n'

/** 公开页面与页脚共用的文案。管理后台文案在后续批次迁移。 */
export const zhCN: Record<string, string> = {
  // 站点元信息（标题 / 分享描述）
  'site.statusSuffix': '系统状态',
  'site.description': '{name} 服务监控系统 —— 实时展示服务可用性、SLA 历史与运维公告。',

  // 通用
  'common.loading': '加载中...',
  'common.dataRefreshFailed': '数据刷新失败',
  'common.loadFailed': '加载数据失败',
  'common.backToStatus': '返回状态页',
  'common.close': '关闭',
  'common.copy': '复制',
  'common.cst': 'CST',
  'common.notFoundService': '未找到该服务，可能已被删除或不再在首页展示。',
  'common.adminPanel': '管理后台',

  // 页头 / 主题菜单 / 语言
  'header.subscribe': '订阅更新',
  'header.switchTheme': '切换主题',
  'header.theme.light': '浅色模式',
  'header.theme.dark': '深色模式',
  'header.theme.system': '跟随系统',
  'header.language': '语言',

  // 首页
  'home.allOperational': '所有系统运行正常',
  'home.outage': '系统出现故障',
  'home.systemStatus': '系统状态',
  'home.maintenances': '维护计划',
  'home.noMaintenance': '暂无计划的维护',
  'home.noMaintenanceHint': '我们会提前通知受影响的服务维护计划。',
  'home.pastIncidents': '过去事件',
  'home.viewServiceDetail': '查看服务详情',
  'home.affectedServices': '受影响服务',
  'home.createdAt': '创建时间',
  'home.updatedAt': '最后更新',
  'home.incidentTimeline': '事件时间线',
  'home.noUpdates': '暂无更新记录',
  'home.invalidIncident': '无效的事件标识',
  'home.loadIncidentFailed': '加载事件详情失败',
  'home.serviceLabel': '服务 #{id}',
  'home.poweredBy': 'Powered By LumiPulse',

  // 服务状态
  'status.operational': '正常',
  'status.degraded': '异常',
  'status.outage': '故障',
  'status.maintenance': '维护中',
  'status.noData': '无数据',
  'status.healthyToday': '服务运行正常',
  'status.downtimeLabel': '异常时间：{duration}',
  'status.daysAgo': '{n} 天前',
  'status.today': '今天',
  'status.uptimeLabel': '{n}% 在线率',

  // 事件
  'incident.status.investigating': '调查中',
  'incident.status.identified': '已确认',
  'incident.status.monitoring': '监控中',
  'incident.status.resolved': '已解决',
  'incident.impact.minor': '轻微',
  'incident.impact.major': '重大',
  'incident.impact.critical': '严重',

  // 服务详情
  'service.uptime': '在线率',
  'service.responseTime': '响应时间',
  'service.probeInterval': '探测频率',
  'service.latency24h': '最近24小时延迟',
  'service.latencySamples': '共 {n} 个成功样本',
  'service.failures': '故障 {n} 次',
  'service.avgLatency': '平均延迟',
  'service.p95': 'P95',
  'service.p99': 'P99',
  'service.peak': '峰值',
  'service.noData': '暂无数据',
  'service.history': '服务历史',
  'service.maintenanceOngoing': '维护中',

  // 延迟图表
  'chart.normal': '正常',
  'chart.failure': '故障',
  'chart.noData': '无数据',

  // 订阅
  'subscribe.title': '订阅更新',
  'subscribe.tab.email': '邮件订阅',
  'subscribe.desc': '获取服务状态变更和事件通知。',
  'subscribe.emailPlaceholder': '输入邮箱地址...',
  'subscribe.submit': '订阅',
  'subscribe.submitting': '提交中...',
  'subscribe.success': '订阅成功！我们将通过邮件通知您服务状态变更。',
  'subscribe.failed': '订阅失败，请稍后重试',
  'subscribe.selectServices': '选择特定服务',
  'subscribe.collapse': '收起',
  'subscribe.selectAll': '全选',
  'subscribe.clearAll': '取消全选',
  'subscribe.emptyMeansAll': '留空则订阅所有服务',
  'subscribe.feedDesc': '复制以下链接到 RSS 阅读器订阅状态更新。',
  'subscribe.copied': '链接已复制到剪贴板',

  // 时长
  'duration.minutes': '{n}分钟',
  'duration.hoursMinutes': '{h}小时{m}分钟',
  'duration.hoursOnly': '{h}小时',
}

registerMessages('zh-CN', zhCN)
