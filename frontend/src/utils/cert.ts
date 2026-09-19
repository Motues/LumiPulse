/**
 * HTTPS 证书到期信息的展示辅助函数。
 *
 * 后端在探测成功后把证书的 NotAfter 写回 `certExpiresAt`（RFC3339，UTC）；
 * 非 HTTPS 服务或尚未探测成功时为空字符串。
 * 这里只做数值与等级计算，具体文案交给调用方的 i18n / 本地格式化。
 */

/** 证书剩余天数（含小数）。无证书信息或无法解析时返回 null */
export function certRemainingDays(iso: string | undefined | null): number | null {
  if (!iso) return null
  const expires = new Date(iso)
  if (Number.isNaN(expires.getTime())) return null
  return (expires.getTime() - Date.now()) / 86400000
}

/** 证书到期展示等级：< 7 天（含已过期）为 critical，< 30 天为 warn，其余为 ok */
export type CertLevel = 'ok' | 'warn' | 'critical'

export function certLevel(iso: string | undefined | null): CertLevel | null {
  const days = certRemainingDays(iso)
  if (days === null) return null
  if (days < 7) return 'critical'
  if (days < 30) return 'warn'
  return 'ok'
}
