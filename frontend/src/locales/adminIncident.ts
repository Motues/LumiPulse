import { registerMessages } from '../composables/useI18n'

/**
 * 管理后台：事件（Incident）、维护计划（Maintenance）与自研日期选择器（DateTimePicker）的文案。
 *
 * 命名空间约定：本文件只允许出现 `admin.incident.` / `admin.maintenance.` / `admin.datepicker.`
 * 前缀的 key，避免与其他批次的管理端文案冲突。
 * 事件状态 / 影响等级直接复用公开页面的 `incident.status.*` / `incident.impact.*`，
 * 保证管理端与公开页面的用词一致。
 */

export const adminIncidentZh: Record<string, string> = {
  // ---- 事件管理：列表 ----
  'admin.incident.listTitle': '事件管理',
  'admin.incident.create': '创建事件',
  'admin.incident.editIncident': '编辑事件',
  'admin.incident.incidentUpdate': '事件更新',
  'admin.incident.backToList': '返回列表',

  // 表头
  'admin.incident.colTitle': '标题',
  'admin.incident.colImpact': '影响',
  'admin.incident.colAck': '确认',
  'admin.incident.colStatus': '状态',
  'admin.incident.colTime': '时间',
  'admin.incident.colActions': '操作',

  // 通用操作
  'admin.incident.edit': '编辑',
  'admin.incident.delete': '删除',
  'admin.incident.merge': '合并',
  'admin.incident.cancel': '取消',
  'admin.incident.save': '保存',
  'admin.incident.submit': '提交',
  'admin.incident.selectPlaceholder': '请选择...',

  // 分页
  'admin.incident.prevPage': '上一页',
  'admin.incident.nextPage': '下一页',
  'admin.incident.pageOf': '第 {page} / {total} 页',

  // 人工确认
  'admin.incident.unacknowledged': '未确认',
  'admin.incident.unacknowledgedEscalated': '未确认（已升级 {n} 次）',
  'admin.incident.acknowledged': '已确认',
  'admin.incident.acknowledgedAt': '已确认 {detail}',
  'admin.incident.ackAction': '确认事件',
  'admin.incident.ackSuccess': '已确认',
  'admin.incident.unackSuccess': '已取消确认',
  'admin.incident.confirmUnack': '取消确认后，未处理时将会重新收到升级通知，确定取消吗？',
  'admin.incident.ackTooltipResolved': '事件已解决，无需确认',
  'admin.incident.ackTooltipUnack': '点击取消确认',
  'admin.incident.ackTooltipAck': '点击确认，停止升级通知',
  'admin.incident.ackTooltipConfirm': '确认后停止升级通知',
  'admin.incident.awaitingAck': '等待人工确认',
  'admin.incident.escalatedTimes': '已升级 {n} 次',
  'admin.incident.adminDefault': '管理员',

  // 详情信息
  'admin.incident.fieldStatus': '当前状态',
  'admin.incident.fieldAck': '人工确认',
  'admin.incident.fieldAffectedServices': '关联服务',
  'admin.incident.fieldCreatedAt': '创建时间',
  'admin.incident.fieldUpdatedAt': '最后更新',
  'admin.incident.unknownService': '未知',

  // 子事件 / 时间线
  'admin.incident.childrenTitle': '子事件（{n}）',
  'admin.incident.splitOut': '拆分出去',
  'admin.incident.timeline': '事件经过',
  'admin.incident.addUpdate': '添加更新',
  'admin.incident.updateContentPlaceholder': '输入更新内容...',

  // 表单字段
  'admin.incident.labelTitle': '标题 *',
  'admin.incident.labelImpact': '影响等级 *',
  'admin.incident.labelStatus': '状态',
  'admin.incident.labelStatusRequired': '状态 *',
  'admin.incident.labelService': '服务 *',
  'admin.incident.labelContent': '内容 *',
  'admin.incident.labelSelectIncident': '选择事件 *',
  'admin.incident.searchServicePlaceholder': '搜索服务...',
  'admin.incident.noMatchingService': '无匹配服务',
  'admin.incident.updateEventFor': '更新事件: {title}',

  // 合并
  'admin.incident.mergeTitle': '合并事件',
  'admin.incident.mergeHint': '选择要合并至「{title}」的事件',
  'admin.incident.noMergeableIncident': '没有可合并的事件',
  'admin.incident.cannotMergeResolved': '已解决的事件不能合并',
  'admin.incident.mergeSuccess': '合并成功',
  'admin.incident.mergeFailed': '合并失败',
  'admin.incident.splitSuccess': '拆分成功',
  'admin.incident.splitFailed': '拆分失败',

  // 事后复盘
  'admin.incident.postmortemPublic': '对外公开',
  'admin.incident.postmortemInternal': '仅内部可见',
  'admin.incident.postmortemUrlLabel': '复盘文档',
  'admin.incident.postmortemUrl': '复盘文档链接',
  'admin.incident.postmortemPublicToggle': '对外公开复盘',
  'admin.incident.postmortemPublicHint': '关闭时公开页面不展示以上内容（默认关闭）',
  'admin.incident.rootCausePlaceholder': '例如：数据库连接池被慢查询耗尽',
  'admin.incident.resolutionPlaceholder': '例如：扩容连接池并给慢查询加上索引',

  // 提示 / 错误
  'admin.incident.loadFailed': '加载失败',
  'admin.incident.loadDetailFailed': '加载事件详情失败',
  'admin.incident.actionFailed': '操作失败',
  'admin.incident.updateSuccess': '更新成功',
  'admin.incident.createSuccess': '创建成功',
  'admin.incident.saveFailed': '保存失败',
  'admin.incident.updateFailed': '更新失败',
  'admin.incident.deleteSuccess': '删除成功',
  'admin.incident.deleteFailed': '删除失败',
  'admin.incident.confirmDelete': '确定要删除吗？',
  'admin.incident.confirmDeleteIncident': '确定要删除此事件吗？',
  'admin.incident.confirmDeleteUpdate': '确定要删除此更新记录吗？',
  'admin.incident.confirmDiscardChanges': '有未保存的更改，确定要关闭吗？',

  // ---- 维护计划 ----
  'admin.maintenance.listTitle': '维护计划',
  'admin.maintenance.create': '创建维护',
  'admin.maintenance.editMaintenance': '编辑维护',
  'admin.maintenance.colTitle': '标题',
  'admin.maintenance.colStatus': '状态',
  'admin.maintenance.colStart': '开始时间',
  'admin.maintenance.colEnd': '结束时间',
  'admin.maintenance.colAffectedServices': '受影响服务',
  'admin.maintenance.colActions': '操作',
  'admin.maintenance.edit': '编辑',
  'admin.maintenance.delete': '删除',
  'admin.maintenance.cancel': '取消',
  'admin.maintenance.save': '保存',

  // 表单字段
  'admin.maintenance.labelTitle': '标题 *',
  'admin.maintenance.labelDescription': '描述',
  'admin.maintenance.labelStart': '开始时间 *',
  'admin.maintenance.labelEnd': '结束时间 *',
  'admin.maintenance.startPlaceholder': '选择开始时间',
  'admin.maintenance.endPlaceholder': '选择结束时间',
  'admin.maintenance.labelStatus': '状态',
  'admin.maintenance.labelAffectedServices': '受影响服务',
  'admin.maintenance.searchServicePlaceholder': '搜索服务...',
  'admin.maintenance.noMatchingService': '无匹配服务',

  // 维护状态
  'admin.maintenance.status.scheduled': '计划中',
  'admin.maintenance.status.inProgress': '进行中',
  'admin.maintenance.status.completed': '已完成',
  'admin.maintenance.status.cancelled': '已取消',

  // 周期重复
  'admin.maintenance.labelRecurrence': '重复',
  'admin.maintenance.recurrence.none': '不重复（一次性）',
  'admin.maintenance.recurrence.daily': '每天',
  'admin.maintenance.recurrence.weekly': '每周',
  'admin.maintenance.recurrence.monthly': '每月',
  'admin.maintenance.recurrence.everyNDays': '每 {n} 天',
  'admin.maintenance.recurrence.weeklyOn': '每周{day}',
  'admin.maintenance.recurrence.everyNWeeksOn': '每 {n} 周{day}',
  'admin.maintenance.recurrence.everyNMonths': '每 {n} 个月',
  'admin.maintenance.recurrence.until': '至 {date}',
  'admin.maintenance.labelRecurrenceUntil': '重复截止',
  'admin.maintenance.recurrenceUntilPlaceholder': '留空则一直重复',
  'admin.maintenance.labelMonthday': '每月第几天',
  'admin.maintenance.monthdayPlaceholder': '留空则按开始日期',
  'admin.maintenance.intervalDays': '每隔几天',
  'admin.maintenance.intervalWeeks': '每隔几周',
  'admin.maintenance.intervalMonths': '每隔几个月',
  'admin.maintenance.hintWeekly': '重复日期由开始时间的星期决定（例如开始时间选在周日 02:00，则每周日 02:00 重复）。',
  'admin.maintenance.hintAdvance': '窗口结束后服务端会自动把开始/结束时间推进到下一个窗口，状态回到「计划中」。',
  'admin.maintenance.hintUntilEmpty': '重复截止留空表示一直重复。',

  // 星期（1=周一 … 7=周日）
  'admin.maintenance.weekday.mon': '周一',
  'admin.maintenance.weekday.tue': '周二',
  'admin.maintenance.weekday.wed': '周三',
  'admin.maintenance.weekday.thu': '周四',
  'admin.maintenance.weekday.fri': '周五',
  'admin.maintenance.weekday.sat': '周六',
  'admin.maintenance.weekday.sun': '周日',

  // 提示 / 错误
  'admin.maintenance.loadFailed': '加载失败',
  'admin.maintenance.updateSuccess': '更新成功',
  'admin.maintenance.createSuccess': '创建成功',
  'admin.maintenance.saveFailed': '保存失败',
  'admin.maintenance.confirmDelete': '确定要删除吗？',
  'admin.maintenance.deleteSuccess': '删除成功',
  'admin.maintenance.deleteFailed': '删除失败',

  // ---- 日期选择器 ----
  'admin.datepicker.placeholder': '请选择',
  'admin.datepicker.prevYear': '上一年',
  'admin.datepicker.prevMonth': '上一月',
  'admin.datepicker.nextMonth': '下一月',
  'admin.datepicker.nextYear': '下一年',
  'admin.datepicker.yearOnly': '{year} 年',
  'admin.datepicker.yearMonth': '{year} 年 {month}',
  'admin.datepicker.now': '此刻',
  'admin.datepicker.today': '今天',
  'admin.datepicker.thisMonth': '本月',
  'admin.datepicker.clear': '清空',
  'admin.datepicker.weekday.mon': '一',
  'admin.datepicker.weekday.tue': '二',
  'admin.datepicker.weekday.wed': '三',
  'admin.datepicker.weekday.thu': '四',
  'admin.datepicker.weekday.fri': '五',
  'admin.datepicker.weekday.sat': '六',
  'admin.datepicker.weekday.sun': '日',
  'admin.datepicker.monthJan': '1 月',
  'admin.datepicker.monthFeb': '2 月',
  'admin.datepicker.monthMar': '3 月',
  'admin.datepicker.monthApr': '4 月',
  'admin.datepicker.monthMay': '5 月',
  'admin.datepicker.monthJun': '6 月',
  'admin.datepicker.monthJul': '7 月',
  'admin.datepicker.monthAug': '8 月',
  'admin.datepicker.monthSep': '9 月',
  'admin.datepicker.monthOct': '10 月',
  'admin.datepicker.monthNov': '11 月',
  'admin.datepicker.monthDec': '12 月',
}

