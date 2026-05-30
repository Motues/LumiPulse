<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'
import type { SummaryResponse, Incident, ServiceSummary } from '../api/types'
import ServiceMatrix from '../components/ServiceMatrix.vue'
import PublicServiceDetail from '../components/PublicServiceDetail.vue'
import { siteName, siteIcon, emailEnabled, showAdminButton, customFooter, subEnabledAny, subEnableEmail, subEnableRss, subEnableAtom } from '../composables/useSiteConfig'
import { useDarkMode } from '../composables/useDarkMode'

const router = useRouter()

const { isDark, themeMode, setMode } = useDarkMode()

const showThemeMenu = ref(false)

const summary = ref<SummaryResponse | null>(null)
const incidents = ref<Incident[]>([])
const loading = ref(true)
const error = ref('')
const showSubscribeModal = ref(false)
const subscribeTab = ref<'rss' | 'atom' | 'email'>('rss')
const showServiceSelect = ref(false)
const subscribeEmail = ref('')
const subscribing = ref(false)
const subscribeMsg = ref('')
const subscribeMsgType = ref('success')
const selectedServices = ref<number[]>([])
const servicesList = ref<ServiceSummary[]>([])

function openSubscribe() {
  subscribeEmail.value = ''
  subscribeMsg.value = ''
  selectedServices.value = []
  showServiceSelect.value = false
  // Pick first available tab
  subscribeTab.value = subEnableEmail.value ? 'email' : subEnableRss.value ? 'rss' : 'atom'
  // Load services
  if (summary.value?.services) {
    servicesList.value = summary.value.services
  }
  showSubscribeModal.value = true
}

async function handleSubscribe() {
  const email = subscribeEmail.value.trim()
  if (!email) return
  subscribing.value = true
  subscribeMsg.value = ''
  try {
    await api.subscribe(email, selectedServices.value.length > 0 ? selectedServices.value : undefined)
    subscribeMsg.value = '订阅成功！我们将通过邮件通知您服务状态变更。'
    subscribeMsgType.value = 'success'
    subscribeEmail.value = ''
    selectedServices.value = []
  } catch (e: any) {
    subscribeMsg.value = e.message || '订阅失败，请稍后重试'
    subscribeMsgType.value = 'error'
  } finally {
    subscribing.value = false
  }
}

function closeSubscribeModal() {
  showSubscribeModal.value = false
  subscribeMsg.value = ''
}

function toggleService(id: number) {
  const idx = selectedServices.value.indexOf(id)
  if (idx >= 0) {
    selectedServices.value.splice(idx, 1)
  } else {
    selectedServices.value.push(id)
  }
}

function getFeedUrl(type: 'rss' | 'atom') {
  return `${window.location.protocol}//${window.location.host}/feed/${type}`
}

function copyFeedUrl(type: 'rss' | 'atom') {
  navigator.clipboard.writeText(getFeedUrl(type)).then(() => {
    subscribeMsg.value = '链接已复制到剪贴板'
    subscribeMsgType.value = 'success'
    setTimeout(() => { subscribeMsg.value = '' }, 2000)
  })
}


const statusColors: Record<string, string> = {
  operational: '#34a761',
  degraded: '#fda305',
  outage: '#df2d2a',
}

const statusText: Record<string, string> = {
  operational: '正常',
  degraded: '异常',
  outage: '故障',
}

const incidentStatusLabel: Record<string, string> = {
  investigating: '调查中',
  identified: '已确认',
  monitoring: '监控中',
  resolved: '已解决',
}

const dateTimeOpts: Intl.DateTimeFormatOptions = { timeZone: 'Asia/Shanghai' }

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('zh-CN', { ...dateTimeOpts, year: 'numeric', month: 'long', day: 'numeric' })
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('zh-CN', { ...dateTimeOpts, hour: '2-digit', minute: '2-digit' })
}

function formatDateTime(iso: string): string {
  return `${formatDate(iso)} ${formatTime(iso)}`
}

function affectedServiceNames(ids: string): string {
  if (!ids || !summary.value) return '-'
  return ids.split(',').map(id => {
    const svc = summary.value!.services.find(s => s.id === Number(id))
    return svc ? svc.name : id
  }).join(', ')
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
const selectedService = ref<ServiceSummary | null>(null)

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
  for (const svc of summary.value.services) {
    try {
      const res = await api.getServiceDailyStats(svc.id, days)
      dailyStats.value.set(svc.id, res.data.days)
    } catch {
      dailyStats.value.set(svc.id, Array.from({ length: days }, () => [-1, -1, -1] as [number, number, number]))
    }
  }
}

let refreshTimer: ReturnType<typeof setInterval> | null = null

