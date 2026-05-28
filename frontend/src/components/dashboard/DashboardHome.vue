<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { DashboardStats, Incident, Maintenance } from '../../api/types'
import ServiceMatrix from '../ServiceMatrix.vue'
import { useToast } from '../../composables/useToast'

const emit = defineEmits<{
  navigate: [section: string, serviceId?: number]
}>()

const { show: toast } = useToast()

const stats = ref<DashboardStats | null>(null)
const loading = ref(true)
const dailyStats = ref<Map<number, [number, number, number][]>>(new Map())
const maintenances = ref<Maintenance[]>([])

const activeMaintenances = computed(() =>
  maintenances.value.filter(m => m.status === 'scheduled' || m.status === 'in_progress')
)

const overallUptime = computed(() => {
  if (!stats.value || stats.value.services.length === 0) return 0
  const total = stats.value.services.reduce((sum, s) => sum + s.uptime, 0)
  return total / stats.value.services.length
})

const avgLatency = computed(() => {
  if (!stats.value) return 0
  const withData = stats.value.services.filter(s => s.latency > 0)
  if (withData.length === 0) return 0
  return Math.round(withData.reduce((sum, s) => sum + s.latency, 0) / withData.length)
})

function getServiceDays(serviceId: number): [number, number, number][] {
  return dailyStats.value.get(serviceId) || []
}