export const adminIncidentEn: Record<string, string> = {
  // ---- Incident manager: list ----
  'admin.incident.listTitle': 'Incidents',
  'admin.incident.create': 'Create incident',
  'admin.incident.editIncident': 'Edit incident',
  'admin.incident.incidentUpdate': 'Incident update',
  'admin.incident.backToList': 'Back to list',

  // Table headers
  'admin.incident.colTitle': 'Title',
  'admin.incident.colImpact': 'Impact',
  'admin.incident.colAck': 'Acknowledged',
  'admin.incident.colStatus': 'Status',
  'admin.incident.colTime': 'Time',
  'admin.incident.colActions': 'Actions',

  // Common actions
  'admin.incident.edit': 'Edit',
  'admin.incident.delete': 'Delete',
  'admin.incident.merge': 'Merge',
  'admin.incident.cancel': 'Cancel',
  'admin.incident.save': 'Save',
  'admin.incident.submit': 'Submit',
  'admin.incident.selectPlaceholder': 'Select...',

  // Pagination
  'admin.incident.prevPage': 'Previous',
  'admin.incident.nextPage': 'Next',
  'admin.incident.pageOf': 'Page {page} of {total}',

  // Manual acknowledgment
  'admin.incident.unacknowledged': 'Unacknowledged',
  'admin.incident.unacknowledgedEscalated': 'Unacknowledged ({n} escalations)',
  'admin.incident.acknowledged': 'Acknowledged',
  'admin.incident.acknowledgedAt': 'Acknowledged {detail}',
  'admin.incident.ackAction': 'Acknowledge',
  'admin.incident.ackSuccess': 'Acknowledged',
  'admin.incident.unackSuccess': 'Acknowledgment removed',
  'admin.incident.confirmUnack': 'If you remove the acknowledgment, escalation notifications resume while the incident is unresolved. Remove it?',
  'admin.incident.ackTooltipResolved': 'Incident resolved, no acknowledgment needed',
  'admin.incident.ackTooltipUnack': 'Click to remove the acknowledgment',
  'admin.incident.ackTooltipAck': 'Click to acknowledge and stop escalation notifications',
  'admin.incident.ackTooltipConfirm': 'Acknowledging stops escalation notifications',
  'admin.incident.awaitingAck': 'Awaiting acknowledgment',
  'admin.incident.escalatedTimes': 'Escalated {n} times',
  'admin.incident.adminDefault': 'Administrator',

  // Detail info
  'admin.incident.fieldStatus': 'Current status',
  'admin.incident.fieldAck': 'Acknowledgment',
  'admin.incident.fieldAffectedServices': 'Affected services',
  'admin.incident.fieldCreatedAt': 'Created',
  'admin.incident.fieldUpdatedAt': 'Last updated',
  'admin.incident.unknownService': 'Unknown',

  // Sub-incidents / timeline
  'admin.incident.childrenTitle': 'Sub-incidents ({n})',
  'admin.incident.splitOut': 'Split out',
  'admin.incident.timeline': 'Incident timeline',
  'admin.incident.addUpdate': 'Add update',
  'admin.incident.updateContentPlaceholder': 'Enter update content...',

  // Form fields
  'admin.incident.labelTitle': 'Title *',
  'admin.incident.labelImpact': 'Impact *',
  'admin.incident.labelStatus': 'Status',
  'admin.incident.labelStatusRequired': 'Status *',
  'admin.incident.labelService': 'Service *',
  'admin.incident.labelContent': 'Content *',
  'admin.incident.labelSelectIncident': 'Select incident *',
  'admin.incident.searchServicePlaceholder': 'Search services...',
  'admin.incident.noMatchingService': 'No matching services',
  'admin.incident.updateEventFor': 'Updating incident: {title}',

  // Merge
  'admin.incident.mergeTitle': 'Merge incidents',
  'admin.incident.mergeHint': 'Select the incident to merge into “{title}”',
  'admin.incident.noMergeableIncident': 'No incidents available to merge',
  'admin.incident.cannotMergeResolved': 'Resolved incidents cannot be merged',
  'admin.incident.mergeSuccess': 'Merged successfully',
  'admin.incident.mergeFailed': 'Failed to merge',
  'admin.incident.splitSuccess': 'Split successfully',
  'admin.incident.splitFailed': 'Failed to split',

  // Postmortem
  'admin.incident.postmortemPublic': 'Public',
  'admin.incident.postmortemInternal': 'Internal only',
  'admin.incident.postmortemUrlLabel': 'Postmortem document',
  'admin.incident.postmortemUrl': 'Postmortem document URL',
  'admin.incident.postmortemPublicToggle': 'Publish postmortem publicly',
  'admin.incident.postmortemPublicHint': 'When off, the content above is hidden from the public page (off by default)',
  'admin.incident.rootCausePlaceholder': 'e.g. The database connection pool was exhausted by slow queries',
  'admin.incident.resolutionPlaceholder': 'e.g. Scaled up the connection pool and added indexes for slow queries',

  // Toasts / errors
  'admin.incident.loadFailed': 'Failed to load',
  'admin.incident.loadDetailFailed': 'Failed to load incident details',
  'admin.incident.actionFailed': 'Operation failed',
  'admin.incident.updateSuccess': 'Updated successfully',
  'admin.incident.createSuccess': 'Created successfully',
  'admin.incident.saveFailed': 'Failed to save',
  'admin.incident.updateFailed': 'Failed to update',
  'admin.incident.deleteSuccess': 'Deleted successfully',
  'admin.incident.deleteFailed': 'Failed to delete',
  'admin.incident.confirmDelete': 'Delete this item?',
  'admin.incident.confirmDeleteIncident': 'Delete this incident?',
  'admin.incident.confirmDeleteUpdate': 'Delete this update entry?',
  'admin.incident.confirmDiscardChanges': 'You have unsaved changes. Close anyway?',

  // ---- Maintenance schedule ----
  'admin.maintenance.listTitle': 'Maintenance schedule',
  'admin.maintenance.create': 'Create maintenance',
  'admin.maintenance.editMaintenance': 'Edit maintenance',
  'admin.maintenance.colTitle': 'Title',
  'admin.maintenance.colStatus': 'Status',
  'admin.maintenance.colStart': 'Start time',
  'admin.maintenance.colEnd': 'End time',
  'admin.maintenance.colAffectedServices': 'Affected services',
  'admin.maintenance.colActions': 'Actions',
  'admin.maintenance.edit': 'Edit',
  'admin.maintenance.delete': 'Delete',
  'admin.maintenance.cancel': 'Cancel',
  'admin.maintenance.save': 'Save',

  // Form fields
  'admin.maintenance.labelTitle': 'Title *',
  'admin.maintenance.labelDescription': 'Description',
  'admin.maintenance.labelStart': 'Start time *',
  'admin.maintenance.labelEnd': 'End time *',
  'admin.maintenance.startPlaceholder': 'Select start time',
  'admin.maintenance.endPlaceholder': 'Select end time',
  'admin.maintenance.labelStatus': 'Status',
  'admin.maintenance.labelAffectedServices': 'Affected services',
  'admin.maintenance.searchServicePlaceholder': 'Search services...',
  'admin.maintenance.noMatchingService': 'No matching services',

  // Maintenance status
  'admin.maintenance.status.scheduled': 'Scheduled',
  'admin.maintenance.status.inProgress': 'In progress',
  'admin.maintenance.status.completed': 'Completed',
  'admin.maintenance.status.cancelled': 'Cancelled',

  // Recurrence
  'admin.maintenance.labelRecurrence': 'Repeat',
  'admin.maintenance.recurrence.none': 'No repeat (one-off)',
  'admin.maintenance.recurrence.daily': 'Daily',
  'admin.maintenance.recurrence.weekly': 'Weekly',
  'admin.maintenance.recurrence.monthly': 'Monthly',
  'admin.maintenance.recurrence.everyNDays': 'Every {n} days',
  'admin.maintenance.recurrence.weeklyOn': 'Weekly on {day}',
  'admin.maintenance.recurrence.everyNWeeksOn': 'Every {n} weeks on {day}',
  'admin.maintenance.recurrence.everyNMonths': 'Every {n} months',
  'admin.maintenance.recurrence.until': 'Until {date}',
  'admin.maintenance.labelRecurrenceUntil': 'Repeat until',
  'admin.maintenance.recurrenceUntilPlaceholder': 'Leave empty to repeat indefinitely',
  'admin.maintenance.labelMonthday': 'Day of the month',
  'admin.maintenance.monthdayPlaceholder': 'Leave empty to use the start date',
  'admin.maintenance.intervalDays': 'Interval (days)',
  'admin.maintenance.intervalWeeks': 'Interval (weeks)',
  'admin.maintenance.intervalMonths': 'Interval (months)',
  'admin.maintenance.hintWeekly': 'The repeat day follows the weekday of the start time (e.g. a start time on Sunday 02:00 repeats every Sunday at 02:00).',
  'admin.maintenance.hintAdvance': 'After a window ends, the server advances the start/end times to the next window and resets the status to “Scheduled”.',
  'admin.maintenance.hintUntilEmpty': 'An empty repeat-until means it keeps repeating.',

  // Weekdays (1 = Monday … 7 = Sunday)
  'admin.maintenance.weekday.mon': 'Mon',
  'admin.maintenance.weekday.tue': 'Tue',
  'admin.maintenance.weekday.wed': 'Wed',
  'admin.maintenance.weekday.thu': 'Thu',
  'admin.maintenance.weekday.fri': 'Fri',
  'admin.maintenance.weekday.sat': 'Sat',
  'admin.maintenance.weekday.sun': 'Sun',

  // Toasts / errors
  'admin.maintenance.loadFailed': 'Failed to load',
  'admin.maintenance.updateSuccess': 'Updated successfully',
  'admin.maintenance.createSuccess': 'Created successfully',
  'admin.maintenance.saveFailed': 'Failed to save',
  'admin.maintenance.confirmDelete': 'Delete this item?',
  'admin.maintenance.deleteSuccess': 'Deleted successfully',
  'admin.maintenance.deleteFailed': 'Failed to delete',

  // ---- Date picker ----
  'admin.datepicker.placeholder': 'Select',
  'admin.datepicker.prevYear': 'Previous year',
  'admin.datepicker.prevMonth': 'Previous month',
  'admin.datepicker.nextMonth': 'Next month',
  'admin.datepicker.nextYear': 'Next year',
  'admin.datepicker.yearOnly': '{year}',
  'admin.datepicker.yearMonth': '{month} {year}',
  'admin.datepicker.now': 'Now',
  'admin.datepicker.today': 'Today',
  'admin.datepicker.thisMonth': 'This month',
  'admin.datepicker.clear': 'Clear',
  'admin.datepicker.weekday.mon': 'Mon',
  'admin.datepicker.weekday.tue': 'Tue',
  'admin.datepicker.weekday.wed': 'Wed',
  'admin.datepicker.weekday.thu': 'Thu',
  'admin.datepicker.weekday.fri': 'Fri',
  'admin.datepicker.weekday.sat': 'Sat',
  'admin.datepicker.weekday.sun': 'Sun',
  'admin.datepicker.monthJan': 'Jan',
  'admin.datepicker.monthFeb': 'Feb',
  'admin.datepicker.monthMar': 'Mar',
  'admin.datepicker.monthApr': 'Apr',
  'admin.datepicker.monthMay': 'May',
  'admin.datepicker.monthJun': 'Jun',
  'admin.datepicker.monthJul': 'Jul',
  'admin.datepicker.monthAug': 'Aug',
  'admin.datepicker.monthSep': 'Sep',
  'admin.datepicker.monthOct': 'Oct',
  'admin.datepicker.monthNov': 'Nov',
  'admin.datepicker.monthDec': 'Dec',
}

registerMessages('zh-CN', adminIncidentZh)
registerMessages('en-US', adminIncidentEn)
