<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { api } from '../api/client'
import type { ServiceSummary, LatencyStats } from '../api/types'
import ServiceMatrix from './ServiceMatrix.vue'
import LatencyChart from './LatencyChart.vue'
import LatencyHeatmap from './LatencyHeatmap.vue'
import { useI18n } from '../composables/useI18n'
import { certLevel, certRemainingDays } from '../utils/cert'
import { parseHomepageBlocks } from '../utils/homepageBlocks'

const props = defineProps<{
  service: ServiceSummary
  dailyDays: [number, number, number, number][]
}>()

const emit = defineEmits<{
  back: []
}>()

const { t, formatDateTime } = useI18n()

/**
 * 展示内容块由管理后台按服务配置（见 utils/homepageBlocks.ts）：
 * 未配置 = 全部展示，管理后台自身始终展示全部内容。
 */
const blocks = computed(() => parseHomepageBlocks(props.service.homepageBlocks))
const showMetrics = computed(() => blocks.value.has('metrics'))
const showInterval = computed(() => blocks.value.has('interval'))
const showCert = computed(() => blocks.value.has('cert'))
const showLatency = computed(() => blocks.value.has('latency'))
const showHeatmap = computed(() => blocks.value.has('heatmap'))
const showHistory = computed(() => blocks.value.has('history'))

const latencies = ref<number[]>([])
const statuses = ref<number[]>([])
const startTime = ref('')
const intervalMin = ref(5)
const stats = ref<LatencyStats | null>(null)
const loading = ref(true)

function statusText(status: string): string {
  switch (status) {
    case 'operational': return t('status.operational')
    case 'degraded': return t('status.degraded')
    case 'outage': return t('status.outage')
    default: return status
  }
}

const statusColors: Record<string, string> = {
  operational: '#34a761',
  degraded: '#fda305',
  outage: '#df2d2a',
}

const hasData = computed(() => statuses.value.some(s => s !== -1))

/** HTTPS 证书到期信息：纯 HTTP 服务没有该字段，整块不展示 */
const cert = computed(() => {
  const iso = props.service.certExpiresAt
  const level = certLevel(iso)
  if (!iso || !level) return null
  const days = certRemainingDays(iso)
  return {
    level,
    expiresAt: formatDateTime(iso),
    remaining: days !== null && days < 0 ? t('service.certExpired') : t('service.certRemaining', { n: Math.max(0, Math.floor(days ?? 0)) }),
  }
})

const certColorClass: Record<string, string> = {
  ok: '',
  warn: 'text-[#f9ac05] dark:text-[#fbbf24]',
  critical: 'text-[#df2d2a] dark:text-[#f87171]',
}

/** 图表概览：故障桶数量（维护窗口内的故障计入其中）。
 *  延迟统计（平均 / P95 / P99 / 峰值）由后端在窗口内计算。 */
const chartStats = computed(() => {
  const vals = latencies.value.filter((_, i) => statuses.value[i] !== -1)
  if (vals.length === 0) return null
  return {
    failures: statuses.value.filter(s => s === 1 || s === 2).length,
  }
})

async function loadLatency() {
  // 关闭延迟曲线时不请求数据，避免无谓的接口往返
  if (!showLatency.value) {
    latencies.value = []
    statuses.value = []
    stats.value = null
    loading.value = false
    return
  }
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
  } catch {
    latencies.value = []
    statuses.value = []
    stats.value = null
  } finally {
    loading.value = false
  }
}

onMounted(loadLatency)
// 公开接口按 hash 定位，服务对象被刷新替换时重新拉取；
// 展示开关变化时同样要重新判断是否需要请求
watch(() => [props.service.publicHash, props.service.homepageBlocks], loadLatency)
</script>

<template>
  <div>
    <div class="flex items-center mb-4">
      <button @click="emit('back')" class="flex items-center gap-1.5 text-sm transition-colors text-[color:var(--text-color)] opacity-50 hover:opacity-80">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
        {{ t('common.backToStatus') }}
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
          {{ statusText(service.status) }}
        </div>
      </div>
      <!-- 指标区：在线率 / 响应时间、探测频率、证书到期均可在管理后台按服务关闭 -->
      <div v-if="showMetrics || showInterval || showCert" class="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
        <div v-if="showMetrics">
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('service.uptime') }}</div>
          <div class="font-medium" :class="service.uptime >= 99.9 ? 'text-emerald-600 dark:text-emerald-400' : ''" :style="service.uptime < 99.9 ? 'color: var(--text-color);' : ''">{{ service.uptime.toFixed(2) }}%</div>
        </div>
        <div v-if="showMetrics">
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('service.responseTime') }}</div>
          <div class="font-medium" style="color: var(--text-color);">{{ service.latency }}ms</div>
        </div>
        <div v-if="showInterval">
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('service.probeInterval') }}</div>
          <div class="font-medium" style="color: var(--text-color);">{{ service.interval }}s</div>
        </div>
        <div v-if="showCert && cert">
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('service.certExpiry') }}</div>
          <div class="font-medium" :class="certColorClass[cert.level]" :style="cert.level === 'ok' ? 'color: var(--text-color);' : ''">
            {{ cert.expiresAt }}
            <span class="text-xs opacity-60">· {{ cert.remaining }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Latency chart -->
    <div v-if="showLatency" class="rounded-lg p-6 mb-6" style="border: 1px solid var(--button-border-color);">
      <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
        <h3 class="font-bold" style="color: var(--text-color);">{{ t('service.latency24h') }}</h3>
        <div v-if="chartStats" class="flex items-center gap-3 text-xs" style="color: var(--text-color); opacity: 0.55;">
          <span v-if="chartStats.failures > 0" class="text-[#df2d2a] dark:text-[#f87171]">{{ t('service.failures', { n: chartStats.failures }) }}</span>
          <span v-if="stats">{{ t('service.latencySamples', { n: stats.samples }) }}</span>
        </div>
      </div>
      <!-- 分位数摘要：均值会被尖峰平均掉，p95/p99 用来暴露长尾 -->
      <div v-if="stats" class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm mb-5">
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('service.avgLatency') }}</div>
          <div class="font-medium" style="color: var(--text-color);">{{ Math.round(stats.avg) }}ms</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('service.p95') }}</div>
          <div class="font-medium" style="color: var(--text-color);">{{ Math.round(stats.p95) }}ms</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('service.p99') }}</div>
          <div class="font-medium" style="color: var(--text-color);">{{ Math.round(stats.p99) }}ms</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">{{ t('service.peak') }}</div>
          <div class="font-medium" style="color: var(--text-color);">{{ stats.max }}ms</div>
        </div>
      </div>
      <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>
      <div v-else-if="!hasData" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('service.noData') }}</div>
      <LatencyChart
        v-else
        :latencies="latencies"
        :statuses="statuses"
        :start-time="startTime"
        :interval-min="intervalMin"
        ring-color="var(--bg-color)"
      />
    </div>

    <!-- Response time heatmap -->
    <div v-if="showHeatmap" class="rounded-lg p-6 mb-6" style="border: 1px solid var(--button-border-color);">
      <LatencyHeatmap :service-hash="service.publicHash" :days="7" />
    </div>

    <!-- Service history matrix -->
    <div v-if="showHistory" class="rounded-lg p-6" style="border: 1px solid var(--button-border-color);">
      <h3 class="font-bold mb-4" style="color: var(--text-color);">{{ t('service.history') }}</h3>
      <ServiceMatrix :days="dailyDays" :uptime="service.uptime" />
    </div>
  </div>
</template>
