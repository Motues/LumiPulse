<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api/client'
import type { SummaryResponse, Incident } from '../api/types'
import ServiceMatrix from '../components/ServiceMatrix.vue'
import { siteName, siteIcon, emailEnabled } from '../composables/useSiteConfig'
import { useDarkMode } from '../composables/useDarkMode'

const { isDark, toggle } = useDarkMode()

const summary = ref<SummaryResponse | null>(null)
const incidents = ref<Incident[]>([])
const loading = ref(true)
const error = ref('')

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

function openUrl(url: string) {
  window.open(url, '_blank')
}

const dailyStats = ref<Map<number, [number, number, number][]>>(new Map())
const isMobile = ref(false)
const hoveredSvcId = ref<number | null>(null)

function getServiceDays(serviceId: number): [number, number, number][] {
  return dailyStats.value.get(serviceId) || []
}

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
    error.value = e.message || '加载数据失败'
  } finally {
    loading.value = false
  }

  // Auto-refresh every 30s for real-time updates
  setInterval(async () => {
    try {
      const [sumRes, incRes] = await Promise.all([
        api.getSummary(),
        api.getPublicIncidents(1, 10),
      ])
      summary.value = sumRes.data
      incidents.value = incRes.data.incidents
    } catch {
      // silent
    }
  }, 30000)
})
</script>

