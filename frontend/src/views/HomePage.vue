<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import type { SummaryResponse, Incident, ServiceSummary, FolderSummary } from '../api/types'
import ServiceMatrix from '../components/ServiceMatrix.vue'
import ServiceStatusBadge from '../components/ServiceStatusBadge.vue'
import ServiceUrlBadge from '../components/ServiceUrlBadge.vue'
import PublicServiceDetail from '../components/PublicServiceDetail.vue'
import PublicHeader from '../components/PublicHeader.vue'
import PublicFooter from '../components/PublicFooter.vue'
import PublicSubscribeModal from '../components/PublicSubscribeModal.vue'
import { useI18n } from '../composables/useI18n'

const route = useRoute()
const router = useRouter()
const { t, formatDate, formatDateTime } = useI18n()
const summary = ref<SummaryResponse | null>(null)
const incidents = ref<Incident[]>([])
const loading = ref(true)
const error = ref('')
const showSubscribeModal = ref(false)

function incidentStatusLabel(status: string): string {
  const key = `incident.status.${status}`
  const label = t(key)
  return label === key ? status : label
}

function isServiceInMaintenance(serviceId: number): boolean {
  if (!summary.value?.maintenances) return false
  return summary.value.maintenances.some(m =>
    m.status === 'in_progress' &&
    m.affectedServices &&
    m.affectedServices.split(',').map(Number).includes(serviceId)
  )
}

const dailyStats = ref<Map<number, [number, number, number, number][]>>(new Map())
const isMobile = ref(false)

// 当前路由中的服务 hash（/services/:hash），用于支持可分享的详情链接
const selectedHash = computed(() =>
  route.name === 'service-detail' ? String(route.params.hash || '') : ''
)
// 直接从已加载的总览里派生选中的服务：自动跟随 30s 自动刷新，且不需要额外请求
const selectedService = computed<ServiceSummary | null>(() => {
  if (!selectedHash.value || !summary.value) return null
  return summary.value.services.find(s => s.publicHash === selectedHash.value) || null
})
// 已经加载完成但 hash 对应不上任何可见服务
const serviceNotFound = computed(() =>
  !!selectedHash.value && !loading.value && !selectedService.value
)
// 是否处于详情视图（/services/:hash）。详情页保持聚焦，
// 「维护计划」「过去事件」只在首页列表展示，不在详情页出现。
const isDetailView = computed(() => !!selectedHash.value)

function openService(svc: ServiceSummary) {
  if (!svc.publicHash) return
  router.push(`/services/${svc.publicHash}`)
}

function closeService() {
  router.push('/')
}

function getServiceDays(serviceId: number): [number, number, number, number][] {
  return dailyStats.value.get(serviceId) || []
}

/**
 * 服务分组的融合矩阵：把分组内各服务的每日
 * [up, down, statusCode, maintenanceCount] 逐日相加。
 * 融合口径与后端一致（按探测次数加权）；事件状态取分组内任一服务的状态码，
 * 因为事件本身就是挂在服务上的，这里只用于格子提示。
 * 维护期失败次数同样相加：只要分组当天有窗口外的真实故障就按真实故障显示，
 * 否则才用蓝色表达「当天只有计划内停机」。
 */
function getFolderDays(folder: FolderSummary): [number, number, number, number][] {
  const days = (dailyStats.value.get(folder.services[0]?.id ?? -1) || []).length
  if (days === 0) return []
  const merged: [number, number, number, number][] = []
  for (let i = 0; i < days; i++) {
    let up = 0
    let down = 0
    let status = -1
    let maintenance = 0
    for (const svc of folder.services) {
      const pair = dailyStats.value.get(svc.id)?.[i]
      if (!pair) continue
      if (pair[0] >= 0) up += pair[0]
      if (pair[1] >= 0) down += pair[1]
      if (pair[3] > 0) maintenance += pair[3]
      if (pair[2] > status) status = pair[2]
    }
    // 分组整体没有任何探测数据时保持「无数据」
    merged.push(up + down === 0 && maintenance === 0 ? [-1, -1, status, 0] : [up, down, status, maintenance])
  }
  return merged
}

