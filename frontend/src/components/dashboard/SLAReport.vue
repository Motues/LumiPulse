<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { MonthlySLAReport, SLATrendPoint } from '../../api/types'
import { useToast } from '../../composables/useToast'
import { useI18n } from '../../composables/useI18n'
import DateTimePicker from './DateTimePicker.vue'

const { show: toast } = useToast()
const { t } = useI18n()

const report = ref<MonthlySLAReport | null>(null)
const trend = ref<SLATrendPoint[]>([])
const retainedDays = ref(90)
const loading = ref(true)
const trendLoading = ref(true)

/** 当前查看的月份（YYYY-MM）。只接受已存在的月份，避免浏览器时区带来的偏差。 */
function currentMonthKey(): string {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

const month = ref(currentMonthKey())

const isCurrentMonth = computed(() => month.value === currentMonthKey())

/** 月份偏移，用于上一月 / 下一月按钮 */
function shiftMonth(key: string, offset: number): string {
  const [y, m] = key.split('-').map(Number)
  const d = new Date(Date.UTC(y, m - 1 + offset, 1))
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`
}

async function loadReport() {
  loading.value = true
  try {
    const res = await api.getSLAReport(month.value)
    report.value = res.data
  } catch (e: any) {
    toast(e.message || t('admin.sla.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadTrend() {
  trendLoading.value = true
  try {
    const res = await api.getSLATrend(6)
    trend.value = res.data.months || []
    retainedDays.value = res.data.retainedDays || 90
  } catch {
    trend.value = []
  } finally {
    trendLoading.value = false
  }
}

function stepMonth(offset: number) {
  const next = shiftMonth(month.value, offset)
  if (next > currentMonthKey()) return
  month.value = next
  loadReport()
}

function onMonthChange() {
  if (!month.value) {
    month.value = currentMonthKey()
  }
  loadReport()
}

/** 可用率展示：低于 99.9% 标黄、低于 99% 标红，与其它页面口径一致 */
function uptimeClass(uptime: number, covered: number): string {
  if (covered === 0) return ''
  if (uptime >= 99.9) return 'text-emerald-600 dark:text-emerald-400'
  if (uptime >= 99) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return '—'
  if (seconds < 60) return t('admin.sla.seconds', { n: Math.round(seconds) })
  if (seconds < 3600) return t('admin.sla.minutes', { n: Math.round(seconds / 60) })
  const hours = seconds / 3600
  if (hours < 24) return t('admin.sla.hours', { n: hours.toFixed(1) })
  return t('admin.sla.days', { n: (hours / 24).toFixed(1) })
}

const trendMaxIncidents = computed(() => Math.max(1, ...trend.value.map(t => t.incidents)))

onMounted(async () => {
  await Promise.all([loadReport(), loadTrend()])
})
</script>

<template>
  <div>
    <!-- 月份切换 -->
    <div class="flex flex-wrap items-center justify-between gap-3 mb-5">
      <div class="flex items-center gap-2">
        <button
          @click="stepMonth(-1)"
          class="px-2.5 py-1.5 rounded-lg text-sm transition-colors btn-cancel"
          :title="t('admin.sla.prevMonth')"
        >&lsaquo;</button>
        <DateTimePicker
          v-model="month"
          mode="month"
          :max="currentMonthKey()"
          :placeholder="t('admin.sla.selectMonth')"
          class="w-[150px]"
          @update:model-value="onMonthChange"
        />
        <button
          @click="stepMonth(1)"
          :disabled="isCurrentMonth"
          class="px-2.5 py-1.5 rounded-lg text-sm transition-colors btn-cancel"
          :style="isCurrentMonth ? { opacity: 0.3, cursor: 'not-allowed' } : {}"
          :title="t('admin.sla.nextMonth')"
        >&rsaquo;</button>
      </div>
      <div class="text-xs" style="color: var(--text-color); opacity: 0.45;">
        {{ t('admin.sla.retainedHint', { days: retainedDays }) }}
      </div>
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <template v-else-if="report">
      <!-- 汇总卡片 -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-5 mb-6">
        <div class="rounded-xl p-4 md:p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.sla.overallUptime') }}</div>
          <div class="text-2xl md:text-3xl font-bold" :class="uptimeClass(report.summary.uptime, report.summary.totalProbes)">
            {{ report.summary.totalProbes > 0 ? report.summary.uptime.toFixed(3) + '%' : t('admin.sla.noData') }}
          </div>
          <div class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
            {{ t('admin.sla.totalProbes', { n: report.summary.totalProbes }) }}
          </div>
        </div>
        <div class="rounded-xl p-4 md:p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.sla.incidents') }}</div>
          <div class="text-2xl md:text-3xl font-bold" style="color: var(--text-color);">{{ report.summary.incidents }}</div>
          <div class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
            {{ t('admin.sla.failedProbes', { n: report.summary.downtimeProbes }) }}
          </div>
        </div>
        <div class="rounded-xl p-4 md:p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.sla.downtime') }}</div>
          <div class="text-2xl md:text-3xl font-bold" style="color: var(--text-color);">{{ formatDuration(report.summary.downtimeSeconds) }}</div>
          <div class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.sla.downtimeHint') }}</div>
        </div>
        <div class="rounded-xl p-4 md:p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
          <div class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.sla.avgResponse') }}</div>
          <div class="text-2xl md:text-3xl font-bold" style="color: var(--text-color);">
            {{ report.summary.totalProbes > 0 ? Math.round(report.summary.avgLatency) + 'ms' : '—' }}
          </div>
          <div class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">{{ report.label }}</div>
        </div>
      </div>

      <!-- 服务明细 -->
      <div class="rounded-xl mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <div class="flex items-center justify-between px-5 py-4" style="border-bottom: 1px solid var(--button-border-color);">
          <h3 class="font-bold" style="color: var(--text-color);">{{ t('admin.sla.serviceBreakdown') }}</h3>
          <span v-if="!report.final" class="text-xs px-2 py-0.5 rounded" style="color: var(--text-color); background-color: var(--button-hover-color);">
            {{ t('admin.sla.accumulating') }}
          </span>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-xs" style="color: var(--text-color); opacity: 0.45;">
                <th class="text-left font-medium px-5 py-3">{{ t('admin.sla.colService') }}</th>
                <th class="text-right font-medium px-5 py-3">{{ t('admin.sla.colUptime') }}</th>
                <th class="text-right font-medium px-5 py-3">{{ t('admin.sla.colProbes') }}</th>
                <th class="text-right font-medium px-5 py-3">{{ t('admin.sla.colFailures') }}</th>
                <th class="text-right font-medium px-5 py-3">{{ t('admin.sla.colIncidents') }}</th>
                <th class="text-right font-medium px-5 py-3">{{ t('admin.sla.colDowntime') }}</th>
                <th class="text-right font-medium px-5 py-3">{{ t('admin.sla.colAvgResponse') }}</th>
                <th class="text-right font-medium px-5 py-3">{{ t('admin.sla.colCoveredDays') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="svc in report.services"
                :key="svc.serviceId"
                style="border-top: 1px solid var(--button-border-color);"
              >
                <td class="px-5 py-3 font-medium" style="color: var(--text-color);">{{ svc.name }}</td>
                <td class="px-5 py-3 text-right font-medium" :class="uptimeClass(svc.uptime, svc.coveredDays)">
                  {{ svc.coveredDays > 0 ? svc.uptime.toFixed(3) + '%' : t('admin.sla.noData') }}
                </td>
                <td class="px-5 py-3 text-right" style="color: var(--text-color); opacity: 0.7;">{{ svc.totalProbes }}</td>
                <td class="px-5 py-3 text-right" style="color: var(--text-color); opacity: 0.7;">{{ svc.downtimeProbes }}</td>
                <td class="px-5 py-3 text-right" style="color: var(--text-color); opacity: 0.7;">{{ svc.incidents }}</td>
                <td class="px-5 py-3 text-right" style="color: var(--text-color); opacity: 0.7;">{{ formatDuration(svc.downtimeSeconds) }}</td>
                <td class="px-5 py-3 text-right" style="color: var(--text-color); opacity: 0.7;">
                  {{ svc.totalProbes > 0 ? Math.round(svc.avgLatency) + 'ms' : '—' }}
                </td>
                <td class="px-5 py-3 text-right text-xs" style="color: var(--text-color); opacity: 0.5;">
                  {{ svc.coveredDays }}/{{ report.days }}
                </td>
              </tr>
              <tr v-if="report.services.length === 0">
                <td colspan="8" class="px-5 py-8 text-center" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.sla.emptyMonth') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- 趋势 -->
      <div class="rounded-xl p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="font-bold mb-4" style="color: var(--text-color);">{{ t('admin.sla.trendTitle') }}</h3>
        <div v-if="trendLoading" class="text-center py-8" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>
        <div v-else-if="trend.length === 0" class="text-center py-8" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.sla.noData') }}</div>
        <div v-else class="space-y-3">
          <button
            v-for="point in trend"
            :key="point.month"
            @click="month = point.month; loadReport()"
            class="w-full text-left flex items-center gap-3 px-2 py-1.5 rounded-lg transition-colors hover:bg-[color:var(--button-hover-color)]"
            :style="point.month === report.month ? 'background-color: var(--button-hover-color);' : ''"
          >
            <span class="w-20 text-xs flex-shrink-0" style="color: var(--text-color); opacity: 0.6;">{{ point.label }}</span>
            <span class="flex-1 h-2 rounded-full overflow-hidden" style="background-color: var(--button-border-color);">
              <span
                class="block h-full rounded-full"
                :style="{
                  width: (point.totalProbes > 0 ? Math.max(point.uptime, 0) : 0) + '%',
                  backgroundColor: point.uptime >= 99.9 ? '#34a761' : point.uptime >= 99 ? '#fda305' : '#df2d2a',
                }"
              />
            </span>
            <span class="w-24 text-right text-xs font-medium flex-shrink-0" :class="uptimeClass(point.uptime, point.totalProbes)">
              {{ point.totalProbes > 0 ? point.uptime.toFixed(2) + '%' : t('admin.sla.noData') }}
            </span>
            <span class="w-20 text-right text-xs flex-shrink-0" style="color: var(--text-color); opacity: 0.45;">
              {{ t('admin.sla.trendIncidents', { n: point.incidents, max: trendMaxIncidents }) }}
            </span>
          </button>
        </div>
      </div>
    </template>
  </div>
</template>
