<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { ServiceFolder, FolderSummary, Service, ServiceSummary } from '../../api/types'
import { useToast } from '../../composables/useToast'
import { useI18n } from '../../composables/useI18n'

const { show: toast } = useToast()
const { t } = useI18n()

const folders = ref<ServiceFolder[]>([])
const summaries = ref<FolderSummary[]>([])
const services = ref<(Service & { uptime: number; latency: number })[]>([])
const loading = ref(true)
const saving = ref(false)
const showForm = ref(false)
const editing = ref<ServiceFolder | null>(null)
/** 展开的分组 ID：展开后展示该分组内各服务的明细，隐藏融合信息 */
const expanded = ref<number[]>([])

const form = ref({ name: '', description: '', showOnHomepage: true })

/** 融合结果按分组 ID 索引，便于列表里对照展示 */
const summaryById = computed(() => {
  const map: Record<number, FolderSummary> = {}
  for (const s of summaries.value) map[s.id] = s
  return map
})

/** 每个分组内的服务（未分组服务不在此列） */
const servicesByFolder = computed(() => {
  const map: Record<number, (Service & { uptime: number; latency: number })[]> = {}
  for (const svc of services.value) {
    if (!svc.folderId) continue
    if (!map[svc.folderId]) map[svc.folderId] = []
    map[svc.folderId].push(svc)
  }
  return map
})

/** 未分组的服务，表单里作为「未分组」选项的依据 */
const ungroupedCount = computed(() => services.value.filter(s => !s.folderId).length)