// --- 服务分组（服务聚合文件夹）---
/** 展开的分组 ID：展开后隐藏融合信息，改为展示各服务明细 */
const expandedFolders = ref<number[]>([])

function isFolderExpanded(id: number): boolean {
  return expandedFolders.value.includes(id)
}

function toggleFolder(id: number) {
  const idx = expandedFolders.value.indexOf(id)
  if (idx >= 0) expandedFolders.value.splice(idx, 1)
  else expandedFolders.value.push(id)
}

/** 分组内所有服务都在维护中时，分组整体显示为「维护中」 */
function isFolderInMaintenance(folder: FolderSummary): boolean {
  return folder.services.length > 0 && folder.services.every(s => isServiceInMaintenance(s.id))
}

/**
 * 首页条目顺序：按服务顺序输出，遇到分组时以「分组融合条目」替代其成员，
 * 未分组的服务保持原有位置。这样分组不会打乱服务的整体排序。
 */
type HomeEntry =
  | { kind: 'folder'; key: string; folder: FolderSummary }
  | { kind: 'service'; key: string; service: ServiceSummary }

const homeEntries = computed<HomeEntry[]>(() => {
  const services = summary.value?.services || []
  const folders = summary.value?.folders || []
  const folderById = new Map(folders.map(f => [f.id, f]))
  const rendered = new Set<number>()
  const entries: HomeEntry[] = []

  for (const svc of services) {
    const folder = svc.folderId ? folderById.get(svc.folderId) : undefined
    if (folder) {
      if (rendered.has(folder.id)) continue
      rendered.add(folder.id)
      entries.push({ kind: 'folder', key: `folder-${folder.id}`, folder })
      continue
    }
    entries.push({ kind: 'service', key: `service-${svc.id}`, service: svc })
  }
  return entries
})

const incidentsByDate = computed(() => {
  const map = new Map<string, Incident[]>()
  const dayCount = isMobile.value ? 30 : 90
  const now = new Date()
  const cst = new Date(now.getTime() + (now.getTimezoneOffset() + 480) * 60000)
  for (let i = dayCount - 1; i >= 0; i--) {
    const d = new Date(cst)
    d.setUTCDate(d.getUTCDate() - i)
    const key = `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}-${String(d.getUTCDate()).padStart(2, '0')}`
    map.set(key, [])
  }
  for (const inc of incidents.value) {
    const incDate = inc.createdAt.slice(0, 10)
    for (const [dateStr, list] of map) {
      if (incDate > dateStr) continue
      if (inc.status === 'resolved') {
        const resolvedDate = inc.resolvedAt ? inc.resolvedAt.slice(0, 10) : inc.updatedAt.slice(0, 10)
        if (resolvedDate < dateStr) continue
      }
      list.push(inc)
    }
  }
  return map
})

async function loadDailyStats() {
  if (!summary.value) return
  const days = isMobile.value ? 30 : 90
  try {
    // 一次批量请求取回所有服务的矩阵数据（原先按服务逐个 await，N 个服务 = N 次串行往返）
    const res = await api.getBatchDailyStats(days)
    const map = new Map<number, [number, number, number, number][]>()
    for (const item of res.data.services) {
      map.set(item.serviceId, item.days)
    }
    for (const svc of summary.value.services) {
      if (!map.has(svc.id)) {
        map.set(svc.id, Array.from({ length: days }, () => [-1, -1, -1, 0] as [number, number, number, number]))
      }
    }
    dailyStats.value = map
  } catch {
    const map = new Map<number, [number, number, number, number][]>()
    for (const svc of summary.value.services) {
      map.set(svc.id, Array.from({ length: days }, () => [-1, -1, -1, 0] as [number, number, number, number]))
    }
    dailyStats.value = map
  }
}

