<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { api } from '../../api/client'
import type { Incident, IncidentUpdate, Service } from '../../api/types'
import { useToast } from '../../composables/useToast'
import CustomSelect from './CustomSelect.vue'

const props = defineProps<{
  incident: Incident
  serviceName: string
  services: Service[]
}>()

const emit = defineEmits<{
  back: []
  updated: []
  deleted: []
  merge: [incident: Incident]
  split: [id: number]
}>()

const { show: toast } = useToast()

const statusLabel: Record<string, string> = {
  investigating: '调查中',
  identified: '已确认',
  monitoring: '监控中',
  resolved: '已解决',
}

const impactLabel: Record<string, string> = { critical: '严重', major: '较大', minor: '轻微' }

const impactClass = (s: string) => {
  switch (s) {
    case 'critical': return 'text-red-600 bg-red-50 border-red-100 dark:text-red-400 dark:bg-red-900/30 dark:border-red-800'
    case 'major': return 'text-orange-600 bg-orange-50 border-orange-100 dark:text-orange-400 dark:bg-orange-900/30 dark:border-orange-800'
    case 'minor': return 'text-yellow-600 bg-yellow-50 border-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30 dark:border-yellow-800'
    default: return 'text-gray-600 bg-gray-50 border-gray-100 dark:text-gray-400 dark:bg-gray-800 dark:border-gray-700'
  }
}

const statusClass = (s: string) => {
  switch (s) {
    case 'investigating': return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
    case 'identified': return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
    case 'monitoring': return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
    case 'resolved': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400'
    default: return 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-400'
  }
}

const dotClass = (s: string) => {
  switch (s) {
    case 'investigating': return 'bg-orange-500'
    case 'identified': return 'bg-orange-500'
    case 'monitoring': return 'bg-blue-500'
    case 'resolved': return 'bg-emerald-500'
    default: return 'bg-gray-400'
  }
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString('zh-CN')
}

const affectedServiceNames = computed(() => {
  if (!props.incident.affectedServices) return [props.serviceName].filter(Boolean)
  const ids = props.incident.affectedServices.split(',').map(Number).filter(Boolean)
  if (ids.length === 0) return [props.serviceName].filter(Boolean)
  return ids.map(id => {
    const svc = props.services.find(s => s.id === id)
    return svc ? svc.name : '未知'
  })
})

// Edit mode
const showEdit = ref(false)
const editForm = ref({ title: '', impact: 'minor' as string })

function openEdit() {
  editForm.value = { title: props.incident.title, impact: props.incident.impact }
  showEdit.value = true
}

async function saveEdit() {
  try {
    await api.updateIncident(props.incident.id, editForm.value)
    showEdit.value = false
    toast('更新成功', 'success')
    emit('updated')
  } catch (e: any) {
    toast(e.message || '更新失败')
  }
}

// Add update
const updateForm = ref({ status: 'investigating', content: '' })

async function submitUpdate() {
  if (!updateForm.value.content.trim()) return
  try {
    await api.createIncidentUpdate(props.incident.id, updateForm.value)
    updateForm.value = { status: updateForm.value.status, content: '' }
    toast('更新成功', 'success')
    emit('updated')
  } catch (e: any) {
    toast(e.message || '更新失败')
  }
}

// Delete
async function handleDelete() {
  if (!confirm('确定要删除此事件吗？')) return
  try {
    await api.deleteIncident(props.incident.id)
    toast('删除成功', 'success')
    emit('deleted')
  } catch (e: any) {
    toast(e.message || '删除失败')
  }
}

// Timeline update edit/delete
const editingUpdate = ref<IncidentUpdate | null>(null)
const editUpdateForm = ref({ status: 'investigating', content: '' })

function startEditUpdate(update: IncidentUpdate) {
  editingUpdate.value = update
  editUpdateForm.value = { status: update.status, content: update.content }
}

function cancelEditUpdate() {
  editingUpdate.value = null
}

async function saveEditUpdate() {
  if (!editingUpdate.value || !editUpdateForm.value.content.trim()) return
  try {
    await api.updateIncidentUpdate(props.incident.id, editingUpdate.value.id, editUpdateForm.value)
    editingUpdate.value = null
    toast('更新成功', 'success')
    emit('updated')
  } catch (e: any) {
    toast(e.message || '更新失败')
  }
}

