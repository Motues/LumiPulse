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

// Subscription method toggles
const subSettings = ref({
  sub_enable_email: 'true',
  sub_enable_rss: 'true',
  sub_enable_atom: 'true',
})
const settingsLoading = ref(true)
const settingsSaving = ref(false)

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

async function loadSettings() {
  settingsLoading.value = true
  try {
    const res = await api.getSettings()
    subSettings.value.sub_enable_email = res.data['sub_enable_email'] ?? 'true'
    subSettings.value.sub_enable_rss = res.data['sub_enable_rss'] ?? 'true'
    subSettings.value.sub_enable_atom = res.data['sub_enable_atom'] ?? 'true'
  } catch (e: any) {
    toast(e.message || '加载设置失败')
  } finally {
    settingsLoading.value = false
  }
}

async function saveSubSettings() {
  settingsSaving.value = true
  try {
    await api.updateSettings(subSettings.value)
    toast('订阅方式设置已更新', 'success')
  } catch (e: any) {
    toast(e.message || '保存失败')
  } finally {
    settingsSaving.value = false
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

onMounted(() => {
  load()
  loadSettings()
})
</script>

<template>
  <div>
    <!-- Subscription Method Settings -->
    <div class="rounded-xl p-6 mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <h3 class="text-sm font-bold mb-4" style="color: var(--text-color);">订阅方式开关</h3>
      <div v-if="settingsLoading" class="text-sm" style="color: var(--text-color); opacity: 0.4;">加载中...</div>
      <div v-else class="space-y-4 max-w-sm">
        <!-- Email -->
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm font-medium" style="color: var(--text-color);">邮件订阅</div>
            <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">通过邮箱接收状态变更通知</div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" v-model="subSettings.sub_enable_email" true-value="true" false-value="false" class="sr-only" />
            <span class="flex items-center rounded-full transition-colors duration-200" :class="subSettings.sub_enable_email === 'true' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'" style="width: 36px; height: 20px; flex-shrink: 0;">
              <span class="bg-white rounded-full shadow transition-transform duration-200" :class="subSettings.sub_enable_email === 'true' ? 'translate-x-[17px]' : 'translate-x-[2px]'" style="width: 15px; height: 15px;"></span>
            </span>
          </label>
        </div>

        <!-- RSS -->
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm font-medium" style="color: var(--text-color);">RSS 订阅</div>
            <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">通过 RSS 阅读器订阅状态更新</div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" v-model="subSettings.sub_enable_rss" true-value="true" false-value="false" class="sr-only" />
            <span class="flex items-center rounded-full transition-colors duration-200" :class="subSettings.sub_enable_rss === 'true' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'" style="width: 36px; height: 20px; flex-shrink: 0;">
              <span class="bg-white rounded-full shadow transition-transform duration-200" :class="subSettings.sub_enable_rss === 'true' ? 'translate-x-[17px]' : 'translate-x-[2px]'" style="width: 15px; height: 15px;"></span>
            </span>
          </label>
        </div>

        <!-- Atom -->
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm font-medium" style="color: var(--text-color);">Atom 订阅</div>
            <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">通过 Atom 阅读器订阅状态更新</div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" v-model="subSettings.sub_enable_atom" true-value="true" false-value="false" class="sr-only" />
            <span class="flex items-center rounded-full transition-colors duration-200" :class="subSettings.sub_enable_atom === 'true' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'" style="width: 36px; height: 20px; flex-shrink: 0;">
              <span class="bg-white rounded-full shadow transition-transform duration-200" :class="subSettings.sub_enable_atom === 'true' ? 'translate-x-[17px]' : 'translate-x-[2px]'" style="width: 15px; height: 15px;"></span>
            </span>
          </label>
        </div>

        <div class="pt-2">
          <button
            @click="saveSubSettings"
            :disabled="settingsSaving"
            class="px-4 py-1.5 bg-emerald-500 hover:bg-emerald-600 disabled:opacity-40 text-white text-xs font-medium rounded-lg transition-colors"
          >
            {{ settingsSaving ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Subscriber List -->
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-bold" style="color: var(--text-color);">订阅列表</h2>
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">加载中...</div>

    <div v-else-if="subscribers.length === 0" class="rounded-xl p-12 text-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <svg class="w-16 h-16 mx-auto mb-4" style="color: var(--text-color); opacity: 0.3;" fill="none" stroke="currentColor" stroke-width="1" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
      </svg>
      <p class="text-base font-medium" style="color: var(--text-color); opacity: 0.5;">暂无订阅者</p>
      <p class="text-sm mt-1" style="color: var(--text-color); opacity: 0.4;">公开页面有用户订阅后，将显示在这里。</p>
    </div>

    <div v-else class="rounded-xl overflow-hidden" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <table class="w-full text-left text-sm">
        <thead class="text-xs" style="background-color: var(--button-hover-color); opacity: 0.5; border-bottom: 1px solid var(--button-border-color); color: var(--text-color);">
          <tr>
            <th class="px-6 py-3 font-medium">邮箱</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">订阅时间</th>
            <th class="px-6 py-3 font-medium text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr v-for="s in subscribers" :key="s.id">
            <td class="px-6 py-4 font-medium" style="color: var(--text-color);">{{ s.email }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', s.verified ? 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15' : '']" :style="!s.verified ? { color: 'var(--text-color)', opacity: 0.5, backgroundColor: 'var(--button-hover-color)', borderColor: 'var(--button-border-color)' } : {}">
                {{ s.verified ? '已验证' : '未验证' }}
              </span>
            </td>
            <td class="px-6 py-4 text-xs" style="color: var(--text-color); opacity: 0.5;">{{ formatDate(s.createdAt) }}</td>
            <td class="px-6 py-4 text-right">
              <button @click="confirmDelete(s)" class="text-red-500 hover:text-red-600 transition-colors" title="删除">
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
      <div class="rounded-xl p-6 w-full max-w-sm mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-2" style="color: var(--text-color);">确认删除</h3>
        <p class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">确定要删除订阅者「{{ deleteTarget?.email }}」吗？</p>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showDelete = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">取消</button>
          <button
            @click="handleDelete"
            :disabled="deleting"
            class="px-4 py-2 bg-red-500 hover:bg-red-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="deleting ? { opacity: 0.3 } : {}"
          >
            {{ deleting ? '删除中...' : '确认删除' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
