<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../../api/client'
import { useToast } from '../../composables/useToast'
import { useI18n } from '../../composables/useI18n'

interface Subscriber {
  id: number
  email: string
  verified: boolean
  createdAt: string
  updatedAt: string
}

const { show: toast } = useToast()
const { t, formatDateTime } = useI18n()
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
    toast(e.message || t('admin.common.loadFailed'))
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
    toast(e.message || t('admin.subscriber.loadSettingsFailed'))
  } finally {
    settingsLoading.value = false
  }
}

async function saveSubSettings() {
  settingsSaving.value = true
  try {
    await api.updateSettings(subSettings.value)
    toast(t('admin.subscriber.settingsSaved'), 'success')
  } catch (e: any) {
    toast(e.message || t('admin.common.saveFailed'))
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
    toast(t('admin.subscriber.deleted'), 'success')
    showDelete.value = false
    deleteTarget.value = null
    load()
  } catch (e: any) {
    toast(e.message || t('admin.common.deleteFailed'))
  } finally {
    deleting.value = false
  }
}

function formatDate(iso: string): string {
  if (!iso) return '-'
  return formatDateTime(iso)
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
      <h3 class="text-sm font-bold mb-4" style="color: var(--text-color);">{{ t('admin.subscriber.settingsTitle') }}</h3>
      <div v-if="settingsLoading" class="text-sm" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>
      <div v-else class="space-y-4 max-w-sm">
        <!-- Email -->
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm font-medium" style="color: var(--text-color);">{{ t('admin.subscriber.email') }}</div>
            <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.subscriber.emailHint') }}</div>
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
            <div class="text-sm font-medium" style="color: var(--text-color);">{{ t('admin.subscriber.rss') }}</div>
            <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.subscriber.rssHint') }}</div>
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
            <div class="text-sm font-medium" style="color: var(--text-color);">{{ t('admin.subscriber.atom') }}</div>
            <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.subscriber.atomHint') }}</div>
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
            {{ settingsSaving ? t('admin.common.saving') : t('admin.common.save') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Subscriber List -->
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ t('admin.subscriber.listTitle') }}</h2>
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <div v-else-if="subscribers.length === 0" class="rounded-xl p-12 text-center" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <svg class="w-16 h-16 mx-auto mb-4" style="color: var(--text-color); opacity: 0.3;" fill="none" stroke="currentColor" stroke-width="1" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
      </svg>
      <p class="text-base font-medium" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.subscriber.emptyTitle') }}</p>
      <p class="text-sm mt-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.subscriber.emptyHint') }}</p>
    </div>

    <div v-else class="rounded-xl overflow-hidden" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <table class="w-full text-left text-sm">
        <thead class="text-xs" style="background-color: var(--button-hover-color); opacity: 0.5; border-bottom: 1px solid var(--button-border-color); color: var(--text-color);">
          <tr>
            <th class="px-6 py-3 font-medium">{{ t('admin.subscriber.colEmail') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.common.colStatus') }}</th>
            <th class="px-6 py-3 font-medium">{{ t('admin.subscriber.colSubscribedAt') }}</th>
            <th class="px-6 py-3 font-medium text-right">{{ t('admin.common.colActions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr v-for="s in subscribers" :key="s.id">
            <td class="px-6 py-4 font-medium" style="color: var(--text-color);">{{ s.email }}</td>
            <td class="px-6 py-4">
              <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', s.verified ? 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15' : '']" :style="!s.verified ? { color: 'var(--text-color)', opacity: 0.5, backgroundColor: 'var(--button-hover-color)', borderColor: 'var(--button-border-color)' } : {}">
                {{ s.verified ? t('admin.subscriber.verified') : t('admin.subscriber.unverified') }}
              </span>
            </td>
            <td class="px-6 py-4 text-xs" style="color: var(--text-color); opacity: 0.5;">{{ formatDate(s.createdAt) }}</td>
            <td class="px-6 py-4 text-right">
              <button @click="confirmDelete(s)" class="text-red-500 hover:text-red-600 transition-colors" :title="t('admin.common.delete')">
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
        <h3 class="text-lg font-bold mb-2" style="color: var(--text-color);">{{ t('admin.common.confirmDelete') }}</h3>
        <p class="text-sm mb-1" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.subscriber.deleteConfirm', { email: deleteTarget?.email ?? '' }) }}</p>
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
  </div>
</template>
