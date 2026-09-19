import { registerMessages } from '../composables/useI18n'

/**
 * 管理后台侧边栏导航与页面标题文案（由 views/DashboardPage.vue 使用）。
 * 命名空间：admin.nav。
 */
export const adminNavZh: Record<string, string> = {
  'admin.nav.dashboard': '控制台',
  'admin.nav.groupMonitoring': '监控管理',
  'admin.nav.services': '服务管理',
  'admin.nav.serviceFolders': '服务分组',
  'admin.nav.probes': '探测任务',
  'admin.nav.logs': '监控日志',
  'admin.nav.sla': 'SLA 报告',
  'admin.nav.groupIncidents': '事件与维护',
  'admin.nav.incidents': '事件管理',
  'admin.nav.maintenances': '维护计划',
  'admin.nav.groupSystem': '系统管理',
  'admin.nav.users': '用户管理',
  'admin.nav.notifications': '通知管理',
  'admin.nav.settings': '系统设置',
  'admin.nav.apiKeys': 'API 密钥',
  'admin.nav.subscribers': '订阅管理',
}

export const adminNavEn: Record<string, string> = {
  'admin.nav.dashboard': 'Dashboard',
  'admin.nav.groupMonitoring': 'Monitoring',
  'admin.nav.services': 'Services',
  'admin.nav.serviceFolders': 'Service groups',
  'admin.nav.probes': 'Probe tasks',
  'admin.nav.logs': 'Logs',
  'admin.nav.sla': 'SLA reports',
  'admin.nav.groupIncidents': 'Incidents & maintenance',
  'admin.nav.incidents': 'Incidents',
  'admin.nav.maintenances': 'Maintenance',
  'admin.nav.groupSystem': 'System',
  'admin.nav.users': 'Users',
  'admin.nav.notifications': 'Notifications',
  'admin.nav.settings': 'Settings',
  'admin.nav.apiKeys': 'API keys',
  'admin.nav.subscribers': 'Subscribers',
}

registerMessages('zh-CN', adminNavZh)
registerMessages('en-US', adminNavEn)