async function deleteUpdate(updateId: number) {
  if (!confirm('确定要删除此更新记录吗？')) return
  try {
    await api.deleteIncidentUpdate(props.incident.id, updateId)
    toast('删除成功', 'success')
    emit('updated')
  } catch (e: any) {
    toast(e.message || '删除失败')
  }
}

// Reset update form status when incident changes
watch(() => props.incident.status, (s) => {
  updateForm.value.status = s
})

const reversedUpdates = computed(() => {
  if (!props.incident.updates) return []
  return [...props.incident.updates].reverse()
})
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <button @click="emit('back')" class="flex items-center gap-1.5 text-sm transition-colors" style="color: var(--text-color); opacity: 0.5;">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
        返回列表
      </button>
      <div class="flex items-center gap-2">
        <button @click="openEdit" class="op-btn op-btn-edit p-1.5 rounded-lg" title="编辑">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
        </button>
        <button @click="emit('merge', incident)" v-if="incident.status !== 'resolved'" class="op-btn p-1.5 rounded-lg" title="合并">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v12m0 0l-3-3m3 3l3-3M4 20h16" /></svg>
        </button>
        <button @click="handleDelete" class="op-btn op-btn-delete p-1.5 rounded-lg" title="删除">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
        </button>
      </div>
    </div>

    <!-- Info card -->
    <div class="rounded-xl p-5 mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="flex items-start justify-between mb-3">
        <h2 class="text-lg font-bold" style="color: var(--text-color);">{{ incident.title }}</h2>
        <span :class="['inline-flex items-center px-2 py-1 rounded text-xs font-medium border', impactClass(incident.impact)]">
          {{ impactLabel[incident.impact] || incident.impact }}
        </span>
      </div>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">当前状态</div>
          <span :class="['inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium', statusClass(incident.status)]">
            <span :class="['w-1.5 h-1.5 rounded-full', dotClass(incident.status)]" />
            {{ statusLabel[incident.status] || incident.status }}
          </span>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">关联服务</div>
          <div class="font-medium" style="color: var(--text-color);">
            <template v-if="affectedServiceNames.length > 0">
              <span v-for="(name, idx) in affectedServiceNames" :key="idx">{{ name }}<span v-if="idx < affectedServiceNames.length - 1">, </span></span>
            </template>
            <template v-else>-</template>
          </div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">创建时间</div>
          <div style="color: var(--text-color); opacity: 0.7;">{{ formatTime(incident.createdAt) }}</div>
        </div>
        <div>
          <div class="mb-1" style="color: var(--text-color); opacity: 0.4;">最后更新</div>
          <div style="color: var(--text-color); opacity: 0.7;">{{ formatTime(incident.updatedAt) }}</div>
        </div>
      </div>
    </div>

    <!-- Children (sub-events) -->
    <div v-if="incident.children && incident.children.length > 0" class="rounded-xl p-5 mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <div class="flex items-center justify-between mb-3">
        <h3 class="font-bold" style="color: var(--text-color);">子事件（{{ incident.children.length }}）</h3>
      </div>
      <div class="space-y-2">
        <div
          v-for="child in incident.children"
          :key="child.id"
          class="group flex items-center justify-between px-4 py-2.5 rounded-lg transition-colors hover:bg-[var(--button-hover-color)]"
        >
          <div class="flex items-center gap-3">
            <span class="text-sm font-medium" style="color: var(--text-color);">{{ child.title }}</span>
            <span :class="['inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border', impactClass(child.impact)]">
              {{ child.impact === 'critical' ? '严重' : child.impact === 'major' ? '较大' : '轻微' }}
            </span>
            <span class="text-xs px-2 py-0.5 rounded" :class="statusClass(child.status)">
              {{ statusLabel[child.status] || child.status }}
            </span>
          </div>
          <button
            @click="emit('split', child.id)"
            class="op-btn p-1.5 rounded-lg opacity-0 group-hover:opacity-100 transition-all"
            title="拆分出去"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 20V8m0 0l3 3m-3-3L9 11M4 4h16" />
            </svg>
          </button>
        </div>
      </div>
    </div>

    <!-- Timeline -->
    <div class="rounded-xl p-5" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <h3 class="font-bold mb-4" style="color: var(--text-color);">事件经过</h3>

      <div v-if="reversedUpdates.length > 0" class="relative">
        <div class="absolute left-[11px] top-2 bottom-2 w-0.5" style="background-color: var(--button-border-color);"></div>
        <div v-for="update in reversedUpdates" :key="update.id" class="group relative pl-8 pb-6 last:pb-0">
          <div :class="['absolute left-0 top-1 w-[22px] h-[22px] rounded-full border-2 flex items-center justify-center', dotClass(update.status)]" style="border-color: var(--bg-color);">
            <svg class="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="3"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" /></svg>
          </div>
          <!-- Normal display -->
          <template v-if="editingUpdate?.id !== update.id">
            <div class="flex items-center gap-2 mb-1">
              <span :class="['px-2 py-0.5 rounded text-xs font-medium', statusClass(update.status)]">
                {{ statusLabel[update.status] || update.status }}
              </span>
              <span class="text-xs" style="color: var(--text-color); opacity: 0.4;">{{ formatTime(update.createdAt) }}</span>
              <div class="flex items-center gap-1 ml-auto opacity-0 group-hover:opacity-100 transition-opacity">
                <button @click="startEditUpdate(update)" class="op-btn op-btn-edit p-0.5" title="编辑">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
                </button>
                <button @click="deleteUpdate(update.id)" class="op-btn op-btn-delete p-0.5" title="删除">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                </button>
              </div>
            </div>
            <div class="text-sm" style="color: var(--text-color); opacity: 0.7;">{{ update.content }}</div>
          </template>
          <!-- Inline edit form -->
          <template v-else>
            <div class="space-y-2 -mt-1">
              <div class="flex gap-2">
                <CustomSelect v-model="editUpdateForm.status" :options="[{ label: '调查中', value: 'investigating' }, { label: '已确认', value: 'identified' }, { label: '监控中', value: 'monitoring' }, { label: '已解决', value: 'resolved' }]" />
                <input v-model="editUpdateForm.content" class="flex-1 px-2 py-1 rounded text-xs focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
              </div>
              <div class="flex gap-2">
                <button @click="saveEditUpdate" class="px-2 py-1 bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-medium rounded transition-colors">保存</button>
                <button @click="cancelEditUpdate" class="btn-cancel px-2 py-1 text-xs rounded">取消</button>
              </div>
            </div>
          </template>
        </div>
      </div>
      <div v-else class="text-sm py-4 text-center" style="color: var(--text-color); opacity: 0.4;">暂无更新记录</div>

      <!-- Add update form -->
      <div class="mt-6 pt-4" style="border-top: 1px solid var(--button-border-color);">
        <div class="text-sm font-medium mb-3" style="color: var(--text-color); opacity: 0.7;">添加更新</div>
        <div class="flex gap-3">
          <CustomSelect v-model="updateForm.status" :options="[{ label: '调查中', value: 'investigating' }, { label: '已确认', value: 'identified' }, { label: '监控中', value: 'monitoring' }, { label: '已解决', value: 'resolved' }]" />
          <input
            v-model="updateForm.content"
            @keyup.enter="submitUpdate"
            placeholder="输入更新内容..."
            class="flex-1 px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
            style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
          />
          <button @click="submitUpdate" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors flex-shrink-0">
            提交
          </button>
        </div>
      </div>
    </div>

    <!-- Edit modal -->
    <div v-if="showEdit" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showEdit = false">
      <div class="rounded-xl p-4 md:p-6 w-full max-w-lg mx-4" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">编辑事件</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">标题 *</label>
            <input v-model="editForm.title" class="w-full px-3 py-2 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color); opacity: 0.7;">影响等级 *</label>
            <CustomSelect v-model="editForm.impact" :options="[{ label: '轻微', value: 'minor' }, { label: '较大', value: 'major' }, { label: '严重', value: 'critical' }]" />
          </div>
        </div>
        <div class="flex justify-end gap-3 mt-6">
          <button @click="showEdit = false" class="btn-cancel px-4 py-2 text-sm rounded-lg">取消</button>
          <button @click="saveEdit" class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