let refreshTimer: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  isMobile.value = window.innerWidth < 768
  try {
    const [sumRes, incRes] = await Promise.all([
      api.getSummary(),
      api.getPublicIncidents(1, 10),
    ])
    summary.value = sumRes.data
    incidents.value = incRes.data.incidents
    await loadDailyStats()
  } catch (e: any) {
    error.value = e.message || t('common.loadFailed')
  } finally {
    loading.value = false
  }

  // Auto-refresh every 30s for real-time updates
  refreshTimer = setInterval(async () => {
    try {
      const [sumRes, incRes] = await Promise.all([
        api.getSummary(),
        api.getPublicIncidents(1, 10),
      ])
      summary.value = sumRes.data
      incidents.value = incRes.data.incidents
    } catch {
      error.value = t('common.dataRefreshFailed')
    }
  }, 30000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <PublicHeader :on-subscribe="() => (showSubscribeModal = true)" />

  <main class="max-w-[1000px] mx-auto px-6 py-8">
    <div v-if="loading" class="text-center py-20" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>
    <div v-else-if="error" class="text-center py-20 text-red-500">{{ error }}</div>

    <template v-else-if="summary">
      <!-- Hero Banner — 只在首页展示，服务详情页不出现 -->
      <section v-if="!isDetailView" :class="[
        'rounded-lg p-8 mb-8 flex items-center justify-between relative overflow-hidden',
        summary.overallStatus === 'operational'
          ? 'bg-[#f0fdf4] dark:bg-[#0a2e1a]'
          : 'bg-[#fef2f2] dark:bg-[#3a1111]'
      ]">
        <div class="relative z-10">
            <h1 :class="['text-3xl font-bold mb-1', summary.overallStatus === 'operational' ? 'text-[#2d7a47] dark:text-[#4ade80]' : 'text-[#9e1f1e] dark:text-[#f87171]']">
              {{ summary.overallStatus === 'operational' ? t('home.allOperational') : t('home.outage') }}
            </h1>
        </div>
      </section>

      <!-- Maintenance Plans (if any) — 仅首页展示，详情页不出现 -->
      <section v-if="!isDetailView && summary.maintenances && summary.maintenances.length > 0" class="rounded-lg p-6 mb-8" style="border: 1px solid var(--button-border-color);">
        <div>
          <h3 class="text-base font-bold mb-2" style="color: var(--text-color);">{{ t('home.maintenances') }}</h3>
          <div v-for="m in summary.maintenances" :key="m.id" class="mb-3 last:mb-0 pb-3 last:pb-0">
            <p class="text-sm font-medium" style="color: var(--text-color);">{{ m.title }}</p>
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.5;">
              {{ formatDateTime(m.scheduledStart) }} {{ t('common.cst') }} - {{ formatDateTime(m.scheduledEnd) }} {{ t('common.cst') }}
            </p>
            <p v-if="m.description" class="text-xs mt-1" style="color: var(--text-color); opacity: 0.5;">{{ m.description }}</p>
          </div>
        </div>
      </section>

      <!-- Service Detail (when a service is selected) -->
      <PublicServiceDetail
        v-if="selectedService"
        :service="selectedService"
        :daily-days="getServiceDays(selectedService.id)"
        @back="closeService"
      />

      <!-- 分享链接失效：hash 对应不上任何首页可见的服务 -->
      <section v-else-if="serviceNotFound" class="rounded-lg p-8 mb-8 text-center" style="border: 1px solid var(--button-border-color);">
        <p class="text-sm mb-3" style="color: var(--text-color); opacity: 0.6;">{{ t('common.notFoundService') }}</p>
        <button @click="closeService" class="text-sm font-medium text-emerald-600 dark:text-emerald-400 hover:opacity-80 transition-opacity">{{ t('common.backToStatus') }}</button>
      </section>

      <!-- Service Status -->
      <section v-else class="rounded-lg mb-8 pb-4" style="border: 1px solid var(--button-border-color);">
        <div class="flex items-center justify-between p-6 pb-4 mb-4">
          <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ t('home.systemStatus') }}</h2>
        </div>

        <template v-for="(entry, idx) in homeEntries" :key="entry.key">
          <!-- 服务分组：默认展示融合信息，展开后展示各服务明细（融合信息消失） -->
          <div
            v-if="entry.kind === 'folder'"
            class="px-6 pb-6 rounded-lg transition-colors"
            :class="{ 'pt-4': idx > 0 }"
          >
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center gap-1.5 min-w-0">
                <button
                  class="p-0.5 flex-shrink-0 transition-transform duration-300 text-[color:var(--text-color)] opacity-50 hover:opacity-100"
                  :class="isFolderExpanded(entry.folder.id) ? 'rotate-90' : ''"
                  :title="isFolderExpanded(entry.folder.id) ? t('home.folderCollapse') : t('home.folderExpand')"
                  @click="toggleFolder(entry.folder.id)"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
                  </svg>
                </button>
                <svg class="w-5 h-5 flex-shrink-0 text-[color:var(--text-color)] opacity-45" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
                </svg>
                <span class="font-bold leading-none truncate" style="color: var(--text-color);">{{ entry.folder.name }}</span>
                <span class="text-xs flex-shrink-0" style="color: var(--text-color); opacity: 0.4;">
                  {{ t('home.folderServiceCount', { n: entry.folder.services.length }) }}
                </span>
              </div>
              <!-- 融合信息：展开后随收起动画消失 -->
              <ServiceStatusBadge
                :status="entry.folder.status"
                :maintenance="isFolderInMaintenance(entry.folder)"
                class="flex-shrink-0 transition-opacity duration-200"
                :class="{ 'opacity-0': isFolderExpanded(entry.folder.id) }"
              />
            </div>

            <!-- 融合矩阵：展开时平滑收起 -->
            <div class="collapse-panel" :class="{ 'is-collapsed': isFolderExpanded(entry.folder.id) }">
              <div class="min-h-0 overflow-hidden">
                <ServiceMatrix
                  :days="getFolderDays(entry.folder)"
                  :uptime="entry.folder.uptime"
                />
              </div>
            </div>

            <!-- 展开：各服务明细（融合信息消失），收起时平滑折叠 -->
            <div class="collapse-panel" :class="{ 'is-collapsed': !isFolderExpanded(entry.folder.id) }">
              <div class="min-h-0 overflow-hidden">
                <div class="space-y-4 pt-1">
                  <div v-for="svc in entry.folder.services" :key="svc.id">
                    <div class="flex items-center justify-between mb-2">
                      <div class="flex items-center min-w-0">
                        <span
                          class="font-medium leading-none cursor-pointer truncate text-[color:var(--text-color)] hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors"
                          :title="t('home.viewServiceDetail')"
                          @click="openService(svc)"
                        >{{ svc.name }}</span>
                        <ServiceUrlBadge :url="svc.url" />
                      </div>
                      <ServiceStatusBadge :status="svc.status" :maintenance="isServiceInMaintenance(svc.id)" class="flex-shrink-0" />
                    </div>
                    <ServiceMatrix :days="getServiceDays(svc.id)" :uptime="svc.uptime" :service-id="svc.id" :incidents-by-date="incidentsByDate" />
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 未分组的服务 -->
          <div
            v-else
            class="px-6 pb-6 rounded-lg transition-colors"
            :class="{ 'pt-4': idx > 0 }"
          >
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center min-w-0">
                <span
                  class="font-bold leading-none cursor-pointer truncate text-[color:var(--text-color)] hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors"
                  :title="t('home.viewServiceDetail')"
                  @click="openService(entry.service)"
                >{{ entry.service.name }}</span>
                <ServiceUrlBadge :url="entry.service.url" />
              </div>
              <ServiceStatusBadge :status="entry.service.status" :maintenance="isServiceInMaintenance(entry.service.id)" class="flex-shrink-0" />
            </div>
            <ServiceMatrix :days="getServiceDays(entry.service.id)" :uptime="entry.service.uptime" :service-id="entry.service.id" :incidents-by-date="incidentsByDate" />
          </div>
        </template>
      </section>

      <!-- Past Incidents — 仅首页展示，详情页不出现 -->
      <section v-if="!isDetailView && incidents.length > 0" class="mb-8">
        <h2 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('home.pastIncidents') }}</h2>
        <div v-for="inc in incidents" :key="inc.publicHash" class="rounded-lg p-6 mb-4 cursor-pointer border border-[color:var(--button-border-color)] hover:border-emerald-500/30 dark:hover:border-emerald-400/30 transition-colors" @click="router.push(`/incidents/${inc.publicHash}`)">
          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
            <div class="flex items-center gap-2">
              <h3 class="text-base font-bold" :class="{
                'text-red-500 dark:text-red-400': inc.impact === 'critical',
                'text-orange-500 dark:text-orange-400': inc.impact === 'major',
                'text-yellow-500 dark:text-yellow-400': inc.impact === 'minor',
              }">{{ inc.title }}</h3>
              <span v-if="inc.updates && inc.updates.length > 0" class="text-xs font-medium px-2 py-0.5 rounded" :class="{
                'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400': inc.updates[inc.updates.length - 1].status === 'investigating' || inc.updates[inc.updates.length - 1].status === 'identified',
                'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400': inc.updates[inc.updates.length - 1].status === 'monitoring',
                'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400': inc.updates[inc.updates.length - 1].status === 'resolved',
              }">{{ incidentStatusLabel(inc.updates[inc.updates.length - 1].status) }}</span>
            </div>
            <div class="text-sm font-medium" style="color: var(--text-color); opacity: 0.5;">{{ formatDate(inc.createdAt) }}</div>
          </div>
          <p v-if="inc.updates && inc.updates.length > 0" class="text-sm mt-2" style="color: var(--text-color); opacity: 0.5;">
            {{ inc.updates[inc.updates.length - 1].content }}
          </p>
        </div>
      </section>

      <!-- No Maintenance — 仅首页展示，详情页不出现 -->
      <section v-if="!isDetailView && (!summary.maintenances || summary.maintenances.length === 0)" class="rounded-lg p-6 mb-8" style="border: 1px solid var(--button-border-color);">
        <div>
          <h3 class="text-base font-bold mb-2" style="color: var(--text-color);">{{ t('home.maintenances') }}</h3>
          <p class="text-sm mb-1" style="color: var(--text-color);">{{ t('home.noMaintenance') }}</p>
          <p class="text-sm" style="color: var(--text-color); opacity: 0.5;">{{ t('home.noMaintenanceHint') }}</p>
        </div>
      </section>
    </template>
  </main>

  <PublicFooter />

  <PublicSubscribeModal
    v-if="showSubscribeModal"
    :services="summary?.services || []"
    @close="showSubscribeModal = false"
  />
