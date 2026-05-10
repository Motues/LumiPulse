<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { DashboardStats, Incident, Maintenance } from '../../api/types'
import ServiceMatrix from '../ServiceMatrix.vue'
import { useToast } from '../../composables/useToast'

const emit = defineEmits<{
  navigate: [section: string]
}>()

const { show: toast } = useToast()

const stats = ref<DashboardStats | null>(null)
const loading = ref(true)
const dailyStats = ref<Map<number, [number, number, number][]>>(new Map())
const maintenances = ref<Maintenance[]>([])

const activeMaintenances = computed(() =>
  maintenances.value.filter(m => m.status === 'scheduled' || m.status === 'in_progress')
)

function getServiceDays(serviceId: number): [number, number, number][] {
  return dailyStats.value.get(serviceId) || []
}

const statusClass = (s: string) => {
  switch (s) {
    case 'operational': return 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-900/30 dark:border-emerald-800'
    case 'degraded': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    case 'outage': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    default: return 'text-gray-600 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700'
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
    <div v-if="loading" class="text-center py-12 text-gray-400 dark:text-gray-500">加载中...</div>

    <template v-if="stats">
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-5">
        <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 p-3 md:p-5 flex justify-between items-center shadow-sm">
          <div>
            <div class="text-sm text-gray-500 dark:text-gray-400 mb-1">服务总数</div>
            <div class="text-2xl md:text-3xl font-bold text-gray-900 dark:text-gray-100">{{ stats.totalServices }}</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-blue-50 dark:bg-blue-900/30 text-blue-500 dark:text-blue-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect><line x1="8" y1="21" x2="16" y2="21"></line><line x1="12" y1="17" x2="12" y2="21"></line></svg>
          </div>
        </div>
        <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 p-5 flex justify-between items-center shadow-sm">
          <div>
            <div class="text-sm text-gray-500 dark:text-gray-400 mb-1">正常运行</div>
            <div class="text-2xl md:text-3xl font-bold text-gray-900 dark:text-gray-100">{{ stats.operationalCount }}</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-emerald-50 dark:bg-emerald-900/30 text-emerald-500 dark:text-emerald-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
          </div>
        </div>
        <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 p-5 flex justify-between items-center shadow-sm">
          <div>
            <div class="text-sm text-gray-500 dark:text-gray-400 mb-1">发生故障</div>
            <div class="text-2xl md:text-3xl font-bold text-gray-900 dark:text-gray-100">{{ stats.outageCount + stats.degradedCount }}</div>
          </div>
          <div class="w-8 h-8 md:w-12 md:h-12 rounded-full bg-red-50 dark:bg-red-900/30 text-red-500 dark:text-red-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" /></svg>
          </div>
        </div>
        <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 p-5 flex justify-between items-center shadow-sm">
          <div>
            <div class="text-sm text-gray-500 dark:text-gray-400 mb-1">维护中</div>
            <div class="text-2xl md:text-3xl font-bold text-gray-900 dark:text-gray-100">{{ stats.activeMaintenances }}</div>
          </div>
          <div class="w-12 h-12 rounded-full bg-amber-50 dark:bg-amber-900/30 text-amber-500 dark:text-amber-400 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" /></svg>
          </div>
        </div>
      </div>

      <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 mt-6 shadow-sm">
        <div class="p-5 border-b border-gray-50 dark:border-gray-800 flex justify-between items-center">
          <h3 class="font-bold text-gray-900 dark:text-gray-100">服务列表</h3>
          <button @click="emit('navigate', 'services')" class="bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium px-3 py-1.5 rounded-lg transition-colors">管理</button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
          <thead class="text-xs text-gray-400 dark:text-gray-500 bg-gray-50/50 dark:bg-gray-800/50 border-b border-gray-100 dark:border-gray-800">
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
          <tbody class="divide-y divide-gray-50 dark:divide-gray-800">
            <tr v-for="svc in stats.services" :key="svc.id" class="hover:bg-gray-50/50 dark:hover:bg-gray-800/50">
              <td class="px-6 py-4 font-bold text-gray-900 dark:text-gray-100">{{ svc.name }}</td>
              <td class="px-6 py-4 text-xs text-gray-500 dark:text-gray-400 max-w-[200px] truncate" :title="svc.url">{{ svc.url || '-' }}</td>
              <td class="px-6 py-4 text-gray-500 dark:text-gray-400">{{ { http: 'HTTP', tcp: 'TCP', ping: 'Ping' }[svc.type] || svc.type }}</td>
              <td class="px-6 py-4">
                <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium border', statusClass(svc.status)]">
                  <span :class="['w-1.5 h-1.5 rounded-full',
                    svc.status === 'operational' ? 'bg-emerald-500' : svc.status === 'degraded' ? 'bg-yellow-400' : 'bg-red-500'
                  ]" />
                  {{ svc.status === 'operational' ? '正常' : svc.status === 'degraded' ? '性能下降' : '故障' }}
                </span>
              </td>
              <td class="px-6 py-4">
                <div class="flex items-center gap-2">
                  <span class="font-medium text-xs w-12 text-right flex-shrink-0" :class="svc.uptime >= 99.9 ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-600 dark:text-gray-400'">{{ svc.uptime.toFixed(1) }}%</span>
                  <ServiceMatrix :days="getServiceDays(svc.id)" :uptime="svc.uptime" :hide-legend="true" :compact="true" class="flex-1" />
                </div>
              </td>
              <td class="px-6 py-4 text-gray-500 dark:text-gray-400">{{ svc.latency }}ms</td>
              <td class="px-6 py-4 text-gray-500 dark:text-gray-400">{{ svc.interval }}s</td>
            </tr>
          </tbody>
        </table>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-5 mt-6">
        <!-- Incidents card -->
        <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 p-5 shadow-sm">
          <h3 class="font-bold text-gray-900 dark:text-gray-100 mb-4">活跃事件</h3>
          <template v-if="stats.recentIncidents && stats.recentIncidents.length > 0">
            <div v-for="inc in stats.recentIncidents" :key="inc.id" class="flex items-start gap-3 py-2 border-b border-gray-50 dark:border-gray-800 last:border-0">
              <div :class="['w-2 h-2 rounded-full mt-2',
                inc.impact === 'critical' ? 'bg-red-500' : inc.impact === 'major' ? 'bg-orange-500' : 'bg-yellow-400'
              ]" />
              <div>
                <div class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ inc.title }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ { investigating: '调查中', identified: '已确认', monitoring: '监控中', resolved: '已解决' }[inc.status] || inc.status }}</div>
              </div>
            </div>
          </template>
          <div v-else class="text-sm text-gray-400 dark:text-gray-500 py-4 text-center">暂无活跃事件</div>
        </div>

        <!-- Maintenance card -->
        <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 p-5 shadow-sm">
          <h3 class="font-bold text-gray-900 dark:text-gray-100 mb-4">维护计划</h3>
          <template v-if="activeMaintenances.length > 0">
            <div v-for="m in activeMaintenances" :key="m.id" class="py-2 border-b border-gray-50 dark:border-gray-800 last:border-0">
              <div class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ m.title }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ new Date(m.scheduledStart).toLocaleString('zh-CN') }} - {{ new Date(m.scheduledEnd).toLocaleString('zh-CN') }}</div>
            </div>
          </template>
          <div v-else class="text-sm text-gray-400 dark:text-gray-500 py-4 text-center">暂无维护计划</div>
        </div>
      </div>
    </template>
  </div>
</template>
