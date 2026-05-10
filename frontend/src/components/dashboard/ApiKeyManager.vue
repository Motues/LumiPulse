<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { ApiKey, ApiKeyCreated } from '../../api/types'
import { useToast } from '../../composables/useToast'

const { show: toast } = useToast()

const keys = ref<ApiKey[]>([])
const loading = ref(true)

// Create modal
const showCreate = ref(false)
const creating = ref(false)
const form = ref({ name: '', expiresAt: '', permanent: true })

// After-creation overlay
const showKey = ref(false)
const createdKey = ref<ApiKeyCreated | null>(null)
const copied = ref(false)

// Delete confirmation
const showDelete = ref(false)
const deleting = ref(false)
const deleteTarget = ref<ApiKey | null>(null)

// Edit modal
const showEdit = ref(false)
const editTarget = ref<ApiKey | null>(null)
const editName = ref('')
const saving = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await api.getApiKeys()
    keys.value = res.data
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = { name: '', expiresAt: '', permanent: true }
  showCreate.value = true
}

async function handleCreate() {
  if (!form.value.name.trim()) return
  creating.value = true
  try {
    const data: { name: string; expiresAt: string } = {
      name: form.value.name.trim(),
      expiresAt: form.value.permanent ? '' : form.value.expiresAt,
    }
    const res = await api.createApiKey(data)
    createdKey.value = res.data
    showCreate.value = false
    showKey.value = true
    await load()
  } catch (e: any) {
    toast(e.message || '创建失败')
  } finally {
    creating.value = false
  }
}

function copyKey() {
  if (!createdKey.value) return
  navigator.clipboard.writeText(createdKey.value.key).then(() => {
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  })
}

function closeKeyOverlay() {
  showKey.value = false
  createdKey.value = null
  copied.value = false
}

function confirmDelete(k: ApiKey) {
  deleteTarget.value = k
  showDelete.value = true
}

async function handleDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await api.deleteApiKey(deleteTarget.value.id)
    toast('密钥已删除', 'success')
    showDelete.value = false
    deleteTarget.value = null
    await load()
  } catch (e: any) {
    toast(e.message || '删除失败')
  } finally {
    deleting.value = false
  }
}

function openEdit(k: ApiKey) {
  editTarget.value = k
  editName.value = k.name
  showEdit.value = true
}

async function handleEdit() {
  if (!editTarget.value || !editName.value.trim()) return
  saving.value = true
  try {
    await api.updateApiKey(editTarget.value.id, { name: editName.value.trim() })
    toast('名称已更新', 'success')
    showEdit.value = false
    editTarget.value = null
    await load()
  } catch (e: any) {
    toast(e.message || '更新失败')
  } finally {
    saving.value = false
  }
}

function formatDate(iso: string): string {
  if (!iso) return '-'
  return new Date(iso).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })
}

function maskKey(k: ApiKey): string {
  return k.maskedKey || '-'
}

