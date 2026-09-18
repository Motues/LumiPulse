<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import type { SummaryResponse, Incident, ServiceSummary } from '../api/types'
import ServiceMatrix from '../components/ServiceMatrix.vue'
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

const statusColors: Record<string, string> = {
  operational: '#34a761',
  degraded: '#fda305',
  outage: '#df2d2a',
}

function statusText(status: string): string {
  switch (status) {
    case 'operational': return t('status.operational')
    case 'degraded': return t('status.degraded')
    case 'outage': return t('status.outage')
    default: return status
  }
}

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

function openUrl(url: string) {
  window.open(url, '_blank')
}

const dailyStats = ref<Map<number, [number, number, number][]>>(new Map())
const isMobile = ref(false)
const hoveredSvcId = ref<number | null>(null)

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

function getServiceDays(serviceId: number): [number, number, number][] {
  return dailyStats.value.get(serviceId) || []
}

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
    const map = new Map<number, [number, number, number][]>()
    for (const item of res.data.services) {
      map.set(item.serviceId, item.days)
    }
    for (const svc of summary.value.services) {
      if (!map.has(svc.id)) {
        map.set(svc.id, Array.from({ length: days }, () => [-1, -1, -1] as [number, number, number]))
      }
    }
    dailyStats.value = map
  } catch {
    const map = new Map<number, [number, number, number][]>()
    for (const svc of summary.value.services) {
      map.set(svc.id, Array.from({ length: days }, () => [-1, -1, -1] as [number, number, number]))
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
      <!-- Hero Banner -->
      <section :class="[
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

        <template v-for="(svc, idx) in summary.services" :key="svc.id">
          <div
            class="px-6 pb-6 rounded-lg transition-colors"
            :class="{ 'pt-4': idx > 0 }"
          >
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center">
                <span
                  class="font-bold leading-none cursor-pointer text-[color:var(--text-color)] hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors"
                  :title="t('home.viewServiceDetail')"
                  @click="openService(svc)"
                >{{ svc.name }}</span>
                <span
                  v-if="svc.url"
                  class="relative inline-flex items-center ml-1"
                  @mouseenter="hoveredSvcId = svc.id"
                  @mouseleave="hoveredSvcId = null"
                >
                  <svg
                    class="w-4 h-4 cursor-pointer transition-colors text-[color:var(--text-color)] opacity-40 hover:text-emerald-500 dark:hover:text-emerald-400"
                    fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"
                    @click.stop="openUrl(svc.url)"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
                  </svg>
                  <div
                    v-if="hoveredSvcId === svc.id"
                    class="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 px-3 py-1.5 text-xs rounded-lg whitespace-nowrap shadow-lg pointer-events-none z-10"
                    :style="{ backgroundColor: 'var(--button-hover-color)', color: 'var(--text-color)', border: '1px solid var(--button-border-color)' }"
                  >
                    {{ svc.url }}
                    <div class="absolute left-1/2 -translate-x-1/2 top-full w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent" :style="{ borderTopColor: 'var(--button-hover-color)' }" />
                  </div>
                </span>
              </div>
              <div class="flex items-center gap-1.5 text-sm font-medium" :class="{
                'text-[#45ba65] dark:text-[#4ade80]': svc.status === 'operational' && !isServiceInMaintenance(svc.id),
                'text-[#f9ac05] dark:text-[#fbbf24]': svc.status === 'degraded',
                'text-[#df2d2a] dark:text-[#f87171]': svc.status === 'outage',
                'text-gray-400 dark:text-gray-500': isServiceInMaintenance(svc.id),
              }">
                <div v-if="isServiceInMaintenance(svc.id)" class="w-2 h-2 rounded-full bg-gray-400 dark:bg-gray-500"></div>
                <div v-else class="w-2 h-2 rounded-full" :style="{ backgroundColor: statusColors[svc.status] }"></div>
                {{ isServiceInMaintenance(svc.id) ? t('status.maintenance') : statusText(svc.status) }}
              </div>
            </div>
            <ServiceMatrix :days="getServiceDays(svc.id)" :uptime="svc.uptime" :service-id="svc.id" :incidents-by-date="incidentsByDate" />
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
