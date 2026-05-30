<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../../api/client'
import { siteName, siteIcon, updateSiteTitle, updateSiteIcon } from '../../composables/useSiteConfig'
import { useToast } from '../../composables/useToast'

const { show: toast } = useToast()

const settings = ref<Record<string, string>>({})
const loading = ref(true)
const saving = ref(false)

const iconPreview = ref('')

async function load() {
  loading.value = true
  try {
    const res = await api.getSettings()
    settings.value = res.data
    iconPreview.value = res.data['site_icon'] || ''
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await api.updateSettings(settings.value)

    // Sync to live site config
    if (settings.value['site_name']) {
      updateSiteTitle(settings.value['site_name'])
    }
    if (settings.value['site_icon']) {
      updateSiteIcon(settings.value['site_icon'])
    }

    toast('设置已保存', 'success')
  } catch (e: any) {
    toast(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

function onIconFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  const reader = new FileReader()
  reader.onload = () => {
    const dataUrl = reader.result as string
    settings.value['site_icon'] = dataUrl
    iconPreview.value = dataUrl
  }
  reader.readAsDataURL(file)
}

function resetIcon() {
  settings.value['site_icon'] = ''
  iconPreview.value = ''
}

const importMsg = ref('')
const importMsgType = ref('success')

async function handleExport() {
  try {
    const token = localStorage.getItem('token')
    const resp = await fetch('/api/v1/admin/export', {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}))
      throw new Error(err.message || '导出失败')
    }
    const blob = await resp.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    const disposition = resp.headers.get('Content-Disposition') || ''
    const match = disposition.match(/filename=(.+)/)
    a.download = match?.[1] || `lumipulse-export-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    toast('导出成功', 'success')
  } catch (e: any) {
    toast(e.message || '导出失败')
  }
}

async function handleImportFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  try {
    if (!confirm('导入将覆盖现有所有服务、事件、维护计划和系统设置，此操作不可撤销。\n\n确定要继续吗？')) {
      importMsg.value = '已取消导入'
      importMsgType.value = 'error'
      return
    }

    const text = await file.text()
    const data = JSON.parse(text)

    const res = await api.request<{ code: number; message: string }>('POST', '/admin/import?confirm=true', data, true)
    importMsg.value = res.message || '导入成功'
    importMsgType.value = 'success'
    toast('导入成功，请刷新页面查看', 'success')
  } catch (e: any) {
    importMsg.value = e.message || '导入失败，请检查文件格式'
    importMsgType.value = 'error'
  } finally {
    ;(input as any).value = ''
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">加载中...</div>

    <div v-else class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <h2 class="text-lg font-bold mb-6" style="color: var(--text-color);">系统设置</h2>
      <div class="space-y-6 max-w-md">
        <!-- Site Name -->
        <div>
          <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">系统名称</label>
          <input
            v-model="settings['site_name']"
            type="text"
            placeholder="LumiPulse"
            class="w-full px-3 py-2 border rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
            style="border-color: var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
          />
          <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">显示在浏览器标签栏和页面各处</p>
        </div>

        <!-- Site Icon -->
        <div>
          <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">系统图标</label>
          <div class="flex items-center gap-4">
            <div
              class="w-12 h-12 rounded-xl border flex items-center justify-center overflow-hidden"
              style="border-color: var(--button-border-color); background-color: var(--bg-color);"
            >
              <img v-if="iconPreview" :src="iconPreview" class="w-8 h-8 object-contain" />
              <svg v-else class="w-6 h-6" style="color: var(--text-color); opacity: 0.3;" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
            </div>
            <label class="px-4 py-2 border rounded-lg text-sm cursor-pointer transition-colors hover:opacity-100" style="background-color: var(--bg-color); border-color: var(--button-border-color); color: var(--text-color); opacity: 0.6;" @mouseenter="($event.target as HTMLElement).style.backgroundColor = 'var(--button-hover-color)'" @mouseleave="($event.target as HTMLElement).style.backgroundColor = 'var(--bg-color)'">
              上传图标
              <input type="file" accept="image/svg+xml,image/png,image/jpeg" class="hidden" @change="onIconFileChange" />
            </label>
            <button v-if="iconPreview" @click="resetIcon" class="text-sm text-red-500 hover:text-red-700">恢复默认</button>
          </div>
          <p class="text-xs mt-1" style="color: var(--text-color); opacity: 0.4;">支持 SVG、PNG、JPG 格式</p>
        </div>

        <!-- Show Admin Footer Button -->
        <div class="flex items-center justify-between pt-4">
          <div>
            <label class="text-sm font-medium" style="color: var(--text-color);">显示管理后台入口</label>
            <p class="text-xs mt-0.5" style="color: var(--text-color); opacity: 0.4;">在公开页面页脚显示"管理后台"链接</p>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" :checked="settings['show_admin_footer_button'] !== 'false'" @change="settings['show_admin_footer_button'] = ($event.target as HTMLInputElement).checked ? 'true' : 'false'" class="sr-only" />
            <span
              class="flex items-center rounded-full transition-colors duration-200"
              :class="settings['show_admin_footer_button'] !== 'false' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'"
              style="width: 40px; height: 22px; flex-shrink: 0;"
            >
              <span
                class="bg-white rounded-full shadow transition-transform duration-200"
                :class="settings['show_admin_footer_button'] !== 'false' ? 'translate-x-[19px]' : 'translate-x-[3px]'"
                style="width: 16px; height: 16px;"
              ></span>
            </span>
          </label>
        </div>

        <!-- Custom Footer HTML -->
        <div class="pt-4">
          <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">自定义页脚内容</label>
          <p class="text-xs mb-2" style="color: var(--text-color); opacity: 0.4;">设置后替代默认页脚，支持 HTML 内容</p>
          <textarea
            v-model="settings['custom_footer']"
            rows="4"
            class="w-full px-3 py-2 border rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 font-mono"
            style="border-color: var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);"
            placeholder='<a href="https://example.com">我的站点</a>'
          ></textarea>
        </div>

        <div class="pt-4">
          <button
            @click="save"
            :disabled="saving"
            class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="saving ? { opacity: 0.3 } : {}"
          >
            {{ saving ? '保存中...' : '保存设置' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Data Export/Import -->
    <div class="rounded-xl p-6 mt-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
      <h2 class="text-lg font-bold mb-2" style="color: var(--text-color);">数据导入导出</h2>
      <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">导出或导入服务、事件、维护计划和系统设置（不含日志和探测记录）。</p>
      <div class="flex gap-3">
        <button
          @click="handleExport"
          class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 text-white text-sm font-medium rounded-lg transition-colors"
        >
          导出数据
        </button>
        <label class="px-4 py-2 border rounded-lg text-sm font-medium cursor-pointer transition-colors hover:bg-[var(--button-hover-color)]" style="border-color: var(--button-border-color); color: var(--text-color);">
          导入数据
          <input type="file" accept=".json" class="hidden" @change="handleImportFile" />
        </label>
      </div>
      <p v-if="importMsg" :class="['text-sm mt-3', importMsgType === 'success' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-500']">{{ importMsg }}</p>
    </div>
  </div>
</template>
