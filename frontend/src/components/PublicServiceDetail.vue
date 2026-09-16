<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { api } from '../api/client'
import type { ServiceSummary } from '../api/types'
import ServiceMatrix from './ServiceMatrix.vue'
import LatencyChart from './LatencyChart.vue'

const props = defineProps<{
  service: ServiceSummary
  dailyDays: [number, number, number][]
}>()

const emit = defineEmits<{
  back: []
}>()

const latencies = ref<number[]>([])
const statuses = ref<number[]>([])
const startTime = ref('')
const intervalMin = ref(5)
const loading = ref(true)

const statusText: Record<string, string> = {
  operational: '正常',
  degraded: '异常',
  outage: '故障',
}

const statusColors: Record<string, string> = {
  operational: '#34a761',
  degraded: '#fda305',
  outage: '#df2d2a',
}

const hasData = computed(() => statuses.value.some(s => s !== -1))

/** 图表概览：平均延迟 / 峰值 / 故障桶数量 */
const chartStats = computed(() => {
  const vals = latencies.value.filter((_, i) => statuses.value[i] !== -1)
  if (vals.length === 0) return null
  const sum = vals.reduce((a, b) => a + b, 0)
  return {
    avg: Math.round(sum / vals.length),
    max: Math.max(...vals),
    failures: statuses.value.filter(s => s === 1).length,
  }
})

async function loadLatency() {
  const hash = props.service.publicHash
  if (!hash) {
    latencies.value = []
    statuses.value = []
    loading.value = false
    return
  }
  loading.value = true
  try {
    const res = await api.getServiceLatency(hash, 1)
    latencies.value = res.data.latencies
    statuses.value = res.data.statuses
    startTime.value = res.data.start
    intervalMin.value = res.data.interval
  } catch {
    latencies.value = []
    statuses.value = []
  } finally {
    loading.value = false
  }
}

onMounted(loadLatency)
// 公开接口按 hash 定位，服务对象被刷新替换时重新拉取
watch(() => props.service.publicHash, loadLatency)
</script>

<template>
  <div>
    <div class="flex items-center mb-4">
      <button @click="emit('back')" class="flex items-center gap-1.5 text-sm transition-colors text-[color:var(--text-color)] opacity-50 hover:opacity-80">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
        返回列表
      </button>
    </div>

    <!-- Info card -->
    <div class="rounded-lg p-6 mb-6" style="border: 1px solid var(--button-border-color);">
      <div class="flex items-start justify-between mb-4">
        <div>
          <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ service.name }}</h2>
          <a v-if="service.url" :href="service.url" target="_blank" class="text-sm transition-colors text-[color:var(--text-color)] opacity-50 hover:text-emerald-500 dark:hover:text-emerald-400 mt-1 inline-block">{{ service.url }}</a>
        </div>
        <div class="flex items-center gap-1.5 text-sm font-medium" :class="{
          'text-[#45ba65] dark:text-[#4ade80]': service.status === 'operational',
          'text-[#f9ac05] dark:text-[#fbbf24]': service.status === 'degraded',
          'text-[#df2d2a] dark:text-[#f87171]': service.status === 'outage',
        }">
          <div class="w-2 h-2 rounded-full" :style="{ backgroundColor: statusColors[service.status] }" />
          {{ statusText[service.status] || service.status }}
        </div>
      </div>
      <div class="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">在线率</div>
          <div class="font-medium" :class="service.uptime >= 99.9 ? 'text-emerald-600 dark:text-emerald-400' : ''" :style="service.uptime < 99.9 ? 'color: var(--text-color);' : ''">{{ service.uptime.toFixed(2) }}%</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">响应时间</div>
          <div class="font-medium" style="color: var(--text-color);">{{ service.latency }}ms</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">探测频率</div>
          <div class="font-medium" style="color: var(--text-color);">{{ service.interval }}s</div>
        </div>
      </div>
    </div>

    <!-- Latency chart -->
    <div class="rounded-lg p-6 mb-6" style="border: 1px solid var(--button-border-color);">
      <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
        <h3 class="font-bold" style="color: var(--text-color);">最近24小时延迟</h3>
        <div v-if="chartStats" class="flex items-center gap-3 text-xs" style="color: var(--text-color); opacity: 0.55;">
          <span>平均 <span class="font-semibold" style="opacity: 0.9;">{{ chartStats.avg }}ms</span></span>
          <span>峰值 <span class="font-semibold" style="opacity: 0.9;">{{ chartStats.max }}ms</span></span>
          <span v-if="chartStats.failures > 0" class="text-[#df2d2a] dark:text-[#f87171]">故障 {{ chartStats.failures }} 次</span>
        </div>
      </div>
      <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">加载中...</div>
      <div v-else-if="!hasData" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">暂无数据</div>
      <LatencyChart
        v-else
        :latencies="latencies"
        :statuses="statuses"
        :start-time="startTime"
        :interval-min="intervalMin"
        ring-color="var(--bg-color)"
      />
    </div>

    <!-- Service history matrix -->
    <div class="rounded-lg p-6" style="border: 1px solid var(--button-border-color);">
      <h3 class="font-bold mb-4" style="color: var(--text-color);">服务历史</h3>
      <ServiceMatrix :days="dailyDays" :uptime="service.uptime" />
    </div>
  </div>
</template>
