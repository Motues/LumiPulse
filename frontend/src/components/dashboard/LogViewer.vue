<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '../../api/client'
import type { LogEntry, ServiceSummary } from '../../api/types'

const logs = ref<LogEntry[]>([])
const loading = ref(true)
const page = ref(1)
const totalPage = ref(0)
const filterServiceId = ref(0)
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
    const res = await api.getLogs(page.value, 50, filterServiceId.value, filterStatus.value)
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
      api.getLogs(1, 50, filterServiceId.value, filterStatus.value).then(res => {
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
        <select
          v-model="filterServiceId"
          @change="onFilterChange"
          class="px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 bg-white dark:bg-gray-800 dark:text-gray-100"
        >
          <option :value="0">全部服务</option>
          <option v-for="svc in services" :key="svc.id" :value="svc.id">{{ svc.name }}</option>
        </select>
        <select
          v-model="filterStatus"
          @change="onFilterChange"
          class="px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 bg-white dark:bg-gray-800 dark:text-gray-100"
        >
          <option value="all">全部状态</option>
          <option value="success">正常</option>
          <option value="failure">异常</option>
        </select>
        <span class="text-xs text-gray-400 dark:text-gray-500">{{ logs.length }} 条记录</span>
      </div>
      <div class="flex items-center gap-2">
        <button
          @click="toggleRefresh"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors"
          :class="autoRefresh ? 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400' : 'bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400'"
        >
          <svg class="w-3.5 h-3.5" :class="{ 'animate-spin': autoRefresh }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ autoRefresh ? '实时' : '暂停' }}
        </button>
      </div>
    </div>

    <!-- Table -->
    <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm overflow-hidden">
      <div v-if="loading" class="text-center py-16 text-gray-400 dark:text-gray-500 text-sm">加载中...</div>
      <div v-else-if="logs.length === 0" class="text-center py-16 text-gray-400 dark:text-gray-500 text-sm">暂无监控日志</div>

      <template v-else>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-50 dark:border-gray-800 text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider">
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
                class="border-b border-gray-50 dark:border-gray-800 hover:bg-gray-50/50 dark:hover:bg-gray-800/50 transition-colors"
              >
                <td class="px-4 py-3 text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap font-mono">
                  {{ formatTime(log.createdAt) }}
                </td>
                <td class="px-4 py-3 text-gray-800 dark:text-gray-200 font-medium whitespace-nowrap">
                  {{ log.serviceName }}
                </td>
                <td class="px-4 py-3 whitespace-nowrap">
                  <span
                    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium"
                    :class="isSuccess(log.status)
                      ? 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300'
                      : 'bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-300'"
                  >
                    <span class="w-1.5 h-1.5 rounded-full" :class="isSuccess(log.status) ? 'bg-emerald-500' : 'bg-red-500'" />
                    {{ isSuccess(log.status) ? '正常' : '异常' }}
                    <span class="text-gray-400 dark:text-gray-500 font-mono">({{ statusLabel(log.status) }})</span>
                  </span>
                </td>
                <td class="px-4 py-3 whitespace-nowrap">
                  <span class="font-mono text-xs" :class="latencyColor(log.latency)">
                    {{ latencyText(log.latency) }}
                  </span>
                </td>
                <td class="px-4 py-3 text-xs text-gray-500 dark:text-gray-400 max-w-xs truncate" :title="log.message">
                  {{ log.message || '-' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div v-if="totalPage > 1" class="flex items-center justify-between px-4 py-3 border-t border-gray-50 dark:border-gray-800">
          <span class="text-xs text-gray-500 dark:text-gray-400">第 {{ page }} / {{ totalPage }} 页</span>
          <div class="flex items-center gap-2">
            <button
              @click="prevPage"
              :disabled="page <= 1"
              class="px-3 py-1 text-xs rounded-md border border-gray-200 dark:border-gray-700 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors dark:text-gray-300"
            >上一页</button>
            <button
              @click="nextPage"
              :disabled="page >= totalPage"
              class="px-3 py-1 text-xs rounded-md border border-gray-200 dark:border-gray-700 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors dark:text-gray-300"
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
