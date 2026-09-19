import { registerMessages } from '../composables/useI18n'

/**
 * 前后端交互与通用交互文案（不专属某个页面）：
 * - client.ts 的请求失败提示
 * - useUnsavedChanges 的未保存确认
 * 命名空间：common.*
 */
export const commonAppZh: Record<string, string> = {
  'common.emptyResponse': '服务返回了空响应',
  'common.unexpectedResponse': '服务返回了非预期内容',
  'common.requestFailed': '请求失败',
  'common.requestFailedWithStatus': '请求失败 ({status})',
  'common.confirmUnsavedChanges': '有未保存的更改，确定要关闭吗？',
}

export const commonAppEn: Record<string, string> = {
  'common.emptyResponse': 'The server returned an empty response',
  'common.unexpectedResponse': 'The server returned an unexpected response',
  'common.requestFailed': 'Request failed',
  'common.requestFailedWithStatus': 'Request failed ({status})',
  'common.confirmUnsavedChanges': 'You have unsaved changes. Close anyway?',
}

registerMessages('zh-CN', commonAppZh)
registerMessages('en-US', commonAppEn)
