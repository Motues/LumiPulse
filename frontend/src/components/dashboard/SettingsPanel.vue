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

        <div class="pt-2">
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
  </div>
</template>
