import { registerMessages } from '../composables/useI18n'

/**
 * 管理后台通用外壳与设置面板文案。
 * 命名空间：admin.settings.* / admin.topbar.* / admin.sidebar.*
 * （admin.sectionNav.* 与 admin.layout.* 暂无文案：SectionNav 由调用方传入文案，
 *   AdminLayout 仅做布局且当前未被引用。）
 */
export const adminSettingsZh: Record<string, string> = {
  // 系统设置面板
  'admin.settings.title': '系统设置',
  'admin.settings.siteName': '系统名称',
  'admin.settings.siteNameHint': '显示在浏览器标签栏和页面各处',
  'admin.settings.siteIcon': '系统图标',
  'admin.settings.uploadIcon': '上传图标',
  'admin.settings.resetDefault': '恢复默认',
  'admin.settings.iconFormatsHint': '支持 SVG、PNG、JPG 格式',
  'admin.settings.showAdminEntry': '显示管理后台入口',
  'admin.settings.showAdminEntryHint': '在公开页面页脚显示"管理后台"链接',
  'admin.settings.customFooter': '自定义页脚内容',
  'admin.settings.customFooterHint': '设置后替代默认页脚，支持 HTML 内容',
  'admin.settings.customFooterPlaceholder': '<a href="https://example.com">我的站点</a>',
  'admin.settings.saving': '保存中...',
  'admin.settings.save': '保存设置',
  'admin.settings.loadFailed': '加载失败',
  'admin.settings.saveSuccess': '设置已保存',
  'admin.settings.saveFailed': '保存失败',
  'admin.settings.dataTransfer': '数据导入导出',
  'admin.settings.dataTransferHint': '导出或导入服务、事件、维护计划和系统设置（不含日志和探测记录）。',
  'admin.settings.exportData': '导出数据',
  'admin.settings.importData': '导入数据',
  'admin.settings.exportSuccess': '导出成功',
  'admin.settings.exportFailed': '导出失败',
  'admin.settings.importConfirm': '导入将覆盖现有所有服务、事件、维护计划和系统设置，此操作不可撤销。\n\n确定要继续吗？',
  'admin.settings.importCancelled': '已取消导入',
  'admin.settings.importSuccess': '导入成功',
  'admin.settings.importSuccessRefresh': '导入成功，请刷新页面查看',
  'admin.settings.importFailed': '导入失败，请检查文件格式',

  // 顶栏
  'admin.topbar.role': '管理员',
  'admin.topbar.users': '用户管理',
  'admin.topbar.logout': '退出登录',
  'admin.topbar.langZh': '中文',
  'admin.topbar.langEn': 'English',

  // 侧边栏
  'admin.sidebar.versionLabel': '版本 v{version}',
}

export const adminSettingsEn: Record<string, string> = {
  // Settings panel
  'admin.settings.title': 'System Settings',
  'admin.settings.siteName': 'Site name',
  'admin.settings.siteNameHint': 'Shown in the browser tab and across the site',
  'admin.settings.siteIcon': 'Site icon',
  'admin.settings.uploadIcon': 'Upload icon',
  'admin.settings.resetDefault': 'Reset to default',
  'admin.settings.iconFormatsHint': 'Supports SVG, PNG and JPG',
  'admin.settings.showAdminEntry': 'Show admin panel entry',
  'admin.settings.showAdminEntryHint': 'Show an "Admin" link in the public page footer',
  'admin.settings.customFooter': 'Custom footer content',
  'admin.settings.customFooterHint': 'Replaces the default footer when set; HTML is supported',
  'admin.settings.customFooterPlaceholder': '<a href="https://example.com">My site</a>',
  'admin.settings.saving': 'Saving...',
  'admin.settings.save': 'Save settings',
  'admin.settings.loadFailed': 'Failed to load',
  'admin.settings.saveSuccess': 'Settings saved',
  'admin.settings.saveFailed': 'Failed to save',
  'admin.settings.dataTransfer': 'Data import / export',
  'admin.settings.dataTransferHint': 'Export or import services, incidents, maintenance windows and system settings (logs and probe records are excluded).',
  'admin.settings.exportData': 'Export data',
  'admin.settings.importData': 'Import data',
  'admin.settings.exportSuccess': 'Exported successfully',
  'admin.settings.exportFailed': 'Export failed',
  'admin.settings.importConfirm': 'Importing will overwrite all existing services, incidents, maintenance windows and system settings. This action cannot be undone.\n\nContinue?',
  'admin.settings.importCancelled': 'Import cancelled',
  'admin.settings.importSuccess': 'Imported successfully',
  'admin.settings.importSuccessRefresh': 'Imported successfully. Refresh the page to see the changes',
  'admin.settings.importFailed': 'Import failed. Please check the file format',

  // Top bar
  'admin.topbar.role': 'Administrator',
  'admin.topbar.users': 'User management',
  'admin.topbar.logout': 'Sign out',
  'admin.topbar.langZh': 'Chinese',
  'admin.topbar.langEn': 'English',

  // Sidebar
  'admin.sidebar.versionLabel': 'Version v{version}',
}

registerMessages('zh-CN', adminSettingsZh)
registerMessages('en-US', adminSettingsEn)
