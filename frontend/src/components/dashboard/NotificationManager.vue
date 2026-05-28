<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import type { Service } from '../../api/types'
import { useToast } from '../../composables/useToast'
import CustomSelect from './CustomSelect.vue'

const { show: toast } = useToast()

const settings = ref<Record<string, string>>({})
const services = ref<(Service & { uptime: number; latency: number })[]>([])
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)

// Test email
const testTo = ref('')

// Service notification select
const notifyServiceIds = ref<number[]>([])
const serviceSearch = ref('')
const showServiceDropdown = ref(false)

const filteredServices = computed(() => {
  if (!serviceSearch.value) return services.value
  const q = serviceSearch.value.toLowerCase()
  return services.value.filter(s => s.name.toLowerCase().includes(q))
})

const smtpFields: { key: string; label: string; type: string; placeholder: string }[] = [
  { key: 'smtp_host', label: 'SMTP 服务器', type: 'text', placeholder: 'smtp.example.com' },
  { key: 'smtp_port', label: 'SMTP 端口', type: 'text', placeholder: '587' },
  { key: 'smtp_user', label: 'SMTP 用户名', type: 'text', placeholder: 'user@example.com' },
  { key: 'smtp_pass', label: 'SMTP 密码', type: 'password', placeholder: '输入密码' },
]