async function load() {
  loading.value = true
  try {
    const [folderRes, summaryRes, serviceRes] = await Promise.all([
      api.getServiceFolders(),
      api.getFolderSummaries(),
      api.getAdminServices(),
    ])
    folders.value = folderRes.data
    summaries.value = summaryRes.data
    services.value = serviceRes.data
  } catch (e: any) {
    toast(e.message || t('admin.folder.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  form.value = { name: '', description: '', showOnHomepage: true }
  showForm.value = true
}

function openEdit(folder: ServiceFolder) {
  editing.value = folder
  form.value = {
    name: folder.name,
    description: folder.description || '',
    showOnHomepage: folder.showOnHomepage,
  }
  showForm.value = true
}

async function save() {
  if (!form.value.name.trim()) {
    toast(t('admin.folder.nameRequired'), 'error')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await api.updateServiceFolder(editing.value.id, form.value)
    } else {
      await api.createServiceFolder(form.value)
    }
    showForm.value = false
    toast(editing.value ? t('admin.folder.updated') : t('admin.folder.created'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.folder.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function remove(folder: ServiceFolder) {
  if (!confirm(t('admin.folder.deleteConfirm', { name: folder.name }))) return
  try {
    await api.deleteServiceFolder(folder.id)
    toast(t('admin.folder.deleted'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.folder.deleteFailed'))
  }
}

/** 展开 / 收起分组明细：展开时隐藏融合信息，只展示各服务 */
function toggleExpand(id: number) {
  const idx = expanded.value.indexOf(id)
  if (idx >= 0) expanded.value.splice(idx, 1)
  else expanded.value.push(id)
}

function isExpanded(id: number): boolean {
  return expanded.value.includes(id)
}

/** 把服务移入 / 移出分组 */
async function moveService(serviceId: number, folderId: number | null) {
  try {
    await api.assignServiceFolder(serviceId, folderId)
    toast(folderId ? t('admin.folder.serviceMoved') : t('admin.folder.serviceRemoved'), 'success')
    load()
  } catch (e: any) {
    toast(e.message || t('admin.folder.moveFailed'))
  }
}

const statusClass = (s: string) => {
  switch (s) {
    case 'operational': return 'text-emerald-600 bg-emerald-50 border-emerald-100 dark:text-emerald-400 dark:bg-emerald-500/15 dark:border-emerald-800'
    case 'degraded': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    case 'outage': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    default: return ''
  }
}

const statusText = (s: string) => t(`status.${s}`)

onMounted(load)
</script>

<template>
  <div>
    <div class="flex justify-between items-center mb-4">
      <div>
        <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ t('admin.folder.title') }}</h2>
        <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.45;">
          {{ t('admin.folder.subtitle') }}
        </p>
      </div>
      <button @click="openCreate" class="bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium px-4 py-2 rounded-lg flex items-center gap-1 transition-colors">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        {{ t('admin.folder.create') }}
      </button>
    </div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <div v-else class="space-y-4">
      <div
        v-for="folder in folders"
        :key="folder.id"
        class="rounded-xl"
        style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);"
      >
        <!-- 分组头部：融合信息 + 展开按钮 -->
        <div class="flex items-center justify-between gap-3 px-6 py-4">
          <div class="flex items-center gap-3 min-w-0">
            <button
              @click="toggleExpand(folder.id)"
              class="p-1 rounded transition-transform"
              :class="isExpanded(folder.id) ? 'rotate-90' : ''"
              style="color: var(--text-color); opacity: 0.5;"
              :title="isExpanded(folder.id) ? t('admin.folder.collapse') : t('admin.folder.expand')"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" /></svg>
            </button>
            <svg class="w-5 h-5 flex-shrink-0" style="color: var(--text-color); opacity: 0.45;" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
            </svg>
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <h3 class="font-bold truncate" style="color: var(--text-color);">{{ folder.name }}</h3>
                <span v-if="!folder.showOnHomepage" class="text-[10px] px-1.5 py-0.5 rounded" style="background-color: var(--button-hover-color); color: var(--text-color); opacity: 0.6;">
                  {{ t('admin.folder.hidden') }}
                </span>
              </div>
              <p v-if="folder.description" class="text-xs truncate" style="color: var(--text-color); opacity: 0.4;">{{ folder.description }}</p>
            </div>
          </div>

          <!-- 融合信息：展开后隐藏，改为展示各服务明细 -->
          <div v-if="!isExpanded(folder.id) && summaryById[folder.id]" class="hidden md:flex items-center gap-5 flex-shrink-0">
            <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium border', statusClass(summaryById[folder.id].status)]">
              <span class="w-1.5 h-1.5 rounded-full" :class="summaryById[folder.id].status === 'operational' ? 'bg-emerald-500' : summaryById[folder.id].status === 'outage' ? 'bg-red-500' : 'bg-yellow-500'" />
              {{ statusText(summaryById[folder.id].status) }}
            </span>
            <span class="text-sm" style="color: var(--text-color); opacity: 0.6;">{{ summaryById[folder.id].uptime.toFixed(2) }}%</span>
            <span class="text-sm" style="color: var(--text-color); opacity: 0.6;">{{ summaryById[folder.id].latency }}ms</span>
          </div>

          <div class="flex items-center gap-1 flex-shrink-0">
            <span class="text-xs mr-2" style="color: var(--text-color); opacity: 0.4;">
              {{ t('admin.folder.serviceCount', { n: servicesByFolder[folder.id]?.length || 0 }) }}
            </span>
            <button @click="openEdit(folder)" class="op-btn op-btn-edit p-1.5 rounded-lg" :title="t('admin.common.edit')">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
            </button>
            <button @click="remove(folder)" class="op-btn op-btn-delete p-1.5 rounded-lg" :title="t('admin.common.delete')">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
            </button>
          </div>
        </div>

        <!-- 展开：各服务明细（融合信息消失） -->
        <div v-if="isExpanded(folder.id)" style="border-top: 1px solid var(--button-border-color);">
          <table v-if="servicesByFolder[folder.id]?.length" class="w-full text-left text-sm">
            <thead class="text-xs" style="color: var(--text-color); opacity: 0.4; border-bottom: 1px solid var(--button-border-color);">
              <tr>
                <th class="px-6 py-2 font-medium">{{ t('admin.folder.serviceName') }}</th>
                <th class="px-6 py-2 font-medium">{{ t('admin.folder.serviceStatus') }}</th>
                <th class="px-6 py-2 font-medium">{{ t('admin.folder.serviceUptime') }}</th>
                <th class="px-6 py-2 font-medium">{{ t('admin.folder.serviceLatency') }}</th>
                <th class="px-6 py-2 font-medium text-right">{{ t('admin.folder.serviceActions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y">
              <tr v-for="svc in servicesByFolder[folder.id]" :key="svc.id" class="hover:bg-[var(--button-hover-color)]">
                <td class="px-6 py-3 font-medium" style="color: var(--text-color);">{{ svc.name }}</td>
                <td class="px-6 py-3">
                  <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium border', statusClass(svc.status)]">
                    {{ statusText(svc.status) }}
                  </span>
                </td>
                <td class="px-6 py-3" style="color: var(--text-color); opacity: 0.5;">{{ svc.uptime.toFixed(2) }}%</td>
                <td class="px-6 py-3" style="color: var(--text-color); opacity: 0.5;">{{ svc.latency }}ms</td>
                <td class="px-6 py-3 text-right">
                  <button @click="moveService(svc.id, null)" class="text-xs px-2 py-1 rounded transition-colors" style="color: var(--text-color); opacity: 0.6; border: 1px solid var(--button-border-color);">
                    {{ t('admin.folder.removeFromFolder') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-else class="px-6 py-6 text-sm text-center" style="color: var(--text-color); opacity: 0.4;">
            {{ t('admin.folder.emptyFolder') }}
          </div>
        </div>
      </div>

      <!-- 空状态 -->
      <div v-if="folders.length === 0" class="text-center py-12 rounded-xl" style="border: 1px dashed var(--button-border-color); color: var(--text-color); opacity: 0.45;">
        <p>{{ t('admin.folder.empty') }}</p>
        <p class="text-sm mt-1">{{ t('admin.folder.emptyHint') }}</p>
      </div>

      <!-- 未分组服务 -->
      <div v-if="ungroupedCount > 0" class="rounded-xl" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <div class="px-6 py-4">
          <h3 class="font-bold text-sm" style="color: var(--text-color);">{{ t('admin.folder.ungrouped', { n: ungroupedCount }) }}</h3>
          <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.folder.ungroupedHint') }}</p>
        </div>
        <div class="px-6 pb-4 space-y-2">
          <div
            v-for="svc in services.filter(s => !s.folderId)"
            :key="svc.id"
            class="flex items-center justify-between gap-3 text-sm"
          >
            <span style="color: var(--text-color); opacity: 0.75;">{{ svc.name }}</span>
            <div class="flex items-center gap-2">
              <span class="text-xs" style="color: var(--text-color); opacity: 0.4;">{{ svc.uptime.toFixed(2) }}%</span>
              <select
                class="text-xs px-2 py-1 rounded"
                style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
                @change="(e: Event) => {
                  const v = (e.target as HTMLSelectElement).value
                  if (v) moveService(svc.id, Number(v))
                }"
              >
                <option value="">{{ t('admin.folder.chooseFolder') }}</option>
                <option v-for="f in folders" :key="f.id" :value="f.id">{{ f.name }}</option>
              </select>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 分组表单 -->
    <div v-if="showForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showForm = false">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ editing ? t('admin.folder.editTitle') : t('admin.folder.createTitle') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.folder.nameLabel') }}</label>
            <input v-model="form.name" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">{{ t('admin.folder.descriptionLabel') }}</label>
            <input v-model="form.description" class="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div class="flex items-center justify-between pt-2">
            <div>
              <div class="text-sm font-medium" style="color: var(--text-color);">{{ t('admin.folder.showOnHomepage') }}</div>
              <div class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.folder.showOnHomepageHint') }}</div>
            </div>
            <label class="relative inline-flex items-center cursor-pointer flex-shrink-0">
              <input type="checkbox" v-model="form.showOnHomepage" class="sr-only" />
              <span
                class="flex items-center rounded-full transition-colors duration-200"
                :class="form.showOnHomepage ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'"
                style="width: 40px; height: 22px; flex-shrink: 0;"
              >
                <span
                  class="bg-white rounded-full shadow transition-transform duration-200"
                  :class="form.showOnHomepage ? 'translate-x-[21px]' : 'translate-x-[3px]'"
                  style="width: 16px; height: 16px;"
                ></span>
              </span>
            </label>
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showForm = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">{{ t('admin.common.cancel') }}</button>
          <button @click="save" :disabled="saving" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors disabled:opacity-40">
            {{ saving ? t('admin.common.saving') : t('admin.common.save') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