function closeThemeMenu() {
  showThemeMenu.value = false
}

onMounted(async () => {
  document.addEventListener('click', closeThemeMenu)
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
    error.value = e.message || '加载数据失败'
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
      error.value = '数据刷新失败'
    }
  }, 30000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  document.removeEventListener('click', closeThemeMenu)
})
</script>

<template>
  <nav class="mt-4" style="background-color: var(--bg-color);">
    <div class="max-w-[1000px] mx-auto px-6 h-16 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <img v-if="siteIcon" :src="siteIcon" class="w-6 h-6 object-contain" />
        <img v-else src="/assets/logo.svg" class="w-6 h-6 object-contain" />
        <span class="text-xl font-bold tracking-tight" style="color: var(--text-color);">{{ siteName }}</span>
      </div>
      <div class="flex items-center gap-3">
        <a v-if="subEnabledAny" href="#" @click.prevent="openSubscribe" class="header-btn px-4 py-2 text-sm font-medium rounded-lg">订阅更新</a>
        <div class="relative">
          <button
            @click.stop="showThemeMenu = !showThemeMenu"
            class="header-btn px-2 py-2 rounded-lg"
            title="切换主题"
          >
            <svg v-if="themeMode === 'light'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
            </svg>
            <svg v-else-if="themeMode === 'dark'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401" />
            </svg>
            <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25A2.25 2.25 0 015.25 3h13.5A2.25 2.25 0 0121 5.25z" />
            </svg>
          </button>
          <div
            v-if="showThemeMenu"
            class="absolute right-0 top-full mt-2 rounded-lg shadow-lg py-1.5 px-1.5 z-50 flex flex-col gap-0.5"
            :style="{ backgroundColor: 'var(--bg-color)', border: '1px solid var(--button-border-color)', width: '140px' }"
            @click.stop
          >
            <button
              @click="setMode('light'); showThemeMenu = false"
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors rounded-md"
              :style="{ color: 'var(--text-color)', backgroundColor: themeMode === 'light' ? 'var(--button-hover-color)' : 'transparent' }"
              onmouseover="this.style.backgroundColor='var(--button-hover-color)'"
              onmouseout="this.style.backgroundColor=this.dataset.active === 'true' ? 'var(--button-hover-color)' : 'transparent'"
              :data-active="themeMode === 'light'"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" /></svg>
              浅色模式
            </button>
            <button
              @click="setMode('dark'); showThemeMenu = false"
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors rounded-md"
              :style="{ color: 'var(--text-color)', backgroundColor: themeMode === 'dark' ? 'var(--button-hover-color)' : 'transparent' }"
              onmouseover="this.style.backgroundColor='var(--button-hover-color)'"
              onmouseout="this.style.backgroundColor=this.dataset.active === 'true' ? 'var(--button-hover-color)' : 'transparent'"
              :data-active="themeMode === 'dark'"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401" /></svg>
              深色模式
            </button>
            <button
              @click="setMode('system'); showThemeMenu = false"
              class="w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors rounded-md"
              :style="{ color: 'var(--text-color)', backgroundColor: themeMode === 'system' ? 'var(--button-hover-color)' : 'transparent' }"
              onmouseover="this.style.backgroundColor='var(--button-hover-color)'"
              onmouseout="this.style.backgroundColor=this.dataset.active === 'true' ? 'var(--button-hover-color)' : 'transparent'"
              :data-active="themeMode === 'system'"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25A2.25 2.25 0 015.25 3h13.5A2.25 2.25 0 0121 5.25z" /></svg>
              跟随系统
            </button>
          </div>
        </div>
      </div>
    </div>
  </nav>

  <main class="max-w-[1000px] mx-auto px-6 py-8">
    <div v-if="loading" class="text-center py-20" style="color: var(--text-color); opacity: 0.4;">加载中...</div>
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
              {{ summary.overallStatus === 'operational' ? '所有系统运行正常' : '系统出现故障' }}
            </h1>
        </div>
      </section>

      <!-- Maintenance Plans (if any) -->
      <section v-if="summary.maintenances && summary.maintenances.length > 0" class="rounded-lg p-6 mb-8" style="border: 1px solid var(--button-border-color);">
        <div>
          <h3 class="text-base font-bold mb-2" style="color: var(--text-color);">维护计划</h3>
          <div v-for="m in summary.maintenances" :key="m.id" class="mb-3 last:mb-0 pb-3 last:pb-0">
            <p class="text-sm font-medium" style="color: var(--text-color);">{{ m.title }}</p>
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.5;">
              {{ formatDateTime(m.scheduledStart) }} CST - {{ formatDateTime(m.scheduledEnd) }} CST
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
        @back="selectedService = null"
      />

      <!-- Service Status -->
      <section v-else class="rounded-lg mb-8 pb-4" style="border: 1px solid var(--button-border-color);">
        <div class="flex items-center justify-between p-6 pb-4 mb-4">
          <h2 class="text-lg font-bold" style="color: var(--text-color);">系统状态</h2>
        </div>

        <template v-for="(svc, idx) in summary.services" :key="svc.id">
          <div
            class="px-6 pb-6 rounded-lg transition-colors"
            :class="{ 'pt-4': idx > 0 }"
          >
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center">
                <span class="font-bold leading-none hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors" style="color: var(--text-color);">{{ svc.name }}</span>
                <span
                  v-if="svc.url"
                  class="relative inline-flex items-center ml-1"
                  @mouseenter="hoveredSvcId = svc.id"
                  @mouseleave="hoveredSvcId = null"
                >
                  <svg
                    class="w-4 h-4 cursor-pointer transition-colors hover:text-emerald-500 dark:hover:text-emerald-400"
                    style="color: var(--text-color); opacity: 0.4;"
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
                {{ isServiceInMaintenance(svc.id) ? '维护中' : statusText[svc.status] }}
              </div>
            </div>
            <ServiceMatrix :days="getServiceDays(svc.id)" :uptime="svc.uptime" :service-id="svc.id" :incidents-by-date="incidentsByDate" />
          </div>
        </template>
      </section>

      <!-- Past Incidents -->
      <section v-if="incidents.length > 0" class="mb-8">
        <h2 class="text-lg font-bold mb-4" style="color: var(--text-color);">过去事件</h2>
        <div v-for="inc in incidents" :key="inc.id" class="rounded-lg p-6 mb-4 cursor-pointer hover:border-emerald-500/30 dark:hover:border-emerald-400/30 transition-colors" style="border: 1px solid var(--button-border-color);" @click="router.push(`/incidents/${inc.id}`)">
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
              }">{{ incidentStatusLabel[inc.updates[inc.updates.length - 1].status] || inc.updates[inc.updates.length - 1].status }}</span>
            </div>
            <div class="text-sm font-medium" style="color: var(--text-color); opacity: 0.5;">{{ formatDate(inc.createdAt) }}</div>
          </div>
          <p v-if="inc.updates && inc.updates.length > 0" class="text-sm mt-2" style="color: var(--text-color); opacity: 0.5;">
            {{ inc.updates[inc.updates.length - 1].content }}
          </p>
        </div>
      </section>

      <!-- No Maintenance -->
      <section v-if="!summary.maintenances || summary.maintenances.length === 0" class="rounded-lg p-6 mb-8" style="border: 1px solid var(--button-border-color);">
        <div>
          <h3 class="text-base font-bold mb-2" style="color: var(--text-color);">维护计划</h3>
          <p class="text-sm mb-1" style="color: var(--text-color);">暂无计划的维护</p>
          <p class="text-sm" style="color: var(--text-color); opacity: 0.5;">我们会提前通知受影响的服务维护计划。</p>
        </div>
      </section>
