<template>
  <div class="mx-auto max-w-7xl">
    <!-- Stats -->
    <div class="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
      <div v-for="stat in stats" :key="stat.label"
        class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
        <p class="text-sm text-gray-500">{{ stat.label }}</p>
        <p class="mt-1 text-2xl font-bold text-gray-900">{{ stat.value }}</p>
        <p class="mt-1 text-xs" :class="stat.trend >= 0 ? 'text-green-600' : 'text-red-600'">
          {{ stat.trend >= 0 ? '+' : '' }}{{ stat.trend }}% 较上周
        </p>
      </div>
    </div>

    <!-- Main content -->
    <div class="mt-8 grid grid-cols-1 gap-6 lg:grid-cols-2">
      <!-- Chart placeholder -->
      <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
        <h3 class="text-base font-semibold text-gray-900">服务可用率</h3>
        <div class="mt-4 flex h-64 items-center justify-center rounded-lg bg-gray-50 text-gray-400">
          <div class="text-center">
            <p class="text-5xl font-bold text-indigo-500">99.9%</p>
            <p class="mt-2 text-sm">过去 30 天 SLA</p>
          </div>
        </div>
      </div>

      <!-- Recent activity -->
      <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
        <h3 class="text-base font-semibold text-gray-900">最近活动</h3>
        <div class="mt-4 space-y-4">
          <div v-for="(event, i) in events" :key="i" class="flex items-start gap-3">
            <div class="mt-1 h-2 w-2 rounded-full" :class="event.color"></div>
            <div class="flex-1">
              <p class="text-sm text-gray-700">{{ event.text }}</p>
              <p class="text-xs text-gray-400">{{ event.time }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Status table -->
    <div class="mt-8 rounded-xl border border-gray-200 bg-white shadow-sm">
      <div class="border-b border-gray-200 px-6 py-4">
        <h3 class="text-base font-semibold text-gray-900">服务状态</h3>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="bg-gray-50 text-xs uppercase text-gray-500">
            <tr>
              <th class="px-6 py-3">服务名称</th>
              <th class="px-6 py-3">状态</th>
              <th class="px-6 py-3">响应时间</th>
              <th class="px-6 py-3">可用率</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200">
            <tr v-for="svc in services" :key="svc.name" class="hover:bg-gray-50">
              <td class="px-6 py-4 font-medium text-gray-900">{{ svc.name }}</td>
              <td class="px-6 py-4">
                <span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium"
                  :class="svc.status === '正常' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'">
                  <span class="h-1.5 w-1.5 rounded-full" :class="svc.status === '正常' ? 'bg-green-500' : 'bg-red-500'"></span>
                  {{ svc.status }}
                </span>
              </td>
              <td class="px-6 py-4 text-gray-600">{{ svc.latency }}</td>
              <td class="px-6 py-4 text-gray-600">{{ svc.sla }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const stats = [
  { label: '总服务数', value: '8', trend: 0 },
  { label: '在线服务', value: '8', trend: 0 },
  { label: '今日告警', value: '2', trend: -50 },
  { label: '平均响应', value: '124ms', trend: -5 },
]

const events = [
  { text: 'API 服务 响应时间异常升高', color: 'bg-orange-500', time: '2 分钟前' },
  { text: 'Web 服务 已完成健康检查', color: 'bg-green-500', time: '15 分钟前' },
  { text: '数据库 连接池恢复正常', color: 'bg-green-500', time: '1 小时前' },
  { text: '缓存服务 进行例行维护', color: 'bg-blue-500', time: '2 小时前' },
]

const services = [
  { name: 'API 服务', status: '正常', latency: '42ms', sla: '99.99%' },
  { name: 'Web 服务', status: '正常', latency: '23ms', sla: '99.95%' },
  { name: '数据库', status: '正常', latency: '8ms', sla: '99.99%' },
  { name: '缓存服务', status: '正常', latency: '3ms', sla: '100%' },
]
</script>