async function load() {
  loading.value = true
  try {
    const [settingsRes, servicesRes] = await Promise.all([
      api.getSettings(),
      api.getAdminServices(),
    ])
    settings.value = settingsRes.data
    services.value = servicesRes.data

    // Parse notify_services
    const raw = settingsRes.data['notify_services'] || ''
    notifyServiceIds.value = raw ? raw.split(',').map(Number) : []
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function save() {
  // Serialize notify_services before saving
  settings.value['notify_services'] = notifyServiceIds.value.join(',')

  saving.value = true
  try {
    await api.updateSettings(settings.value)
    toast('设置已保存', 'success')
  } catch (e: any) {
    toast(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

function toggleNotifyService(svc: { id: number }) {
  const idx = notifyServiceIds.value.indexOf(svc.id)
  if (idx >= 0) {
    notifyServiceIds.value.splice(idx, 1)
  } else {
    notifyServiceIds.value.push(svc.id)
  }
  serviceSearch.value = ''
}

function delayBlur() {
  setTimeout(() => { showServiceDropdown.value = false }, 200)
}

async function testEmail() {
  if (!testTo.value) return
  testing.value = true
  try {
    await api.testEmail(testTo.value)
    toast('测试邮件发送成功，请检查收件箱', 'success')
  } catch (e: any) {
    toast(e.message || '发送失败')
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>

    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">加载中...</div>

    <template v-else>
      <!-- SMTP Config -->
      <div class="rounded-xl p-6 mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">SMTP 邮件配置</h3>
        <div class="space-y-4 max-w-md">
          <div v-for="field in smtpFields" :key="field.key">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ field.label }}</label>
            <input
              v-model="settings[field.key]"
              :type="field.type"
              :placeholder="field.placeholder"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">安全连接</label>
            <CustomSelect v-model="settings['smtp_encryption']" :options="[{ label: '无加密', value: '' }, { label: 'STARTTLS（端口 587）', value: 'starttls' }, { label: 'SSL/TLS（端口 465）', value: 'tls' }]" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">启用邮件通知</label>
            <button
              type="button"
              @click="settings['email_enabled'] = settings['email_enabled'] === 'true' ? 'false' : 'true'"
              class="relative inline-flex h-6 w-11 items-center rounded-full flex-shrink-0 transition-colors"
              :class="settings['email_enabled'] === 'true' ? 'bg-emerald-500' : ''"
              :style="settings['email_enabled'] !== 'true' ? { opacity: 0.3 } : {}"
            >
              <span
                class="inline-block h-4 w-4 transform rounded-full bg-white transition"
                :class="settings['email_enabled'] === 'true' ? 'translate-x-[22px]' : 'translate-x-[3px]'"
              />
            </button>
          </div>
          <button
            @click="save"
            :disabled="saving"
            class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            :style="saving ? { opacity: 0.3 } : {}"
          >
            {{ saving ? '保存中...' : '保存配置' }}
          </button>
        </div>
      </div>

      <!-- Service Notification Config -->
      <div class="rounded-xl p-6 mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">服务异常通知</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">选择需要监控的服务，当服务出现异常时将自动发送邮件通知。</p>

        <div class="space-y-4 max-w-md">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">监控服务</label>
            <div class="relative">
              <div class="w-full px-3 py-2 border rounded-lg text-sm cursor-text focus-within:border-emerald-500 min-h-[38px] flex flex-wrap gap-1" style="border-color: var(--button-border-color); background-color: var(--bg-color);">
                <span
                  v-for="id in notifyServiceIds" :key="id"
                  class="inline-flex items-center gap-1 px-2 py-0.5 bg-emerald-50 dark:bg-emerald-500/15 text-emerald-700 dark:text-emerald-400 rounded text-xs"
                >
                  {{ services.find(s => s.id === id)?.name || id }}
                  <button @click="toggleNotifyService({ id })" class="ml-0.5 hover:text-emerald-900 dark:hover:text-emerald-300">&times;</button>
                </span>
                <input
                  v-model="serviceSearch"
                  @focus="showServiceDropdown = true"
                  @blur="delayBlur"
                  type="text"
                  placeholder="搜索服务..."
                  class="border-0 outline-none text-sm flex-1 min-w-[80px] bg-transparent"
                  style="color: var(--text-color);"
                />
              </div>
              <div
                v-if="showServiceDropdown"
                class="absolute z-10 mt-1 w-full rounded-lg shadow-lg max-h-48 overflow-y-auto thin-scroll p-1"
                style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);"
              >
                <div
                  v-for="svc in filteredServices" :key="svc.id"
                  @mousedown.prevent="toggleNotifyService(svc)"
                  class="svc-opt flex items-center justify-between"
                  :class="{ active: notifyServiceIds.includes(svc.id) }"
                >
                  <span>{{ svc.name }}</span>
                  <span v-if="notifyServiceIds.includes(svc.id)" class="text-emerald-500">✓</span>
                </div>
                <div v-if="filteredServices.length === 0" class="px-3 py-2 text-sm" style="color: var(--text-color); opacity: 0.4;">无匹配服务</div>
              </div>
            </div>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">通知邮箱</label>
            <p class="text-xs mb-1" style="color: var(--text-color); opacity: 0.4;">多个邮箱用逗号分隔</p>
            <input
              v-model="settings['notify_emails']"
              type="text"
              placeholder="admin@example.com,backup@example.com"
              class="w-full px-3 py-2 rounded-lg text-sm input-field"
            />
          </div>

          <div>
            <button
              @click="save"
              :disabled="saving"
              class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
              :style="saving ? { opacity: 0.3 } : {}"
            >
              {{ saving ? '保存中...' : '保存配置' }}
            </button>
          </div>
        </div>
      </div>

      <!-- Test Email -->
      <div class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">测试邮件发送</h3>
        <p class="text-sm mb-4" style="color: var(--text-color); opacity: 0.5;">向指定地址发送一封测试邮件，验证 SMTP 配置是否有效。</p>
        <div class="flex items-end gap-3 max-w-md">
          <div class="flex-1">
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">收件地址</label>
            <input v-model="testTo" type="email" placeholder="test@example.com" class="w-full px-3 py-2 rounded-lg text-sm input-field" />
          </div>
          <button
            @click="testEmail"
            :disabled="!testTo || testing"
            class="px-6 py-2 bg-blue-500 hover:bg-blue-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors whitespace-nowrap"
            :style="(!testTo || testing) ? { opacity: 0.3 } : {}"
          >
            {{ testing ? '发送中...' : '发送测试' }}
          </button>
        </div>
      </div>
    </template>
  </div>
</template>
