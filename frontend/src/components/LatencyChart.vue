<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  latencies: number[]
  statuses: number[]
  startTime: string
  intervalMin: number
  /** 悬浮标记的描边色，需与所在卡片背景保持一致 */
  ringColor?: string
}>()

const hoverIndex = ref(-1)
const pointCount = computed(() => props.latencies.length)

// ---- SVG 图表常量 ----
const chartW = 800
const chartH = 250
const padL = 54
const padR = 24
const padT = 24
const padB = 42
const plotW = chartW - padL - padR
const plotH = chartH - padT - padB
const plotBottom = padT + plotH

// 每个组件实例独立的渐变 ID，避免同一页面多个图表互相覆盖
const uid = `lat-${Math.random().toString(36).slice(2, 8)}`

// 状态：-1=无数据，0=正常，1=故障
const LINE_COLORS: Record<number, string> = { 0: '#34a761', 1: '#df2d2a', '-1': '#9ca3af' }
const STATUS_LABELS: Record<number, string> = { 0: '正常', 1: '故障', '-1': '无数据' }
const GRADIENT_STOPS: { status: number; color: string }[] = [
  { status: 0, color: '#34a761' },
  { status: 1, color: '#df2d2a' },
]

interface Pt { x: number; y: number }

/** 取某个时间点在上海时区下的时/分 */
function cstParts(d: Date): { hour: number; minute: number } {
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: 'Asia/Shanghai', hour: '2-digit', minute: '2-digit', hour12: false,
  }).formatToParts(d)
  const hour = Number(parts.find(p => p.type === 'hour')?.value || '0') % 24
  const minute = Number(parts.find(p => p.type === 'minute')?.value || '0')
  return { hour, minute }
}

function timeAt(i: number): Date {
  const base = new Date(props.startTime)
  if (Number.isNaN(base.getTime())) return new Date()
  return new Date(base.getTime() + i * props.intervalMin * 60000)
}

