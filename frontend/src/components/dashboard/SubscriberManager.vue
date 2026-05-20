<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../../api/client'
import { useToast } from '../../composables/useToast'

interface Subscriber {
  id: number
  email: string
  verified: boolean
  createdAt: string
  updatedAt: string
}

const { show: toast } = useToast()
const subscribers = ref<Subscriber[]>([])
const loading = ref(true)
const showDelete = ref(false)
const deleteTarget = ref<Subscriber | null>(null)
const deleting = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await api.request<{ code: number; data: Subscriber[] }>('GET', '/admin/subscribers', undefined, true)
    subscribers.value = res.data
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

function confirmDelete(s: Subscriber) {
  deleteTarget.value = s
  showDelete.value = true
}

async function handleDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await api.request('DELETE', `/admin/subscribers/${deleteTarget.value.id}`, undefined, true)
    toast('已删除', 'success')
    showDelete.value = false
    deleteTarget.value = null
    load()
  } catch (e: any) {
    toast(e.message || '删除失败')
  } finally {
    deleting.value = false
  }
}

function formatDate(iso: string): string {
  if (!iso) return '-'
  return new Date(iso).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })
}

onMounted(load)
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100">订阅管理</h2>
    </div>

    <div v-if="loading" class="text-center py-12 text-gray-400 dark:text-gray-500">加载中...</div>

    <div v-else-if="subscribers.length === 0" class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm p-12 text-center">
      <svg class="w-16 h-16 mx-auto mb-4 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" stroke-width="1" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
      </svg>
      <p class="text-base font-medium text-gray-500 dark:text-gray-400">暂无订阅者</p>
      <p class="text-sm text-gray-400 dark:text-gray-500 mt-1">公开页面有用户订阅后，将显示在这里。</p>
    </div>

    <div v-else class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm overflow-hidden">
      <table class="w-full text-left text-sm">
        <thead class="text-xs text-gray-400 dark:text-gray-500 bg-gray-50/50 dark:bg-gray-800/50 border-b border-gray-100 dark:border-gray-800">
          <tr>
            <th class="px-6 py-3 font-medium">邮箱</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">订阅时间</th>
            <th class="px-6 py-3 font-medium text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50 dark:divide-gray-800">
          <tr v-for="s in subscribers" :key="s.id" class="hover:bg-gray-50/50 dark:hover:bg-gray-800/50">
            <td class="px-6 py-4 font-medium text-gray-900 dark:text-gray-100">{{ s.email }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', s.verified ? 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-900/30' : 'text-gray-500 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700']">
                {{ s.verified ? '已验证' : '未验证' }}
              </span>
            </td>
            <td class="px-6 py-4 text-xs text-gray-500 dark:text-gray-400">{{ formatDate(s.createdAt) }}</td>
            <td class="px-6 py-4 text-right">
              <button @click="confirmDelete(s)" class="text-gray-400 hover:text-red-500 dark:text-gray-500 dark:hover:text-red-400 transition-colors" title="删除">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Delete Confirmation -->
    <div v-if="showDelete" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showDelete = false">
      <div class="bg-white dark:bg-gray-900 rounded-xl shadow-xl p-6 w-full max-w-sm mx-4">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-2">确认删除</h3>
        <p class="text-sm text-gray-500 dark:text-gray-400 mb-1">确定要删除订阅者「{{ deleteTarget?.email }}」吗？</p>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showDelete = false" class="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 transition-colors">取消</button>
          <button
            @click="handleDelete"
            :disabled="deleting"
            class="px-4 py-2 bg-red-500 hover:bg-red-600 disabled:bg-gray-300 dark:disabled:bg-gray-700 text-white text-sm font-medium rounded-lg transition-colors"
          >
            {{ deleting ? '删除中...' : '确认删除' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
