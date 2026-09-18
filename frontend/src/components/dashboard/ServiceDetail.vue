<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { api } from '../../api/client'
import type { ServiceDetail as ServiceDetailType, LatencyStats } from '../../api/types'
import { useToast } from '../../composables/useToast'
import LatencyChart from '../LatencyChart.vue'

const props = defineProps<{
  service: ServiceDetailType
}>()

const emit = defineEmits<{
  back: []
}>()

const { show: toast } = useToast()
const latencies = ref<number[]>([])
const statuses = ref<number[]>([])
const startTime = ref('')
const intervalMin = ref(5)
const stats = ref<LatencyStats | null>(null)
const loading = ref(true)

const hasData = computed(() => statuses.value.some(s => s !== -1))

const statusLabel: Record<string, string> = { operational: '正常', degraded: '性能下降', outage: '故障' }

const statusClass = (s: string) => {
  switch (s) {
    case 'operational': return 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15 dark:border-emerald-800'
    case 'degraded': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    case 'outage': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    default: return ''
  }
}

async function loadLatency() {
  const hash = props.service.publicHash
  if (!hash) {
    latencies.value = []
    statuses.value = []
    stats.value = null
    loading.value = false
    return
  }
  loading.value = true
  stats.value = null
  try {
    const res = await api.getServiceLatency(hash, 1)
    latencies.value = res.data.latencies
    statuses.value = res.data.statuses
    startTime.value = res.data.start
    intervalMin.value = res.data.interval
    stats.value = res.data.stats
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadLatency)

watch(() => props.service.publicHash, () => {
  loadLatency()
})
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <button @click="emit('back')" class="flex items-center gap-1.5 text-sm transition-colors hover:opacity-100" style="color: var(--text-color); opacity: 0.5;">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
        返回列表
      </button>
    </div>

    <!-- Info card -->
    <div class="rounded-xl p-5 mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="flex items-start justify-between mb-4">
        <div>
          <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ service.name }}</h2>
          <div class="text-sm mt-1" style="color: var(--text-color); opacity: 0.5;">{{ service.url }}</div>
        </div>
        <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium border', statusClass(service.status)]" :style="!['operational','degraded','outage'].includes(service.status) ? 'color: var(--text-color); opacity: 0.6; background-color: var(--bg-color); border-color: var(--button-border-color);' : ''">
          <span :class="['w-1.5 h-1.5 rounded-full',
            service.status === 'operational' ? 'bg-emerald-500' : service.status === 'degraded' ? 'bg-yellow-400' : 'bg-red-500'
          ]" />
          {{ statusLabel[service.status] || service.status }}
        </span>
      </div>
      <div v-if="service.description" class="text-sm mb-4" style="color: var(--text-color); opacity: 0.6;">{{ service.description }}</div>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">类型</div>
          <div class="font-medium" style="color: var(--text-color);">{{ { http: 'HTTP', tcp: 'TCP', ping: 'Ping' }[service.type] || service.type }}</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">在线率</div>
          <div class="font-medium" :class="service.uptime >= 99.9 ? 'text-emerald-600' : ''" :style="service.uptime >= 99.9 ? '' : 'color: var(--text-color);'">{{ service.uptime.toFixed(2) }}%</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">当前延迟</div>
          <div class="font-medium" style="color: var(--text-color);">{{ service.latency }}ms</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">探测间隔</div>
          <div class="font-medium" style="color: var(--text-color);">{{ service.interval }}s</div>
        </div>
      </div>
    </div>

    <!-- Latency chart -->
    <div class="rounded-xl p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
        <h3 class="font-bold" style="color: var(--text-color);">最近24小时延迟</h3>
        <span v-if="stats" class="text-xs" style="color: var(--text-color); opacity: 0.45;">
          共 {{ stats.samples }} 个成功样本
        </span>
      </div>
      <!-- 分位数摘要：均值会被尖峰平均掉，p95/p99 用来暴露长尾 -->
      <div v-if="stats" class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm mb-5">
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">平均延迟</div>
          <div class="font-medium" style="color: var(--text-color);">{{ Math.round(stats.avg) }}ms</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">P95</div>
          <div class="font-medium" style="color: var(--text-color);">{{ Math.round(stats.p95) }}ms</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">P99</div>
          <div class="font-medium" style="color: var(--text-color);">{{ Math.round(stats.p99) }}ms</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">峰值</div>
          <div class="font-medium" style="color: var(--text-color);">{{ stats.max }}ms</div>
        </div>
      </div>
      <div v-if="loading" class="text-center py-12 " style="color: var(--text-color); opacity: 0.4;">加载中...</div>
      <div v-else-if="!hasData" class="text-center py-12 " style="color: var(--text-color); opacity: 0.4;">暂无数据</div>
      <LatencyChart
        v-else
        :latencies="latencies"
        :statuses="statuses"
        :start-time="startTime"
        :interval-min="intervalMin"
        ring-color="var(--bg-color)"
      />
    </div>
  </div>
</template>