</template>

<style scoped>
.header-btn {
  color: var(--text-color);
  border: 1px solid var(--button-border-color);
  background-color: var(--bg-color);
  transition: background-color 0.2s;
}

/**
 * 服务分组展开 / 收起的平滑动画。
 * 用 grid-template-rows 在 1fr 与 0fr 之间过渡，高度会跟着内容自适应，
 * 比 max-height 更贴合实际高度（不需要猜一个够大的上限）。
 * 直接子元素必须 min-height:0 + overflow:hidden，否则 0fr 压不下去。
 * 提示框等浮层因此统一用 fixed 定位（见 ServiceUrlBadge / ServiceMatrix）。
 */
.collapse-panel {
  display: grid;
  grid-template-rows: 1fr;
  opacity: 1;
  transition: grid-template-rows 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.25s ease;
}
.collapse-panel.is-collapsed {
  grid-template-rows: 0fr;
  opacity: 0;
}
.collapse-panel > * {
  min-height: 0;
  overflow: hidden;
}
@media (prefers-reduced-motion: reduce) {
  .collapse-panel {
    transition: none;
  }
}
.header-btn:hover {
  background-color: var(--button-hover-color);
}
.footer-link {
  color: var(--text-color);
  opacity: 0.5;
}
.footer-link:hover {
  opacity: 1;
}
.timeline-dot {
  position: absolute;
  left: -5px;
  top: 50%;
  transform: translateY(-50%);
  width: 12px;
  height: 12px;
  border-radius: 50%;
  z-index: 1;
}
</style>