function formatCST(iso: string): string {
  if (!iso) return ''
  const s = /[Z+-]/.test(iso) ? iso : iso + '+08:00'
  return new Date(s).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

const statusClass = (s: string) => {
  switch (s) {
    case 'operational': return 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15 dark:border-emerald-800'
    case 'degraded': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    case 'outage': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    default: return ''
  }
}

async function loadDailyStats() {
  if (!stats.value) return
  for (const svc of stats.value.services) {
    try {
      const res = await api.getServiceDailyStats(svc.id, 30)
      dailyStats.value.set(svc.id, res.data.days)
    } catch {
      dailyStats.value.set(svc.id, Array.from({ length: 30 }, () => [-1, -1, -1] as [number, number, number]))
    }
  }
}

onMounted(async () => {
  try {
    const [statsRes, maintRes] = await Promise.all([
      api.getStats(),
      api.getAdminMaintenances(),
    ])
    stats.value = statsRes.data
    maintenances.value = maintRes.data
    await loadDailyStats()
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">加载中...</div>

    <template v-if="stats">
      <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 md:gap-5">
        <div class="rounded-xl p-3 md:p-5 flex justify-between items-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div>
            <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">服务总数</div>
            <div class="text-2xl md:text-3xl font-bold " style="color: var(--text-color);">{{ stats.totalServices }}</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-blue-50 dark:bg-blue-900/30 text-blue-500 dark:text-blue-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect><line x1="8" y1="21" x2="16" y2="21"></line><line x1="12" y1="17" x2="12" y2="21"></line></svg>
          </div>
        </div>
        <div class="rounded-xl p-3 md:p-5 flex justify-between items-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div>
            <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">正常运行</div>
            <div class="text-2xl md:text-3xl font-bold " style="color: var(--text-color);">{{ stats.operationalCount }}</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-emerald-50 dark:bg-emerald-500/15 text-emerald-500 dark:text-emerald-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
          </div>
        </div>
        <div class="rounded-xl p-3 md:p-5 flex justify-between items-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div>
            <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">发生故障</div>
            <div class="text-2xl md:text-3xl font-bold " style="color: var(--text-color);">{{ stats.outageCount + stats.degradedCount }}</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-red-50 dark:bg-red-900/30 text-red-500 dark:text-red-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" /></svg>
          </div>
        </div>
        <div class="rounded-xl p-3 md:p-5 flex justify-between items-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div>
            <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">维护中</div>
            <div class="text-2xl md:text-3xl font-bold " style="color: var(--text-color);">{{ stats.activeMaintenances }}</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-amber-50 dark:bg-amber-900/30 text-amber-500 dark:text-amber-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" /></svg>
          </div>
        </div>
        <div class="rounded-xl p-3 md:p-5 flex justify-between items-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div>
            <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">整体在线率</div>
            <div class="text-2xl md:text-3xl font-bold " style="color: var(--text-color);">{{ overallUptime.toFixed(1) }}%</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-cyan-50 dark:bg-cyan-900/30 text-cyan-500 dark:text-cyan-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" /></svg>
          </div>
        </div>
        <div class="rounded-xl p-3 md:p-5 flex justify-between items-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div>
            <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">平均响应</div>
            <div class="text-2xl md:text-3xl font-bold " style="color: var(--text-color);">{{ avgLatency }}ms</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-purple-50 dark:bg-purple-900/30 text-purple-500 dark:text-purple-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" /></svg>
          </div>
        </div>
      </div>

      <!-- 近30天事件趋势 -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3 md:gap-5 mt-3 md:mt-5">
        <div class="rounded-xl p-3 md:p-5 flex justify-between items-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div>
            <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">近30天事件</div>
            <div class="text-2xl md:text-3xl font-bold " style="color: var(--text-color);">{{ stats.recentIncidentsTotal }}</div>
            <div class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">已解决 {{ stats.recentIncidentsResolved }}</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-indigo-50 dark:bg-indigo-900/30 text-indigo-500 dark:text-indigo-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 13h4v8H3zM10 9h4v12h-4zM17 5h4v16h-4z" /></svg>
          </div>
        </div>
        <div class="rounded-xl p-3 md:p-5 flex justify-between items-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div>
            <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">解决率</div>
            <div class="text-2xl md:text-3xl font-bold " style="color: var(--text-color);">
              {{ stats.recentIncidentsTotal > 0 ? ((stats.recentIncidentsResolved / stats.recentIncidentsTotal) * 100).toFixed(0) + '%' : 'N/A' }}
            </div>
            <div class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">近30天</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-teal-50 dark:bg-teal-900/30 text-teal-500 dark:text-teal-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
          </div>
        </div>
      </div>

      <div class="rounded-xl mt-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <div class="p-5 border-b flex justify-between items-center" style="border-color: var(--button-border-color);">
          <h3 class="font-bold " style="color: var(--text-color);">服务列表</h3>
          <button @click="emit('navigate', 'services')" class="bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium px-3 py-1.5 rounded-lg transition-colors">管理</button>
        </div>
        <div class="overflow-x-auto thin-scroll">
          <table class="w-full text-left text-sm">
          <thead class="text-xs border-b" style="color: var(--text-color); background-color: var(--button-hover-color); opacity: 0.5; border-color: var(--button-border-color);">
            <tr>
              <th class="px-6 py-3 font-medium">服务名称</th>
              <th class="px-6 py-3 font-medium">URL</th>
              <th class="px-6 py-3 font-medium">类型</th>
              <th class="px-6 py-3 font-medium">状态</th>
              <th class="px-6 py-3 font-medium">在线率</th>
              <th class="px-6 py-3 font-medium">响应时间</th>
              <th class="px-6 py-3 font-medium">探测频率</th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="svc in stats.services" :key="svc.id" @click="emit('navigate', 'services', svc.id)" class="cursor-pointer hover:bg-[var(--button-hover-color)]">
              <td class="px-6 py-4 font-bold " style="color: var(--text-color);">{{ svc.name }}</td>
              <td class="px-6 py-4 text-xs max-w-[200px] truncate" style="color: var(--text-color); opacity: 0.5;" :title="svc.url">{{ svc.url || '-' }}</td>
              <td class="px-6 py-4 " style="color: var(--text-color); opacity: 0.5;">{{ { http: 'HTTP', tcp: 'TCP', ping: 'Ping' }[svc.type] || svc.type }}</td>
              <td class="px-6 py-4">
                <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium border', statusClass(svc.status)]" :style="!['operational','degraded','outage'].includes(svc.status) ? 'color: var(--text-color); opacity: 0.6; background-color: var(--bg-color); border-color: var(--button-border-color);' : ''">
                  <span :class="['w-1.5 h-1.5 rounded-full',
                    svc.status === 'operational' ? 'bg-emerald-500' : svc.status === 'degraded' ? 'bg-yellow-400' : 'bg-red-500'
                  ]" />
                  {{ svc.status === 'operational' ? '正常' : svc.status === 'degraded' ? '性能下降' : '故障' }}
                </span>
              </td>
              <td class="px-6 py-4">
                <div class="flex items-center gap-2">
                  <span class="font-medium text-xs w-12 text-right flex-shrink-0" :class="svc.uptime >= 99.9 ? 'text-emerald-600' : ''" :style="svc.uptime >= 99.9 ? '' : 'color: var(--text-color); opacity: 0.6;'">{{ svc.uptime.toFixed(1) }}%</span>
                  <ServiceMatrix :days="getServiceDays(svc.id)" :uptime="svc.uptime" :hide-legend="true" :compact="true" class="flex-1" />
                </div>
              </td>
              <td class="px-6 py-4 " style="color: var(--text-color); opacity: 0.5;">{{ svc.latency }}ms</td>
              <td class="px-6 py-4 " style="color: var(--text-color); opacity: 0.5;">{{ svc.interval }}s</td>
            </tr>
          </tbody>
        </table>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-5 mt-6">
        <!-- Incidents card -->
        <div class="rounded-xl p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div class="flex justify-between items-center mb-4">
            <h3 class="font-bold " style="color: var(--text-color);">活跃事件</h3>
            <button @click="emit('navigate', 'incidents')" class="bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium px-3 py-1.5 rounded-lg transition-colors">管理</button>
          </div>
          <template v-if="stats.recentIncidents && stats.recentIncidents.length > 0">
            <div v-for="inc in stats.recentIncidents" :key="inc.id" @click="emit('navigate', 'incidents')" class="flex items-start gap-3 py-2 border-b last:border-0 cursor-pointer hover:bg-[var(--button-hover-color)] rounded-lg px-2 -mx-2" style="border-color: var(--button-border-color);">
              <div :class="['w-2 h-2 rounded-full mt-2',
                inc.impact === 'critical' ? 'bg-red-500' : inc.impact === 'major' ? 'bg-orange-500' : 'bg-yellow-400'
              ]" />
              <div>
                <div class="text-sm font-medium " style="color: var(--text-color);">{{ inc.title }}</div>
                <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.5;">{{ { investigating: '调查中', identified: '已确认', monitoring: '监控中', resolved: '已解决' }[inc.status] || inc.status }}</div>
              </div>
            </div>
          </template>
          <div v-else class="text-sm py-4 text-center" style="color: var(--text-color); opacity: 0.4;">暂无活跃事件</div>
        </div>

        <!-- Maintenance card -->
        <div class="rounded-xl p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div class="flex justify-between items-center mb-4">
            <h3 class="font-bold " style="color: var(--text-color);">维护计划</h3>
            <button @click="emit('navigate', 'maintenances')" class="bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium px-3 py-1.5 rounded-lg transition-colors">管理</button>
          </div>
          <template v-if="activeMaintenances.length > 0">
            <div v-for="m in activeMaintenances" :key="m.id" @click="emit('navigate', 'maintenances')" class="py-2 border-b last:border-0 cursor-pointer hover:bg-[var(--button-hover-color)] rounded-lg px-2 -mx-2" style="border-color: var(--button-border-color);">
              <div class="text-sm font-medium " style="color: var(--text-color);">{{ m.title }}</div>
              <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.5;">{{ formatCST(m.scheduledStart) }} - {{ formatCST(m.scheduledEnd) }}</div>
            </div>
          </template>
          <div v-else class="text-sm py-4 text-center" style="color: var(--text-color); opacity: 0.4;">暂无维护计划</div>
        </div>
      </div>
    </template>
  </div>
</template>
