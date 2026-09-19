export interface MenuItem {
  label: string
  icon: string
  route: string
}

export interface UserInfo {
  name: string
  avatar?: string
  role: string
}

/** 二级导航的单个条目 */
export interface SectionItem {
  id: string
  label: string
  /** SVG path 的 d 属性 */
  icon: string
}

/** 二级导航中的一组条目 */
export interface SectionGroup {
  title: string
  items: SectionItem[]
}
