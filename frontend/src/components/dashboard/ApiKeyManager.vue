<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { ApiKey, ApiKeyCreated } from '../../api/types'
import { useToast } from '../../composables/useToast'
import { useI18n } from '../../composables/useI18n'
import DateTimePicker from './DateTimePicker.vue'

const { show: toast } = useToast()
const { t, formatDateTime } = useI18n()

const keys = ref<ApiKey[]>([])
const loading = ref(true)

// Create modal
const showCreate = ref(false)
const creating = ref(false)
const form = ref({ name: '', expiresAt: '', permanent: true, scope: 'read', rateLimitPerMinute: 0 })

const scopeOptions = computed(() => [
  { label: t('admin.apikey.scopeReadFull'), value: 'read' },
  { label: t('admin.apikey.scopeWriteFull'), value: 'write' },
])

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
const editScope = ref('read')
const editRateLimit = ref(0)
const saving = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await api.getApiKeys()
    keys.value = res.data
  } catch (e: any) {
    toast(e.message || t('admin.common.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = { name: '', expiresAt: '', permanent: true, scope: 'read', rateLimitPerMinute: 0 }
  showCreate.value = true
}

/**
 * 把选择器给出的 "YYYY-MM-DDTHH:mm" 转成带北京时区偏移的 RFC3339 串。
 * 后端用 time.RFC3339 解析 expires_at，直接提交 datetime-local 形式会被判为非法而立刻过期。
 */
function toCSTTimestamp(local: string): string {
  if (!local) return ''
  const t = local.length === 16 ? `${local}:00` : local
  return /[Zz]$|[+-]\d{2}:?\d{2}$/.test(t) ? t : `${t}+08:00`
}

async function handleCreate() {
  if (!form.value.name.trim()) return
  if (!form.value.permanent && !form.value.expiresAt) {
    toast(t('admin.apikey.expiresRequired'))
    return
  }
  creating.value = true
  try {
    const data: { name: string; expiresAt: string; scope: string; rateLimitPerMinute: number } = {
      name: form.value.name.trim(),
      expiresAt: form.value.permanent ? '' : toCSTTimestamp(form.value.expiresAt),
      scope: form.value.scope,
      rateLimitPerMinute: Number(form.value.rateLimitPerMinute) || 0,
    }
    const res = await api.createApiKey(data)
    createdKey.value = res.data
    showCreate.value = false
    showKey.value = true
    await load()
  } catch (e: any) {
    toast(e.message || t('admin.apikey.createFailed'))
  } finally {
    creating.value = false
  }
}

function copyKey() {
  if (!createdKey.value) return
  navigator.clipboard.writeText(createdKey.value.key).then(() => {
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  }).catch(() => {
    // Clipboard write may fail in non-HTTPS contexts
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
    toast(t('admin.apikey.deleted'), 'success')
    showDelete.value = false
    deleteTarget.value = null
    await load()
  } catch (e: any) {
    toast(e.message || t('admin.common.deleteFailed'))
  } finally {
    deleting.value = false
  }
}

function openEdit(k: ApiKey) {
  editTarget.value = k
  editName.value = k.name
  editScope.value = k.scope || 'read'
  editRateLimit.value = k.rateLimitPerMinute || 0
  showEdit.value = true
}

async function handleEdit() {
  if (!editTarget.value || !editName.value.trim()) return
  saving.value = true
  try {
    await api.updateApiKey(editTarget.value.id, {
      name: editName.value.trim(),
      scope: editScope.value,
      rateLimitPerMinute: Number(editRateLimit.value) || 0,
    })
    toast(t('admin.apikey.updated'), 'success')
    showEdit.value = false
    editTarget.value = null
    await load()
  } catch (e: any) {
    toast(e.message || t('admin.apikey.updateFailed'))
  } finally {
    saving.value = false
  }
}

function formatDate(iso: string): string {
  if (!iso) return '-'
  return formatDateTime(iso)
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
      <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ t('admin.apikey.title') }}</h2>
      <button
        @click="openCreate"
        class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors"
      >
        + {{ t('admin.apikey.create') }}
      </button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <!-- Empty state -->
    <div v-else-if="keys.length === 0" class="rounded-xl p-12 text-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <svg class="w-16 h-16 mx-auto mb-4" style="color: var(--text-color); opacity: 0.3;" fill="none" stroke="currentColor" stroke-width="1" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 11-7.778 7.778 5.5 5.5 0 017.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
      </svg>
      <p class="text-base font-medium" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.apikey.emptyTitle') }}</p>
      <p class="text-sm mt-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.apikey.emptyHint') }}</p>
    </div>

    <!-- Table -->
    <div v-else class="rounded-xl overflow-hidden" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <table class="w-full text-left text-sm">
        <thead class="text-xs" style="background-color: var(--button-hover-color); opacity: 0.5; border-bottom: 1px solid var(--button-border-color); color: var(--text-color);">
          <tr>
            <th class="px-6 py-3 font-medium">{{ t('admin.apikey.colName') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.apikey.colKey') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.apikey.colScope') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.apikey.colCreatedAt') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.apikey.colExpiresAt') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.apikey.colLastUsed') }}</th>
            <th class="px-6 py-3 font-medium"></th>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr v-for="k in keys" :key="k.id">
            <td class="px-6 py-4">
              <span class="font-medium" style="color: var(--text-color);">{{ k.name }}</span>
            </td>
            <td class="px-6 py-4">
              <code class="text-xs px-2 py-1 rounded font-mono" style="background-color: var(--button-hover-color); color: var(--text-color); opacity: 0.7;">{{ maskKey(k) }}</code>
            </td>
            <td class="px-6 py-4">
              <span
                class="text-xs px-2 py-0.5 rounded font-medium"
                :class="(k.scope || 'read') === 'write'
                  ? 'bg-amber-50 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400'
                  : 'bg-emerald-50 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-400'"
              >{{ (k.scope || 'read') === 'write' ? t('admin.apikey.scopeWrite') : t('admin.apikey.scopeRead') }}</span>
              <div v-if="k.rateLimitPerMinute > 0" class="text-[10px] mt-1" style="color: var(--text-color); opacity: 0.4;">
                {{ t('admin.apikey.rateLimitShort', { n: k.rateLimitPerMinute }) }}
              </div>
            </td>
            <td class="px-6 py-4 text-xs" style="color: var(--text-color); opacity: 0.5;">{{ formatDate(k.createdAt) }}</td>
            <td class="px-6 py-4">
              <span v-if="!k.expiresAt" class="text-emerald-600 dark:text-emerald-400 text-xs font-medium">{{ t('admin.apikey.neverExpires') }}</span>
              <span v-else class="text-xs" style="color: var(--text-color); opacity: 0.5;">{{ formatDate(k.expiresAt) }}</span>
            </td>
            <td class="px-6 py-4 text-xs">
              <div v-if="k.lastUsedAt" style="color: var(--text-color); opacity: 0.5;">
                <div>{{ formatDate(k.lastUsedAt) }}</div>
                <div v-if="k.lastUsedIP" style="color: var(--text-color); opacity: 0.4;">{{ k.lastUsedIP }}</div>
              </div>
              <span v-else style="color: var(--text-color); opacity: 0.4;">{{ t('admin.apikey.neverUsed') }}</span>
            </td>
            <td class="px-6 py-4 text-right">
              <button @click="openEdit(k)" class="op-btn op-btn-edit mr-3" :title="t('admin.apikey.editName')">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button @click="confirmDelete(k)" class="text-red-500 hover:text-red-600 transition-colors" :title="t('admin.common.delete')">
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
      <div class="rounded-xl p-6 w-full max-w-md mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.apikey.createTitle') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.apikey.name') }} <span class="text-red-500">*</span></label>
            <input
              v-model="form.name"
              type="text"
              :placeholder="t('admin.apikey.namePlaceholder')"
              class="w-full px-3 py-2 border rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
              style="border-color: var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
            />
          </div>
          <div>
            <label class="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" v-model="form.permanent" class="rounded border-gray-300 text-emerald-500 focus:ring-emerald-500" />
              <span class="text-sm" style="color: var(--text-color);">{{ t('admin.apikey.neverExpires') }}</span>
            </label>
          </div>
          <div v-if="!form.permanent">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.apikey.expiresAt') }}</label>
            <DateTimePicker v-model="form.expiresAt" :placeholder="t('admin.apikey.expiresPlaceholder')" />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.apikey.scope') }}</label>
            <CustomSelect v-model="form.scope" :options="scopeOptions" />
            <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.apikey.scopeHint') }}
            </p>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.apikey.rateLimit') }}</label>
            <input
              v-model.number="form.rateLimitPerMinute"
              type="number"
              min="0"
              max="6000"
              :placeholder="t('admin.apikey.rateLimitPlaceholder')"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showCreate = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.common.cancel') }}</button>
          <button
            @click="handleCreate"
            :disabled="creating || !form.name.trim()"
            class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="(creating || !form.name.trim()) ? { opacity: 0.3 } : {}"
          >
            {{ creating ? t('admin.apikey.creating') : t('admin.apikey.create') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Key Reveal Overlay -->
    <div v-if="showKey && createdKey" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
      <div class="rounded-xl p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <div class="flex items-center gap-2 mb-4">
          <svg class="w-6 h-6 text-emerald-500" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <h3 class="text-lg font-bold" style="color: var(--text-color);">{{ t('admin.apikey.createdTitle') }}</h3>
        </div>

        <div class="rounded-lg p-3 text-sm mb-4" style="background-color: var(--button-hover-color); border: 1px solid var(--button-border-color); color: var(--text-color);">
          <strong>{{ t('admin.apikey.createdHintBold') }}</strong>{{ t('admin.apikey.createdHintRest') }}
        </div>

        <div class="rounded-lg p-4 mb-4" style="background-color: var(--button-hover-color); border: 1px solid var(--button-border-color);">
          <div class="text-xs mb-1" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.apikey.name') }}</div>
          <div class="font-medium" style="color: var(--text-color);">{{ createdKey.name }}</div>
        </div>

        <div class="rounded-lg p-4 mb-4" style="background-color: var(--button-hover-color); border: 1px solid var(--button-border-color);">
          <div class="text-xs mb-1" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.apikey.fullKey') }}</div>
          <code class="block text-sm rounded px-3 py-2 font-mono break-all select-all" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color); color: var(--text-color);">{{ createdKey.key }}</code>
        </div>

        <div class="flex justify-end gap-3">
          <button @click="copyKey" class="px-4 py-2 bg-indigo-500 hover:bg-indigo-600 text-white text-sm font-medium rounded-lg transition-colors">
            {{ copied ? t('admin.apikey.copied') : t('admin.apikey.copyKey') }}
          </button>
          <button @click="closeKeyOverlay" class="px-4 py-2 text-sm font-medium rounded-lg transition-colors" style="background-color: var(--button-hover-color); color: var(--text-color);">
            {{ t('admin.apikey.savedClose') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation -->
    <div v-if="showDelete" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showDelete = false">
      <div class="rounded-xl p-6 w-full max-w-sm mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-2" style="color: var(--text-color);">{{ t('admin.common.confirmDelete') }}</h3>
        <p class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.apikey.deleteConfirm', { name: deleteTarget?.name ?? '' }) }}</p>
        <p class="text-sm text-red-500">{{ t('admin.apikey.deleteWarning') }}</p>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showDelete = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.common.cancel') }}</button>
          <button
            @click="handleDelete"
            :disabled="deleting"
            class="px-4 py-2 bg-red-500 hover:bg-red-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="deleting ? { opacity: 0.3 } : {}"
          >
            {{ deleting ? t('admin.common.deleting') : t('admin.common.confirmDelete') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Edit Modal -->
    <div v-if="showEdit && editTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showEdit = false">
      <div class="rounded-xl p-6 w-full max-w-md mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.apikey.editTitle') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.apikey.name') }}</label>
            <input
              v-model="editName"
              type="text"
              class="w-full px-3 py-2 border rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
              style="border-color: var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
              autofocus
              @keydown.enter="handleEdit"
            />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.apikey.scope') }}</label>
            <CustomSelect v-model="editScope" :options="scopeOptions" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.apikey.rateLimit') }}</label>
            <input
              v-model.number="editRateLimit"
              type="number"
              min="0"
              max="6000"
              :placeholder="t('admin.apikey.rateLimitPlaceholder')"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showEdit = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.common.cancel') }}</button>
          <button
            @click="handleEdit"
            :disabled="saving || !editName.trim()"
            class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="(saving || !editName.trim()) ? { opacity: 0.3 } : {}"
          >
            {{ saving ? t('admin.common.saving') : t('admin.common.save') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
