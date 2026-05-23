<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { api } from '../../api/client'
import type { ServiceDetail as ServiceDetailType } from '../../api/types'
import { useToast } from '../../composables/useToast'

interface LatencyPoint {
  latency: number
  createdAt: string
}

const props = defineProps<{
  service: ServiceDetailType
}>()

const emit = defineEmits<{
  back: []
}>()

const { show: toast } = useToast()
const points = ref<LatencyPoint[]>([])
const loading = ref(true)
const hoverIndex = ref(-1)

const statusLabel: Record<string, string> = { operational: '正常', degraded: '性能下降', outage: '故障' }

const statusClass = (s: string) => {
  switch (s) {
    case 'operational': return 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-900/30 dark:border-emerald-800'
    case 'degraded': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    case 'outage': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    default: return 'text-gray-600 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700'
  }
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

// SVG chart constants
const chartW = 800
const chartH = 250
const padL = 50
const padR = 20
const padT = 20
const padB = 40
const plotW = chartW - padL - padR
const plotH = chartH - padT - padB

const chartData = computed(() => {
  if (points.value.length === 0) return null

  const data = points.value
  const latencies = data.map(p => p.latency)
  const maxLat = Math.max(...latencies, 1)
  const minLat = Math.min(...latencies, 0)
  const range = maxLat - minLat || 1

  const toX = (i: number) => padL + (data.length === 1 ? plotW / 2 : (i / (data.length - 1)) * plotW)
  const toY = (val: number) => padT + plotH - ((val - minLat) / range) * plotH

  const linePath = data.map((p, i) => `${i === 0 ? 'M' : 'L'}${toX(i).toFixed(1)},${toY(p.latency).toFixed(1)}`).join(' ')
  const areaPath = linePath + ` L${toX(data.length - 1).toFixed(1)},${(padT + plotH).toFixed(1)} L${toX(0).toFixed(1)},${(padT + plotH).toFixed(1)} Z`

  // Y axis labels (4 ticks)
  const yTicks: { y: number; label: string }[] = []
  for (let i = 0; i <= 4; i++) {
    const val = minLat + (range * i) / 4
    yTicks.push({ y: toY(val), label: Math.round(val) + 'ms' })
  }

  // X axis labels (max 8 ticks)
  const step = Math.max(1, Math.floor(data.length / 8))
  const xTicks: { x: number; label: string }[] = []
  for (let i = 0; i < data.length; i += step) {
    xTicks.push({ x: toX(i), label: formatTime(data[i].createdAt) })
  }
  // Always show last
  if (data.length > 1 && xTicks[xTicks.length - 1]?.x !== toX(data.length - 1)) {
    xTicks.push({ x: toX(data.length - 1), label: formatTime(data[data.length - 1].createdAt) })
  }

  return { linePath, areaPath, toX, toY, yTicks, xTicks, maxLat, minLat, range }
})

function onMouseMove(e: MouseEvent) {
  if (!chartData.value || points.value.length === 0) return
  const svg = (e.currentTarget as SVGElement)
  const rect = svg.getBoundingClientRect()
  const scaleX = chartW / rect.width
  const mouseX = (e.clientX - rect.left) * scaleX
  const idx = Math.round(((mouseX - padL) / plotW) * (points.value.length - 1))
  hoverIndex.value = Math.max(0, Math.min(idx, points.value.length - 1))
}

async function loadLatency() {
  loading.value = true
  try {
    const res = await api.getServiceLatency(props.service.id, 1)
    points.value = res.data
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadLatency)

watch(() => props.service.id, () => {
  loadLatency()
})
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <button @click="emit('back')" class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
        返回列表
      </button>
    </div>

    <!-- Info card -->
    <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 p-5 shadow-sm mb-6">
      <div class="flex items-start justify-between mb-4">
        <div>
          <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ service.name }}</h2>
          <div class="text-sm text-gray-500 dark:text-gray-400 mt-1">{{ service.url }}</div>
        </div>
        <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium border', statusClass(service.status)]">
          <span :class="['w-1.5 h-1.5 rounded-full',
            service.status === 'operational' ? 'bg-emerald-500' : service.status === 'degraded' ? 'bg-yellow-400' : 'bg-red-500'
          ]" />
          {{ statusLabel[service.status] || service.status }}
        </span>
      </div>
      <div v-if="service.description" class="text-sm text-gray-600 dark:text-gray-400 mb-4">{{ service.description }}</div>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <div class="text-gray-400 dark:text-gray-500 mb-1">类型</div>
          <div class="text-gray-900 dark:text-gray-100 font-medium">{{ { http: 'HTTP', tcp: 'TCP', ping: 'Ping' }[service.type] || service.type }}</div>
        </div>
        <div>
          <div class="text-gray-400 dark:text-gray-500 mb-1">在线率</div>
          <div class="font-medium" :class="service.uptime >= 99.9 ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-900 dark:text-gray-100'">{{ service.uptime.toFixed(2) }}%</div>
        </div>
        <div>
          <div class="text-gray-400 dark:text-gray-500 mb-1">当前延迟</div>
          <div class="text-gray-900 dark:text-gray-100 font-medium">{{ service.latency }}ms</div>
        </div>
        <div>
          <div class="text-gray-400 dark:text-gray-500 mb-1">探测间隔</div>
          <div class="text-gray-900 dark:text-gray-100 font-medium">{{ service.interval }}s</div>
        </div>
      </div>
    </div>

    <!-- Latency chart -->
    <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 p-5 shadow-sm">
      <h3 class="font-bold text-gray-900 dark:text-gray-100 mb-4">最近24小时延迟</h3>
      <div v-if="loading" class="text-center py-12 text-gray-400 dark:text-gray-500">加载中...</div>
      <div v-else-if="points.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">暂无数据</div>
      <div v-else class="w-full" style="aspect-ratio: 800/250;">
        <svg
          :viewBox="`0 0 ${chartW} ${chartH}`"
          class="w-full h-full"
          preserveAspectRatio="xMidYMid meet"
          @mousemove="onMouseMove"
          @mouseleave="hoverIndex = -1"
        >
          <!-- Grid lines -->
          <line v-for="tick in chartData!.yTicks" :key="tick.label"
            :x1="padL" :y1="tick.y" :x2="chartW - padR" :y2="tick.y"
            class="stroke-gray-200 dark:stroke-gray-700" stroke-dasharray="4,4" stroke-width="1"
          />

          <!-- Y axis labels -->
          <text v-for="tick in chartData!.yTicks" :key="'yl-' + tick.label"
            :x="padL - 8" :y="tick.y + 4"
            text-anchor="end" class="fill-gray-400 dark:fill-gray-500" style="font-size: 11px;"
          >{{ tick.label }}</text>

          <!-- X axis labels -->
          <text v-for="tick in chartData!.xTicks" :key="'xl-' + tick.label"
            :x="tick.x" :y="chartH - 8"
            text-anchor="middle" class="fill-gray-400 dark:fill-gray-500" style="font-size: 11px;"
          >{{ tick.label }}</text>

          <!-- Area fill -->
          <path :d="chartData!.areaPath" class="fill-emerald-500/10" />

          <!-- Line -->
          <path :d="chartData!.linePath" fill="none" class="stroke-emerald-500" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />

          <!-- Hover crosshair -->
          <template v-if="hoverIndex >= 0 && hoverIndex < points.length">
            <line
              :x1="chartData!.toX(hoverIndex)" :y1="padT"
              :x2="chartData!.toX(hoverIndex)" :y2="padT + plotH"
              class="stroke-gray-400 dark:stroke-gray-500" stroke-width="1" stroke-dasharray="3,3"
            />
            <circle
              :cx="chartData!.toX(hoverIndex)" :cy="chartData!.toY(points[hoverIndex].latency)"
              r="4" class="fill-emerald-500 stroke-white dark:stroke-gray-900" stroke-width="2"
            />
            <!-- Tooltip -->
            <rect
              :x="chartData!.toX(hoverIndex) - 35" :y="chartData!.toY(points[hoverIndex].latency) - 28"
              width="70" height="22" rx="4"
              class="fill-gray-900 dark:fill-gray-100"
            />
            <text
              :x="chartData!.toX(hoverIndex)" :y="chartData!.toY(points[hoverIndex].latency) - 13"
              text-anchor="middle" class="fill-white dark:fill-gray-900" style="font-size: 11px; font-weight: 500;"
            >{{ points[hoverIndex].latency }}ms</text>
          </template>
        </svg>
      </div>
    </div>
  </div>
</template>
