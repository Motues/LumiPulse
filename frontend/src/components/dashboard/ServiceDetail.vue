<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { api } from '../../api/client'
import type { ServiceDetail as ServiceDetailType } from '../../api/types'
import { useToast } from '../../composables/useToast'

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
const loading = ref(true)
const hoverIndex = ref(-1)

const pointCount = computed(() => latencies.value.length)

const statusLabel: Record<string, string> = { operational: '正常', degraded: '性能下降', outage: '故障' }

const statusClass = (s: string) => {
  switch (s) {
    case 'operational': return 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-900/30 dark:border-emerald-800'
    case 'degraded': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    case 'outage': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    default: return 'text-gray-600 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700'
  }
}

function timeAtIndex(i: number): string {
  const t = new Date(startTime.value)
  t.setMinutes(t.getMinutes() + i * intervalMin.value)
  return t.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
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
  const n = pointCount.value
  if (n === 0) return null

  // 只用有数据的点计算Y轴范围
  let maxLat = 1, minLat = 0
  const validLatencies = latencies.value.filter((_, i) => statuses.value[i] !== -1)
  if (validLatencies.length > 0) {
    maxLat = Math.max(...validLatencies, 1)
    minLat = Math.min(...validLatencies, 0)
  }
  const range = maxLat - minLat || 1

  const toX = (i: number) => padL + (n === 1 ? plotW / 2 : (i / (n - 1)) * plotW)
  const toY = (val: number) => padT + plotH - ((val - minLat) / range) * plotH

  // 按状态分段
  const statusColors: Record<number, string> = { 0: '#34a761', 1: '#df2d2a', '-1': '#9ca3af' }
  const segments: { linePath: string; areaPath: string; color: string; status: number }[] = []
  let segStart = 0
  for (let i = 1; i <= n; i++) {
    if (i === n || statuses.value[i] !== statuses.value[segStart]) {
      const len = i - segStart
      if (len > 0) {
        const lp = Array.from({ length: len }, (_, j) =>
          `${j === 0 ? 'M' : 'L'}${toX(segStart + j).toFixed(1)},${toY(latencies.value[segStart + j]).toFixed(1)}`
        ).join(' ')
        const ap = lp + ` L${toX(i - 1).toFixed(1)},${(padT + plotH).toFixed(1)} L${toX(segStart).toFixed(1)},${(padT + plotH).toFixed(1)} Z`
        segments.push({ linePath: lp, areaPath: ap, color: statusColors[statuses.value[segStart]] || '#9ca3af', status: statuses.value[segStart] })
      }
      segStart = i
    }
  }

  // Y axis labels (4 ticks)
  const yTicks: { y: number; label: string }[] = []
  for (let i = 0; i <= 4; i++) {
    const val = minLat + (range * i) / 4
    yTicks.push({ y: toY(val), label: Math.round(val) + 'ms' })
  }

  // X axis labels (max 8 ticks)
  const step = Math.max(1, Math.floor(n / 8))
  const xTicks: { x: number; label: string }[] = []
  for (let i = 0; i < n; i += step) {
    xTicks.push({ x: toX(i), label: timeAtIndex(i) })
  }
  if (n > 1 && xTicks[xTicks.length - 1]?.x !== toX(n - 1)) {
    xTicks.push({ x: toX(n - 1), label: timeAtIndex(n - 1) })
  }

  return { segments, toX, toY, yTicks, xTicks, maxLat, minLat, range }
})

function onMouseMove(e: MouseEvent) {
  const n = pointCount.value
  if (!chartData.value || n === 0) return
  const svg = (e.currentTarget as SVGElement)
  const rect = svg.getBoundingClientRect()
  const scaleX = chartW / rect.width
  const mouseX = (e.clientX - rect.left) * scaleX
  const idx = Math.round(((mouseX - padL) / plotW) * (n - 1))
  hoverIndex.value = Math.max(0, Math.min(idx, n - 1))
}

async function loadLatency() {
  loading.value = true
  try {
    const res = await api.getServiceLatency(props.service.id, 1)
    latencies.value = res.data.latencies
    statuses.value = res.data.statuses
    startTime.value = res.data.start
    intervalMin.value = res.data.interval
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
      <div v-else-if="statuses.every(s => s === -1)" class="text-center py-12 text-gray-400 dark:text-gray-500">暂无数据</div>
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
            text-anchor="end" class="fill-gray-400 dark:fill-gray-500" style="font-size: 9px;"
          >{{ tick.label }}</text>

          <!-- X axis labels -->
          <text v-for="tick in chartData!.xTicks" :key="'xl-' + tick.label"
            :x="tick.x" :y="chartH - 8"
            text-anchor="middle" class="fill-gray-400 dark:fill-gray-500" style="font-size: 9px;"
          >{{ tick.label }}</text>

          <!-- Colored area fills -->
          <path v-for="(seg, si) in chartData!.segments" :key="'sa-' + si"
            :d="seg.areaPath" :fill="seg.color" fill-opacity="0.08"
          />

          <!-- Colored lines -->
          <path v-for="(seg, si) in chartData!.segments" :key="'sl-' + si"
            :d="seg.linePath" fill="none" :stroke="seg.color"
            stroke-width="2" stroke-linejoin="round" stroke-linecap="round"
          />

          <!-- Hover crosshair -->
          <template v-if="hoverIndex >= 0 && hoverIndex < pointCount">
            <line
              :x1="chartData!.toX(hoverIndex)" :y1="padT"
              :x2="chartData!.toX(hoverIndex)" :y2="padT + plotH"
              class="stroke-gray-400 dark:stroke-gray-500" stroke-width="1" stroke-dasharray="3,3"
            />
            <circle
              :cx="chartData!.toX(hoverIndex)" :cy="chartData!.toY(latencies[hoverIndex])"
              r="4" class="fill-emerald-500 stroke-white dark:stroke-gray-900" stroke-width="2"
            />
            <!-- Tooltip -->
            <rect
              :x="chartData!.toX(hoverIndex) - 35" :y="chartData!.toY(latencies[hoverIndex]) - 28"
              width="70" height="22" rx="4"
              class="fill-gray-900 dark:fill-gray-100"
            />
            <text
              :x="chartData!.toX(hoverIndex)" :y="chartData!.toY(latencies[hoverIndex]) - 13"
              text-anchor="middle" class="fill-white dark:fill-gray-900" style="font-size: 9px; font-weight: 500;"
            >{{ statuses[hoverIndex] === -1 ? '无数据' : latencies[hoverIndex] + 'ms' }}</text>
          </template>
        </svg>
      </div>
      <div class="flex items-center justify-center gap-4 mt-2 text-xs text-gray-400 dark:text-gray-500">
        <span class="flex items-center gap-1"><span class="inline-block w-2 h-2 rounded-full bg-[#34a761]"></span>正常</span>
        <span class="flex items-center gap-1"><span class="inline-block w-2 h-2 rounded-full bg-[#df2d2a]"></span>故障</span>
        <span class="flex items-center gap-1"><span class="inline-block w-2 h-2 rounded-full bg-gray-400"></span>无数据</span>
      </div>
    </div>
  </div>
</template>