</template>
  </main>

  <!-- Subscribe Modal -->
  <Teleport to="body">
    <div v-if="showSubscribeModal" class="fixed inset-0 z-50 flex items-center justify-center" @click.self="closeSubscribeModal">
      <div class="absolute inset-0 bg-black/40" />
      <div class="relative rounded-xl shadow-xl p-6 w-full max-w-md mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-base font-bold" style="color: var(--text-color);">订阅更新</h3>
          <button @click="closeSubscribeModal" class="p-1 rounded-lg transition-colors hover:bg-[var(--button-hover-color)]" style="color: var(--text-color); opacity: 0.5;">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>
        </div>

        <!-- Tabs -->
        <div class="flex border-b mb-4" style="border-color: var(--button-border-color);">
          <button
            v-for="tab in [
              subEnableEmail ? { key: 'email', label: '邮件订阅' } : null,
              subEnableRss ? { key: 'rss', label: 'RSS' } : null,
              subEnableAtom ? { key: 'atom', label: 'Atom' } : null,
            ].filter(Boolean)"
            :key="(tab as any).key"
            @click="subscribeTab = (tab as any).key; subscribeMsg = ''"
            class="px-4 py-2 text-sm font-medium transition-colors -mb-px"
            :class="subscribeTab === (tab as any).key ? 'border-b-2 border-emerald-500 text-emerald-600' : 'opacity-50 hover:opacity-80'"
            style="color: subscribeTab === (tab as any).key ? '' : 'var(--text-color)';"
          >{{ (tab as any).label }}</button>
        </div>

        <!-- Email Tab -->
        <div v-if="subscribeTab === 'email'">
          <p class="text-sm mb-3" style="color: var(--text-color); opacity: 0.6;">获取服务状态变更和事件通知。</p>
            <div class="flex gap-2 mb-3">
              <input
                v-model="subscribeEmail"
                type="email"
                placeholder="输入邮箱地址..."
                class="flex-1 px-4 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500"
                style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
              />
              <button
                @click="handleSubscribe"
                :disabled="subscribing || !subscribeEmail"
                class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:opacity-40 text-white text-sm font-medium rounded-lg transition-colors"
              >
                {{ subscribing ? '提交中...' : '订阅' }}
              </button>
            </div>

            <!-- Service selection toggle -->
            <div v-if="servicesList.length > 0">
              <button @click="showServiceSelect = !showServiceSelect" type="button" class="text-xs flex items-center gap-1 transition-colors hover:opacity-80" style="color: var(--text-color); opacity: 0.5;">
                <svg class="w-3.5 h-3.5" :class="showServiceSelect ? 'rotate-90' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" /></svg>
                {{ showServiceSelect ? '收起' : '选择特定服务' }}
              </button>
              <div v-if="showServiceSelect" class="mt-2">
                <button
                  @click="selectedServices = (selectedServices.length === servicesList.length ? [] : servicesList.map(s => s.id))"
                  class="text-xs mb-2 transition-colors"
                  style="color: var(--text-color); opacity: 0.5;"
                >{{ selectedServices.length === servicesList.length ? '取消全选' : '全选' }}</button>
                <div class="max-h-36 overflow-y-auto space-y-1 thin-scroll p-1">
                  <label
                    v-for="svc in servicesList"
                    :key="svc.id"
                    class="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm cursor-pointer transition-colors hover:bg-[var(--button-hover-color)]"
                  >
                    <input
                      type="checkbox"
                      :checked="selectedServices.includes(svc.id)"
                      @change="toggleService(svc.id)"
                      class="rounded border-gray-300 text-emerald-500 focus:ring-emerald-500"
                    />
                    <span style="color: var(--text-color);">{{ svc.name }}</span>
                  </label>
                </div>
                <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">留空则订阅所有服务</p>
              </div>
            </div>
        </div>

        <!-- RSS Tab -->
        <div v-if="subscribeTab === 'rss'">
          <p class="text-sm mb-3" style="color: var(--text-color); opacity: 0.6;">复制以下链接到 RSS 阅读器订阅状态更新。</p>
          <div class="flex gap-2">
            <input
              :value="getFeedUrl('rss')"
              readonly
              class="flex-1 px-3 py-2 rounded-lg text-xs font-mono focus:outline-none"
              style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
            />
            <button @click="copyFeedUrl('rss')" class="px-3 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium rounded-lg transition-colors whitespace-nowrap">
              复制
            </button>
          </div>
        </div>

        <!-- Atom Tab -->
        <div v-if="subscribeTab === 'atom'">
          <p class="text-sm mb-3" style="color: var(--text-color); opacity: 0.6;">复制以下链接到 RSS 阅读器订阅状态更新。</p>
          <div class="flex gap-2">
            <input
              :value="getFeedUrl('atom')"
              readonly
              class="flex-1 px-3 py-2 rounded-lg text-xs font-mono focus:outline-none"
              style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
            />
            <button @click="copyFeedUrl('atom')" class="px-3 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium rounded-lg transition-colors whitespace-nowrap">
              复制
            </button>
          </div>
        </div>

        <p v-if="subscribeMsg" :class="['text-xs mt-3', subscribeMsgType === 'success' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-500']">{{ subscribeMsg }}</p>
        <button
          v-if="subscribeMsgType === 'success' && subscribeTab === 'email'"
          @click="closeSubscribeModal"
          class="mt-3 w-full px-4 py-2 text-sm font-medium rounded-lg modal-close-btn"
        >
          关闭
        </button>
      </div>
    </div>
  </Teleport>

  <footer style="background-color: var(--bg-color);" class="mt-4">
    <div class="max-w-[1000px] mx-auto px-6 py-8 flex flex-col md:flex-row justify-between items-center gap-4">
      <div class="text-sm">
        <a href="https://github.com/Motues/LumiPulse" target="_blank" rel="noopener noreferrer" class="footer-link transition-opacity">Powered By LumiPulse</a>
      </div>
      <div class="flex items-center gap-6 text-sm">
        <span v-if="customFooter" v-html="customFooter" class="footer-link"></span>
        <a v-else-if="showAdminButton" href="/dashboard" class="footer-link transition-opacity">管理后台</a>
      </div>
    </div>
  </footer>
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
