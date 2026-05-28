<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '../../api/client'
import type { LogEntry, ServiceSummary } from '../../api/types'
import CustomSelect from './CustomSelect.vue'

const logs = ref<LogEntry[]>([])
const loading = ref(true)
const page = ref(1)
const totalPage = ref(0)
const filterServiceId = ref('0')
const filterStatus = ref('all')
const services = ref<ServiceSummary[]>([])
const autoRefresh = ref(true)
let refreshTimer: ReturnType<typeof setInterval> | null = null

async function loadServices() {
  try {
    const res = await api.getSummary()
    services.value = res.data.services
  } catch {
    // silent
  }
}

async function fetchLogs() {
  try {
    const res = await api.getLogs(page.value, 50, Number(filterServiceId.value), filterStatus.value)
    logs.value = res.data.logs
    totalPage.value = res.data.pagination.totalPage
  } catch {
    // silent
  } finally {
    loading.value = false
  }
}

function startRefresh() {
  stopRefresh()
  if (autoRefresh.value) {
    refreshTimer = setInterval(() => {
      api.getLogs(1, 50, Number(filterServiceId.value), filterStatus.value).then(res => {
        logs.value = res.data.logs
        totalPage.value = res.data.pagination.totalPage
        page.value = 1
      }).catch(() => {})
    }, 10000)
  }
}

function stopRefresh() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

function toggleRefresh() {
  autoRefresh.value = !autoRefresh.value
  if (autoRefresh.value) {
    startRefresh()
  } else {
    stopRefresh()
  }
}

function prevPage() {
  if (page.value > 1) {
    page.value--
    fetchLogs()
  }
}

function nextPage() {
  if (page.value < totalPage.value) {
    page.value++
    fetchLogs()
  }
}

function onFilterChange() {
  page.value = 1
  loading.value = true
  fetchLogs()
}

function isSuccess(status: number): boolean {
  return (status >= 200 && status < 400) || status === 1
}

function statusLabel(status: number): string {
  if (status === 1) return 'TCP OK'
  if (status >= 200 && status < 300) return `${status}`
  if (status >= 300 && status < 400) return `${status}`
  return `${status}`
}

function latencyText(ms: number): string {
  if (ms === 0) return '<1ms'
  return `${ms}ms`
}

function latencyColor(ms: number): string {
  if (ms < 200) return 'text-emerald-600 dark:text-emerald-400'
  if (ms < 500) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-500 dark:text-red-400'
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

onMounted(() => {
  loadServices()
  fetchLogs()
  startRefresh()
})

onUnmounted(() => {
  stopRefresh()
})
</script>

<template>
  <div>
    <!-- Toolbar -->
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-center gap-3">
        <CustomSelect
          v-model="filterServiceId"
          @update:modelValue="onFilterChange"
          :options="[{ label: '全部服务', value: '0' }, ...services.map(svc => ({ label: svc.name, value: String(svc.id) }))]"
          min-width="140px"
        />
        <CustomSelect
          v-model="filterStatus"
          @update:modelValue="onFilterChange"
          :options="[{ label: '全部状态', value: 'all' }, { label: '正常', value: 'success' }, { label: '异常', value: 'failure' }]"
          min-width="110px"
        />
        <span class="text-xs" style="color: var(--text-color); opacity: 0.4;">{{ logs.length }} 条记录</span>
      </div>
      <div class="flex items-center gap-2">
        <button
          @click="toggleRefresh"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
          :class="autoRefresh ? 'bg-emerald-50 dark:bg-emerald-500/15 text-emerald-600 dark:text-emerald-400' : ''"
          :style="!autoRefresh ? { backgroundColor: 'var(--bg-color)', color: 'var(--text-color)', opacity: 0.5 } : {}"
        >
          <svg class="w-3.5 h-3.5" :class="{ 'animate-spin': autoRefresh }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ autoRefresh ? '实时' : '暂停' }}
        </button>
      </div>
    </div>

    <!-- Table -->
    <div class="rounded-xl overflow-hidden" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div v-if="loading" class="text-center py-16 text-sm" style="color: var(--text-color); opacity: 0.4;">加载中...</div>
      <div v-else-if="logs.length === 0" class="text-center py-16 text-sm" style="color: var(--text-color); opacity: 0.4;">暂无监控日志</div>

      <template v-else>
        <div class="overflow-x-auto thin-scroll">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-xs uppercase tracking-wider" style="border-bottom: 1px solid var(--button-border-color); color: var(--text-color); opacity: 0.5;">
                <th class="text-left px-4 py-3 font-medium">时间</th>
                <th class="text-left px-4 py-3 font-medium">服务</th>
                <th class="text-left px-4 py-3 font-medium">状态</th>
                <th class="text-left px-4 py-3 font-medium">延迟</th>
                <th class="text-left px-4 py-3 font-medium">信息</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="log in logs" :key="log.id"
                class="transition-colors"
                style="border-bottom: 1px solid var(--button-border-color);"
              >
                <td class="px-4 py-3 text-xs whitespace-nowrap font-mono" style="color: var(--text-color); opacity: 0.5;">
                  {{ formatTime(log.createdAt) }}
                </td>
                <td class="px-4 py-3 font-medium whitespace-nowrap" style="color: var(--text-color); opacity: 0.8;">
                  {{ log.serviceName }}
                </td>
                <td class="px-4 py-3 whitespace-nowrap">
                  <span
                    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium"
                    :class="isSuccess(log.status)
                      ? 'bg-emerald-50 dark:bg-emerald-500/15 text-emerald-700 dark:text-emerald-300'
                      : 'bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-300'"
                  >
                    <span class="w-1.5 h-1.5 rounded-full" :class="isSuccess(log.status) ? 'bg-emerald-500' : 'bg-red-500'" />
                    {{ isSuccess(log.status) ? '正常' : '异常' }}
                    <span class="font-mono" style="color: var(--text-color); opacity: 0.4;">({{ statusLabel(log.status) }})</span>
                  </span>
                </td>
                <td class="px-4 py-3 whitespace-nowrap">
                  <span class="font-mono text-xs" :class="latencyColor(log.latency)">
                    {{ latencyText(log.latency) }}
                  </span>
                </td>
                <td class="px-4 py-3 text-xs max-w-xs truncate" :title="log.message" style="color: var(--text-color); opacity: 0.5;">
                  {{ log.message || '-' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div v-if="totalPage > 1" class="flex items-center justify-between px-4 py-3" style="border-top: 1px solid var(--button-border-color);">
          <span class="text-xs" style="color: var(--text-color); opacity: 0.5;">第 {{ page }} / {{ totalPage }} 页</span>
          <div class="flex items-center gap-2">
            <button
              @click="prevPage"
              :disabled="page <= 1"
              class="px-3 py-1 text-xs rounded-md border disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
              style="border-color: var(--button-border-color); color: var(--text-color);"
            >上一页</button>
            <button
              @click="nextPage"
              :disabled="page >= totalPage"
              class="px-3 py-1 text-xs rounded-md border disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
              style="border-color: var(--button-border-color); color: var(--text-color);"
            >下一页</button>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
.animate-spin {
  animation: spin 1s linear infinite;
}
</style>