<template>
  <nav class="border-b border-gray-100 dark:border-gray-800 bg-white dark:bg-gray-900">
    <div class="max-w-[1000px] mx-auto px-6 h-16 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <img v-if="siteIcon" :src="siteIcon" class="w-6 h-6 object-contain" />
        <img v-else src="/assets/logo.svg" class="w-6 h-6 object-contain" />
        <span class="text-xl font-bold tracking-tight text-gray-900 dark:text-white">{{ siteName }}</span>
      </div>
      <div class="flex items-center gap-3">
        <button
          @click="toggle"
          class="text-gray-400 dark:text-gray-500 hover:text-emerald-500 dark:hover:text-emerald-400 transition-colors"
          :title="isDark ? '切换浅色模式' : '切换深色模式'"
        >
          <svg v-if="isDark" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
          </svg>
          <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
          </svg>
        </button>
        <a v-if="emailEnabled" href="#" class="px-4 py-1.5 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 text-sm font-medium rounded-lg transition-colors">订阅更新</a>
      </div>
    </div>
  </nav>

  <main class="max-w-[1000px] mx-auto px-6 py-8">
    <div v-if="loading" class="text-center py-20 text-gray-400 dark:text-gray-500">加载中...</div>
    <div v-else-if="error" class="text-center py-20 text-red-500">{{ error }}</div>

    <template v-else-if="summary">
      <!-- Hero Banner -->
      <section :class="[
        'rounded-lg p-8 mb-8 flex items-center justify-between relative overflow-hidden border shadow-sm',
        summary.overallStatus === 'operational'
          ? 'bg-[#f0fdf4] dark:bg-[#0a2e1a] border-[#45ba65]/20 dark:border-[#45ba65]/30'
          : 'bg-[#fef2f2] dark:bg-[#3a1111] border-[#df2d2a]/20 dark:border-[#df2d2a]/30'
      ]">
        <div class="relative z-10">
            <h1 :class="['text-3xl font-bold mb-1', summary.overallStatus === 'operational' ? 'text-[#2d7a47] dark:text-[#4ade80]' : 'text-[#9e1f1e] dark:text-[#f87171]']">
              {{ summary.overallStatus === 'operational' ? '所有系统运行正常' : '系统出现故障' }}
            </h1>
        </div>
      </section>

      <!-- Maintenance Plans (if any) -->
      <section v-if="summary.maintenances && summary.maintenances.length > 0" class="bg-[#fafafa] dark:bg-gray-800/50 rounded-lg border border-gray-100 dark:border-gray-700 p-6 mb-8">
        <div>
          <h3 class="text-base font-bold text-gray-900 dark:text-gray-100 mb-2">维护计划</h3>
          <div v-for="m in summary.maintenances" :key="m.id" class="mb-2 last:mb-0">
            <p class="text-sm text-gray-800 dark:text-gray-200 font-medium">{{ m.title }}</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(m.scheduledStart) }} - {{ formatDateTime(m.scheduledEnd) }}</p>
          </div>
        </div>
      </section>

      <!-- Service Status -->
      <section class="bg-white dark:bg-gray-900 rounded-lg shadow-sm border border-gray-100 dark:border-gray-800 mb-8 pb-4">
        <div class="flex items-center justify-between p-6 border-b border-gray-50 dark:border-gray-800 mb-4">
          <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100">系统状态</h2>
        </div>

        <template v-for="(svc, idx) in summary.services" :key="svc.id">
          <div class="px-6 pb-6" :class="{ 'pt-4': idx > 0 }">
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center">
                <span class="font-bold text-gray-900 dark:text-gray-100 leading-none">{{ svc.name }}</span>
                <span
                  v-if="svc.url"
                  class="relative inline-flex items-center ml-1"
                  @mouseenter="hoveredSvcId = svc.id"
                  @mouseleave="hoveredSvcId = null"
                >
                  <svg
                    class="w-4 h-4 text-gray-400 dark:text-gray-500 hover:text-emerald-500 dark:hover:text-emerald-400 cursor-pointer transition-colors"
                    fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"
                    @click="openUrl(svc.url)"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
                  </svg>
                  <div
                    v-if="hoveredSvcId === svc.id"
                    class="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 px-3 py-1.5 bg-gray-800 dark:bg-gray-700 text-white dark:text-gray-100 text-xs rounded-lg whitespace-nowrap shadow-lg pointer-events-none z-10"
                  >
                    {{ svc.url }}
                    <div class="absolute left-1/2 -translate-x-1/2 top-full w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent border-t-gray-800 dark:border-t-gray-700" />
                  </div>
                </span>
              </div>
              <div class="flex items-center gap-1.5 text-sm font-medium" :class="{
                'text-[#45ba65] dark:text-[#4ade80]': svc.status === 'operational',
                'text-[#f9ac05] dark:text-[#fbbf24]': svc.status === 'degraded',
                'text-[#df2d2a] dark:text-[#f87171]': svc.status === 'outage',
              }">
                <div class="w-2 h-2 rounded-full" :style="{ backgroundColor: statusColors[svc.status] }"></div>
                {{ statusText[svc.status] }}
              </div>
            </div>
            <ServiceMatrix :days="getServiceDays(svc.id)" :uptime="svc.uptime" />
          </div>
          <hr v-if="idx < summary.services.length - 1" class="border-gray-100 dark:border-gray-800 mx-6" />
        </template>
      </section>

      <!-- Past Incidents -->
      <section v-if="incidents.length > 0" class="mb-8">
        <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-4">过去事件</h2>
        <div v-for="inc in incidents" :key="inc.id" class="bg-white dark:bg-gray-900 rounded-lg shadow-sm border border-gray-100 dark:border-gray-800 p-6 mb-4">
          <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between mb-2">
            <div class="flex items-center gap-1">
              <h3 class="text-lg font-bold" :class="{
                'text-red-500 dark:text-red-400': inc.impact === 'critical',
                'text-orange-500 dark:text-orange-400': inc.impact === 'major',
                'text-yellow-500 dark:text-yellow-400': inc.impact === 'minor',
              }">{{ inc.title }}</h3>
            </div>
            <div class="text-sm font-medium text-gray-700 dark:text-gray-300 mt-1 sm:mt-0">{{ formatDate(inc.createdAt) }}</div>
          </div>
          <p v-if="inc.updates && inc.updates.length > 0" class="text-sm text-gray-500 dark:text-gray-400 mb-6">
            {{ inc.updates[inc.updates.length - 1].content }}
          </p>
          <div v-if="inc.updates && inc.updates.length > 0" class="relative">
            <ul class="space-y-4 relative z-10">
              <li v-for="upd in inc.updates" :key="upd.id" class="relative pl-4">
                <div class="timeline-dot dark:bg-gray-500 dark:border-gray-900" />
                <div class="flex flex-col sm:flex-row sm:justify-between sm:items-start">
                  <div class="text-sm">
                    <span class="font-bold text-gray-900 dark:text-gray-100">{{ incidentStatusLabel[upd.status] || upd.status }}</span>
                    <span class="text-gray-500 dark:text-gray-400 ml-2">{{ upd.content }}</span>
                  </div>
                  <div class="text-sm text-gray-400 dark:text-gray-500 mt-0.5 sm:mt-0 sm:ml-4 sm:flex-shrink-0">{{ formatTime(upd.createdAt) }} CST</div>
                </div>
              </li>
            </ul>
          </div>
        </div>
      </section>

      <!-- No Maintenance -->
      <section v-if="!summary.maintenances || summary.maintenances.length === 0" class="bg-[#fafafa] dark:bg-gray-800/50 rounded-lg border border-gray-100 dark:border-gray-700 p-6 mb-8">
        <div>
          <h3 class="text-base font-bold text-gray-900 dark:text-gray-100 mb-2">维护计划</h3>
          <p class="text-sm text-gray-800 dark:text-gray-200 mb-1">暂无计划的维护</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">我们会提前通知受影响的服务维护计划。</p>
        </div>
      </section>
    </template>
  </main>

  <footer class="border-t border-gray-100 dark:border-gray-800 bg-white dark:bg-gray-900 mt-4">
    <div class="max-w-[1000px] mx-auto px-6 py-8 flex flex-col md:flex-row justify-between items-center gap-4">
      <div class="text-sm text-gray-500 dark:text-gray-400">
        <a href="https://github.com/Motues/LumiPulse" target="_blank" rel="noopener noreferrer" class="hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors">Powered By LumiPulse</a>
      </div>
      <div class="flex items-center gap-6 text-sm text-gray-500 dark:text-gray-400">
        <a href="/dashboard" class="hover:text-gray-900 dark:hover:text-gray-100 transition-colors">管理后台</a>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.timeline-dot {
  position: absolute;
  left: -5px;
  top: 6px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background-color: #9ca3af;
  border: 2px solid white;
  z-index: 1;
}
</style>