onMounted(load)
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100">API 密钥管理</h2>
      <button
        @click="openCreate"
        class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors"
      >
        + 创建密钥
      </button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="text-center py-12 text-gray-400 dark:text-gray-500">加载中...</div>

    <!-- Empty state -->
    <div v-else-if="keys.length === 0" class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm p-12 text-center">
      <svg class="w-16 h-16 mx-auto mb-4 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" stroke-width="1" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 11-7.778 7.778 5.5 5.5 0 017.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
      </svg>
      <p class="text-base font-medium text-gray-500 dark:text-gray-400">暂无 API 密钥</p>
      <p class="text-sm text-gray-400 dark:text-gray-500 mt-1">创建密钥后可用于 API 访问</p>
    </div>

    <!-- Table -->
    <div v-else class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm overflow-hidden">
      <table class="w-full text-left text-sm">
        <thead class="text-xs text-gray-400 dark:text-gray-500 bg-gray-50/50 dark:bg-gray-800/50 border-b border-gray-100 dark:border-gray-800">
          <tr>
            <th class="px-6 py-3 font-medium">名称</th>
            <th class="px-6 py-3 font-medium">密钥</th>
            <th class="px-6 py-3 font-medium">创建时间</th>
            <th class="px-6 py-3 font-medium">过期时间</th>
            <th class="px-6 py-3 font-medium">最近使用</th>
            <th class="px-6 py-3 font-medium"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50 dark:divide-gray-800">
          <tr v-for="k in keys" :key="k.id" class="hover:bg-gray-50/50 dark:hover:bg-gray-800/50">
            <td class="px-6 py-4">
              <span class="font-medium text-gray-900 dark:text-gray-100">{{ k.name }}</span>
            </td>
            <td class="px-6 py-4">
              <code class="text-xs bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded text-gray-600 dark:text-gray-300 font-mono">{{ maskKey(k) }}</code>
            </td>
            <td class="px-6 py-4 text-gray-500 dark:text-gray-400 text-xs">{{ formatDate(k.createdAt) }}</td>
            <td class="px-6 py-4">
              <span v-if="!k.expiresAt" class="text-emerald-600 dark:text-emerald-400 text-xs font-medium">永久有效</span>
              <span v-else class="text-gray-500 dark:text-gray-400 text-xs">{{ formatDate(k.expiresAt) }}</span>
            </td>
            <td class="px-6 py-4 text-xs">
              <div v-if="k.lastUsedAt" class="text-gray-500 dark:text-gray-400">
                <div>{{ formatDate(k.lastUsedAt) }}</div>
                <div v-if="k.lastUsedIP" class="text-gray-400 dark:text-gray-500">{{ k.lastUsedIP }}</div>
              </div>
              <span v-else class="text-gray-400 dark:text-gray-500">从未使用</span>
            </td>
            <td class="px-6 py-4 text-right">
              <button @click="openEdit(k)" class="text-gray-400 hover:text-blue-500 dark:text-gray-500 dark:hover:text-blue-400 transition-colors mr-3" title="编辑名称">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click="confirmDelete(k)" class="text-gray-400 hover:text-red-500 dark:text-gray-500 dark:hover:text-red-400 transition-colors" title="删除">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create Modal -->
    <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showCreate = false">
      <div class="bg-white dark:bg-gray-900 rounded-xl shadow-xl p-6 w-full max-w-md mx-4">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-4">创建 API 密钥</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">密钥名称 <span class="text-red-500">*</span></label>
            <input
              v-model="form.name"
              type="text"
              placeholder="例如：生产环境监控"
              class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100"
            />
          </div>
          <div>
            <label class="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" v-model="form.permanent" class="rounded border-gray-300 text-emerald-500 focus:ring-emerald-500" />
              <span class="text-sm text-gray-700 dark:text-gray-300">永久有效</span>
            </label>
          </div>
          <div v-if="!form.permanent">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">过期时间</label>
            <input
              v-model="form.expiresAt"
              type="datetime-local"
              class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100"
            />
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showCreate = false" class="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 transition-colors">取消</button>
          <button
            @click="handleCreate"
            :disabled="creating || !form.name.trim()"
            class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:bg-gray-300 dark:disabled:bg-gray-700 text-white text-sm font-medium rounded-lg transition-colors"
          >
            {{ creating ? '创建中...' : '创建' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Key Reveal Overlay -->
    <div v-if="showKey && createdKey" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="closeKeyOverlay">
      <div class="bg-white dark:bg-gray-900 rounded-xl shadow-xl p-6 w-full max-w-lg mx-4">
        <div class="flex items-center gap-2 mb-4">
          <svg class="w-6 h-6 text-emerald-500" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100">密钥创建成功</h3>
        </div>

        <div class="bg-amber-50 border border-amber-200 rounded-lg p-3 text-sm text-amber-700 mb-4">
          <strong>请立即复制并妥善保存此密钥。</strong>关闭后密钥将不再完整显示。
        </div>

        <div class="bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 mb-4">
          <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">密钥名称</div>
          <div class="font-medium text-gray-900 dark:text-gray-100">{{ createdKey.name }}</div>
        </div>

        <div class="bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 mb-4">
          <div class="text-xs text-gray-500 dark:text-gray-400 mb-1">完整密钥</div>
          <div class="flex items-center gap-2">
            <code class="flex-1 text-sm bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded px-3 py-2 font-mono break-all select-all dark:text-gray-200">{{ createdKey.key }}</code>
            <button
              @click="copyKey"
              class="flex-shrink-0 px-3 py-2 bg-indigo-500 hover:bg-indigo-600 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {{ copied ? '已复制' : '复制' }}
            </button>
          </div>
        </div>

        <div class="flex justify-end">
          <button @click="closeKeyOverlay" class="px-4 py-2 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 text-sm font-medium rounded-lg transition-colors">
            我已保存，关闭
          </button>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation -->
    <div v-if="showDelete" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showDelete = false">
      <div class="bg-white dark:bg-gray-900 rounded-xl shadow-xl p-6 w-full max-w-sm mx-4">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-2">确认删除</h3>
        <p class="text-sm text-gray-500 dark:text-gray-400 mb-1">确定要删除密钥「{{ deleteTarget?.name }}」吗？</p>
        <p class="text-sm text-red-500">使用了此密钥的应用将立即无法访问 API。</p>
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

    <!-- Edit Modal -->
    <div v-if="showEdit && editTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showEdit = false">
      <div class="bg-white dark:bg-gray-900 rounded-xl shadow-xl p-6 w-full max-w-md mx-4">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-4">修改密钥名称</h3>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">密钥名称</label>
          <input
            v-model="editName"
            type="text"
            class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100"
            autofocus
            @keydown.enter="handleEdit"
          />
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showEdit = false" class="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 transition-colors">取消</button>
          <button
            @click="handleEdit"
            :disabled="saving || !editName.trim()"
            class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:bg-gray-300 dark:disabled:bg-gray-700 text-white text-sm font-medium rounded-lg transition-colors"
          >
            {{ saving ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