function labelAt(i: number): string {
  const { hour, minute } = cstParts(timeAt(i))
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`
}

/** 把坐标轴上限收敛到更美观的刻度值 */
function niceMax(v: number): number {
  if (!isFinite(v) || v <= 0) return 10
  const exp = Math.floor(Math.log10(v))
  const base = Math.pow(10, exp)
  const n = v / base
  const step = n <= 1 ? 1 : n <= 2 ? 2 : n <= 2.5 ? 2.5 : n <= 5 ? 5 : 10
  return step * base
}

const chartData = computed(() => {
  const n = pointCount.value
  if (n === 0) return null

  // 只用有数据的点计算 Y 轴范围，并留出约 15% 顶部留白
  const valid = props.latencies.filter((_, i) => props.statuses[i] !== -1)
  const rawMax = valid.length ? Math.max(...valid) : 0
  const minLat = 0
  const maxLat = Math.max(niceMax(rawMax * 1.15), 10)
  const range = maxLat - minLat || 1

  const toX = (i: number) => padL + (n === 1 ? plotW / 2 : (i / (n - 1)) * plotW)
  const toY = (val: number) => plotBottom - ((val - minLat) / range) * plotH
  // 控制点做纵向夹紧，避免样条插值在尖峰处冲出绘图区
  const clampY = (y: number) => Math.max(padT - 4, Math.min(plotBottom, y))

  // 连续有效点组成一个 run；run 内部再按状态切成不同颜色的折线段
  const runs: number[][] = []
  let cur: number[] = []
  for (let i = 0; i < n; i++) {
    if (props.statuses[i] === -1) {
      if (cur.length) runs.push(cur)
      cur = []
    } else {
      cur.push(i)
    }
  }
  if (cur.length) runs.push(cur)

  const segments: { key: string; linePath: string; areaPath: string; color: string; status: number }[] = []
  const dots: { key: string; x: number; y: number; color: string }[] = []

  for (let r = 0; r < runs.length; r++) {
    const run = runs[r]
    const P: Pt[] = run.map(i => ({ x: toX(i), y: toY(props.latencies[i]) }))

    if (run.length === 1) {
      const st = props.statuses[run[0]]
      dots.push({ key: `d${r}`, x: P[0].x, y: P[0].y, color: LINE_COLORS[st] || '#9ca3af' })
      continue
    }

    // Catmull-Rom 转三次贝塞尔；控制点按整个 run 计算，
    // 这样在颜色切换的位置曲线切线依然连续，不会出现折角
    const k = (1 / 6) * 0.85
    const ctrl: { c1: Pt; c2: Pt }[] = []
    for (let i = 0; i < P.length - 1; i++) {
      const p0 = P[i - 1] ?? P[i]
      const p1 = P[i]
      const p2 = P[i + 1]
      const p3 = P[i + 2] ?? P[i + 1]
      ctrl.push({
        c1: { x: p1.x + (p2.x - p0.x) * k, y: clampY(p1.y + (p2.y - p0.y) * k) },
        c2: { x: p2.x - (p3.x - p1.x) * k, y: clampY(p2.y - (p3.y - p1.y) * k) },
      })
    }

    let a = 0
    while (a < run.length) {
      let b = a
      const st = props.statuses[run[a]]
      while (b + 1 < run.length && props.statuses[run[b + 1]] === st) b++

      const f = (v: number) => v.toFixed(1)
      let d = ''
      let areaStartX: number

      if (a > 0) {
        // 与前一段的交界处取贝塞尔中点，保证相邻颜色段无缝衔接
        const A = P[a - 1], B = P[a], C1 = ctrl[a - 1].c1, C2 = ctrl[a - 1].c2
        const mid = {
          x: (A.x + 3 * C1.x + 3 * C2.x + B.x) / 8,
          y: (A.y + 3 * C1.y + 3 * C2.y + B.y) / 8,
        }
        const q1 = { x: (C1.x + 2 * C2.x + B.x) / 4, y: (C1.y + 2 * C2.y + B.y) / 4 }
        const q2 = { x: (C2.x + B.x) / 2, y: (C2.y + B.y) / 2 }
        d = `M${f(mid.x)},${f(mid.y)} C${f(q1.x)},${f(q1.y)} ${f(q2.x)},${f(q2.y)} ${f(B.x)},${f(B.y)}`
        areaStartX = mid.x
      } else {
        d = `M${f(P[a].x)},${f(P[a].y)}`
        areaStartX = P[a].x
      }

      for (let i = a; i < b; i++) {
        const C1 = ctrl[i].c1, C2 = ctrl[i].c2, B = P[i + 1]
        d += ` C${f(C1.x)},${f(C1.y)} ${f(C2.x)},${f(C2.y)} ${f(B.x)},${f(B.y)}`
      }

      let areaEndX = P[b].x
      if (b < run.length - 1) {
        // 与后一段的交界处同样取中点，路径只画到一半
        const A = P[b], B = P[b + 1], C1 = ctrl[b].c1, C2 = ctrl[b].c2
        const mid = {
          x: (A.x + 3 * C1.x + 3 * C2.x + B.x) / 8,
          y: (A.y + 3 * C1.y + 3 * C2.y + B.y) / 8,
        }
        const q1 = { x: (A.x + C1.x) / 2, y: (A.y + C1.y) / 2 }
        const q2 = { x: (A.x + 2 * C1.x + C2.x) / 4, y: (A.y + 2 * C1.y + C2.y) / 4 }
        d += ` C${f(q1.x)},${f(q1.y)} ${f(q2.x)},${f(q2.y)} ${f(mid.x)},${f(mid.y)}`
        areaEndX = mid.x
      }

      const areaPath = `${d} L${f(areaEndX)},${f(plotBottom)} L${f(areaStartX)},${f(plotBottom)} Z`
      segments.push({ key: `s${r}-${a}`, linePath: d, areaPath, color: LINE_COLORS[st] || '#9ca3af', status: st })
      a = b + 1
    }
  }

  // Y 轴：0 → maxLat 均分 4 段
  const yTicks: { y: number; label: string }[] = []
  for (let i = 0; i <= 4; i++) {
    const val = minLat + (range * i) / 4
    yTicks.push({ y: toY(val), label: `${Math.round(val)}ms` })
  }

  // X 轴：优先对齐到整点（1 天数据按 4 小时一个刻度），保证标签美观
  const strideHours = n > 200 ? 4 : n > 96 ? 2 : 1
  let picked: number[] = []
  for (let i = 0; i < n; i++) {
    const { hour, minute } = cstParts(timeAt(i))
    if (minute === 0 && hour % strideHours === 0) picked.push(i)
  }
  if (picked.length > 7) {
    const skip = Math.ceil(picked.length / 7)
    picked = picked.filter((_, idx) => idx % skip === 0)
  }
  if (picked.length < 2) {
    const step = Math.max(1, Math.floor(n / 6))
    picked = []
    for (let i = 0; i < n; i += step) picked.push(i)
  }
  const xTicks = picked.map(i => {
    const x = toX(i)
    const anchor = x < padL + 18 ? 'start' : x > chartW - padR - 18 ? 'end' : 'middle'
    return { x, label: labelAt(i), anchor }
  })

  return { segments, dots, toX, toY, yTicks, xTicks, maxLat, minLat, range }
})

const hover = computed(() => {
  const i = hoverIndex.value
  const data = chartData.value
  if (!data || i < 0 || i >= pointCount.value) return null

  const st = props.statuses[i]
  const hasData = st !== -1
  const x = data.toX(i)
  const y = hasData ? data.toY(props.latencies[i]) : padT + plotH / 2
  const color = LINE_COLORS[st] || '#9ca3af'

  const boxW = 108
  const boxH = 46
  const gap = 14
  const tx = Math.max(padL + boxW / 2, Math.min(chartW - padR - boxW / 2, x))
  let bottom = y - gap
  let above = true
  if (bottom - boxH < padT - 4) {
    above = false
    bottom = y + gap + boxH
  }

  return {
    x, y, hasData, color,
    time: labelAt(i),
    label: hasData ? `${props.latencies[i]}ms` : '无数据',
    statusLabel: STATUS_LABELS[st] || '',
    box: { x: tx - boxW / 2, y: bottom - boxH, w: boxW, h: boxH },
    stemY: above ? bottom : bottom - boxH,
  }
})

function onMouseMove(e: MouseEvent) {
  const n = pointCount.value
  if (!chartData.value || n === 0) return
  const svg = e.currentTarget as SVGElement
  const rect = svg.getBoundingClientRect()
  if (rect.width === 0) return
  const scaleX = chartW / rect.width
  const mouseX = (e.clientX - rect.left) * scaleX
  if (mouseX < padL - 12 || mouseX > chartW - padR + 12) {
    hoverIndex.value = -1
    return
  }
  const idx = n === 1 ? 0 : Math.round(((mouseX - padL) / plotW) * (n - 1))
  hoverIndex.value = Math.max(0, Math.min(idx, n - 1))
}
</script>

<template>
  <div>
    <div class="w-full" style="aspect-ratio: 800/250;">
      <svg
        :viewBox="`0 0 ${chartW} ${chartH}`"
        class="w-full h-full"
        preserveAspectRatio="xMidYMid meet"
        @mousemove="onMouseMove"
        @mouseleave="hoverIndex = -1"
      >
        <defs>
          <linearGradient v-for="g in GRADIENT_STOPS" :key="g.status" :id="`${uid}-grad-${g.status}`" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" :stop-color="g.color" stop-opacity="0.32" />
            <stop offset="60%" :stop-color="g.color" stop-opacity="0.10" />
            <stop offset="100%" :stop-color="g.color" stop-opacity="0.01" />
          </linearGradient>
        </defs>

        <!-- 横向网格 -->
        <line v-for="(tick, ti) in chartData!.yTicks" :key="'grid-' + ti"
          :x1="padL" :y1="tick.y" :x2="chartW - padR" :y2="tick.y"
          stroke="var(--button-border-color)" :stroke-opacity="ti === 0 ? 0.9 : 0.5" stroke-width="1"
        />
        <!-- Y 轴标签 -->
        <text v-for="(tick, ti) in chartData!.yTicks" :key="'yl-' + ti"
          :x="padL - 10" :y="tick.y + 3.5"
          text-anchor="end" fill="var(--text-color)" fill-opacity="0.42" style="font-size: 10px;"
        >{{ tick.label }}</text>
        <!-- X 轴标签 -->
        <text v-for="(tick, ti) in chartData!.xTicks" :key="'xl-' + ti"
          :x="tick.x" :y="chartH - 12" :text-anchor="tick.anchor"
          fill="var(--text-color)" fill-opacity="0.42" style="font-size: 10px;"
        >{{ tick.label }}</text>

        <!-- 渐变面积 -->
        <path v-for="seg in chartData!.segments" :key="'area-' + seg.key"
          :d="seg.areaPath" :fill="`url(#${uid}-grad-${seg.status})`"
        />

        <!-- 平滑折线 -->
        <path v-for="seg in chartData!.segments" :key="'line-' + seg.key"
          :d="seg.linePath" fill="none" :stroke="seg.color"
          stroke-width="2" stroke-linejoin="round" stroke-linecap="round"
        />

        <!-- 孤立数据点 -->
        <circle v-for="dot in chartData!.dots" :key="'dot-' + dot.key"
          :cx="dot.x" :cy="dot.y" r="3" :fill="dot.color"
        />

        <!-- 悬浮指示 -->
        <template v-if="hover">
          <line
            :x1="hover.x" :y1="padT" :x2="hover.x" :y2="plotBottom"
            :stroke="hover.color" stroke-opacity="0.55" stroke-width="1" stroke-dasharray="4,4"
          />
          <line
            :x1="hover.x" :y1="hover.stemY" :x2="hover.x" :y2="hover.y"
            :stroke="hover.color" stroke-opacity="0.3" stroke-width="1"
          />
          <circle v-if="hover.hasData" :cx="hover.x" :cy="hover.y" r="9" :fill="hover.color" fill-opacity="0.16" />
          <circle
            :cx="hover.x" :cy="hover.y" :r="hover.hasData ? 4.5 : 3.5"
            :fill="hover.hasData ? hover.color : 'none'"
            :stroke="hover.hasData ? (ringColor || 'var(--bg-color)') : hover.color"
            stroke-width="2"
          />
          <g :transform="`translate(${hover.box.x}, ${hover.box.y})`" style="pointer-events: none;">
            <rect :width="hover.box.w" :height="hover.box.h" rx="8" class="chart-tip" />
            <circle cx="15" cy="16" r="3.5" :fill="hover.color" />
            <text x="26" y="19.5" class="chart-tip-time" style="font-size: 10.5px;">{{ hover.time }}</text>
            <text x="15" y="38" class="chart-tip-value" :fill="hover.color" style="font-size: 12px; font-weight: 600;">{{ hover.label }}</text>
            <text :x="hover.box.w - 15" y="38" text-anchor="end" class="chart-tip-sub" style="font-size: 10px;">{{ hover.statusLabel }}</text>
          </g>
        </template>
      </svg>
    </div>

    <div class="flex items-center justify-center gap-4 mt-3 text-xs" style="color: var(--text-color); opacity: 0.45;">
      <span class="flex items-center gap-1.5"><span class="inline-block w-2.5 h-1 rounded-full bg-[#34a761]"></span>正常</span>
      <span class="flex items-center gap-1.5"><span class="inline-block w-2.5 h-1 rounded-full bg-[#df2d2a]"></span>故障</span>
      <span class="flex items-center gap-1.5"><span class="inline-block w-2.5 h-1 rounded-full bg-gray-400"></span>无数据</span>
    </div>
  </div>
</template>

<style scoped>
.chart-tip {
  fill: var(--bg-color);
  stroke: var(--button-border-color);
  stroke-width: 1;
  filter: drop-shadow(0 4px 12px rgba(0, 0, 0, 0.16));
}
.chart-tip-time {
  fill: var(--text-color);
  opacity: 0.6;
}
.chart-tip-value {
  font-variant-numeric: tabular-nums;
}
.chart-tip-sub {
  fill: var(--text-color);
  opacity: 0.45;
}
</style>
