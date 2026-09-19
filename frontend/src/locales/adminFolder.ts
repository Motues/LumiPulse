import { registerMessages } from '../composables/useI18n'

/**
 * 服务分组（服务聚合文件夹）管理页文案。
 * 命名空间：admin.folder。通用按钮文案复用 admin.common.*（由其它批次提供），
 * 这里只补上分组页独有的那一个 key。
 */
export const adminFolderZh: Record<string, string> = {
  'admin.common.edit': '编辑',

  'admin.folder.title': '服务分组',
  'admin.folder.subtitle':
    '把多个服务放进同一个分组，公开首页会把它们融合成一个条目展示；点击箭头展开可查看各服务明细。',
  'admin.folder.create': '创建分组',
  'admin.folder.createTitle': '创建分组',
  'admin.folder.editTitle': '编辑分组',
  'admin.folder.nameLabel': '分组名称 *',
  'admin.folder.descriptionLabel': '描述',
  'admin.folder.showOnHomepage': '在首页展示',
  'admin.folder.showOnHomepageHint': '关闭后该分组及其下所有服务都不会出现在公开首页',
  'admin.folder.hidden': '首页隐藏',
  'admin.folder.serviceCount': '{n} 个服务',
  'admin.folder.expand': '展开服务明细',
  'admin.folder.collapse': '收起服务明细',
  'admin.folder.emptyFolder': '该分组下还没有服务，可在下方「未分组服务」里把服务移入。',
  'admin.folder.empty': '暂无服务分组',
  'admin.folder.emptyHint': '创建分组后，把多个服务聚合到一起展示。',
  'admin.folder.ungrouped': '未分组服务（{n}）',
  'admin.folder.ungroupedHint': '这些服务各自独立展示在首页，可以把它们移入某个分组。',
  'admin.folder.chooseFolder': '移入分组...',
  'admin.folder.serviceName': '服务',
  'admin.folder.serviceStatus': '状态',
  'admin.folder.serviceUptime': '在线率',
  'admin.folder.serviceLatency': '响应时间',
  'admin.folder.serviceActions': '操作',
  'admin.folder.removeFromFolder': '移出分组',
  'admin.folder.serviceMoved': '已移入分组',
  'admin.folder.serviceRemoved': '已移出分组',
  'admin.folder.moveFailed': '移动失败',
  'admin.folder.nameRequired': '请填写分组名称',
  'admin.folder.created': '创建成功',
  'admin.folder.updated': '更新成功',
  'admin.folder.deleted': '删除成功',
  'admin.folder.loadFailed': '加载失败',
  'admin.folder.saveFailed': '保存失败',
  'admin.folder.deleteFailed': '删除失败',
  'admin.folder.deleteConfirm': '确定要删除分组「{name}」吗？分组内的服务不会被删除，只会变回未分组。',
}

export const adminFolderEn: Record<string, string> = {
  'admin.common.edit': 'Edit',

  'admin.folder.title': 'Service groups',
  'admin.folder.subtitle':
    'Put several services in one group and the public homepage shows them merged into a single entry; expand a group to see each service separately.',
  'admin.folder.create': 'Create group',
  'admin.folder.createTitle': 'Create group',
  'admin.folder.editTitle': 'Edit group',
  'admin.folder.nameLabel': 'Group name *',
  'admin.folder.descriptionLabel': 'Description',
  'admin.folder.showOnHomepage': 'Show on homepage',
  'admin.folder.showOnHomepageHint': 'When off, this group and all its services are hidden from the public homepage',
  'admin.folder.hidden': 'Hidden',
  'admin.folder.serviceCount': '{n} services',
  'admin.folder.expand': 'Expand services',
  'admin.folder.collapse': 'Collapse services',
  'admin.folder.emptyFolder': 'No services in this group yet — use “Ungrouped services” below to move some in.',
  'admin.folder.empty': 'No service groups yet',
  'admin.folder.emptyHint': 'Create a group to display several services as one entry.',
  'admin.folder.ungrouped': 'Ungrouped services ({n})',
  'admin.folder.ungroupedHint': 'These services are shown individually on the homepage; you can move them into a group.',
  'admin.folder.chooseFolder': 'Move to group...',
  'admin.folder.serviceName': 'Service',
  'admin.folder.serviceStatus': 'Status',
  'admin.folder.serviceUptime': 'Uptime',
  'admin.folder.serviceLatency': 'Response time',
  'admin.folder.serviceActions': 'Actions',
  'admin.folder.removeFromFolder': 'Remove from group',
  'admin.folder.serviceMoved': 'Service moved into the group',
  'admin.folder.serviceRemoved': 'Service removed from the group',
  'admin.folder.moveFailed': 'Failed to move the service',
  'admin.folder.nameRequired': 'Please enter a group name',
  'admin.folder.created': 'Group created',
  'admin.folder.updated': 'Group updated',
  'admin.folder.deleted': 'Group deleted',
  'admin.folder.loadFailed': 'Failed to load',
  'admin.folder.saveFailed': 'Failed to save',
  'admin.folder.deleteFailed': 'Failed to delete',
  'admin.folder.deleteConfirm': 'Delete the group “{name}”? Its services are not deleted, they just become ungrouped.',
}

registerMessages('zh-CN', adminFolderZh)
registerMessages('en-US', adminFolderEn)
