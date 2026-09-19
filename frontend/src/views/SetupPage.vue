<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'
import { useAuth } from '../stores/auth'
import { siteName, siteIcon } from '../composables/useSiteConfig'
import { useToast } from '../composables/useToast'
import { useI18n } from '../composables/useI18n'
import Toast from '../components/dashboard/Toast.vue'

const router = useRouter()
const { setToken } = useAuth()
const { show: toast } = useToast()
const { t } = useI18n()

const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const touched = ref({ username: false, password: false, confirm: false })

const usernameError = computed(() => {
  if (!touched.value.username) return ''
  if (!username.value) return t('admin.setup.usernameRequired')
  if (username.value.length < 2) return t('admin.user.usernameMin')
  return ''
})

const passwordError = computed(() => {
  if (!touched.value.password) return ''
  if (!password.value) return t('admin.setup.passwordRequired')
  if (password.value.length < 4) return t('admin.user.passwordMin')
  return ''
})

const confirmError = computed(() => {
  if (!touched.value.confirm) return ''
  if (!confirmPassword.value) return t('admin.setup.confirmRequired')
  if (password.value !== confirmPassword.value) return t('admin.user.passwordMismatch')
  return ''
})

const canSubmit = computed(() => {
  return username.value.length >= 2 && password.value.length >= 4 && password.value === confirmPassword.value
})

function fieldClass(err: string) {
  if (!err) return 'focus:border-emerald-500 focus:ring-emerald-500'
  return 'focus:border-red-500 focus:ring-red-500'
}

function fieldStyle(err: string) {
  return {
    border: `1px solid ${err ? '#ef4444' : 'var(--button-border-color)'}`,
    backgroundColor: 'var(--bg-color)',
    color: 'var(--text-color)',
  }
}

async function handleSetup() {
  touched.value = { username: true, password: true, confirm: true }
  if (usernameError.value || passwordError.value || confirmError.value) return

  loading.value = true
  try {
    await api.setup(username.value, password.value)
    setToken('')
    toast(t('admin.setup.success'), 'success')
    setTimeout(() => router.push('/login'), 2000)
  } catch (e: any) {
    toast(e.message || t('admin.setup.failed'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center px-4" style="background-color: var(--bg-color);">
    <div class="w-full max-w-sm">
      <div class="text-center mb-8">
        <div class="flex items-center justify-center gap-2 mb-2">
          <img v-if="siteIcon" :src="siteIcon" class="w-8 h-8 object-contain" />
          <img v-else src="/assets/logo.svg" class="w-8 h-8 object-contain" />
          <span class="text-2xl font-bold tracking-tight" style="color: var(--text-color);">{{ siteName }}</span>
        </div>
        <p class="text-sm" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.setup.subtitle') }}</p>
      </div>

      <div class="rounded-xl p-6" style="border: 1px solid var(--button-border-color);">
        <form @submit.prevent="handleSetup" class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.setup.newUsername') }}</label>
            <input v-model="username" type="text" :placeholder="t('admin.setup.newUsernamePlaceholder')" @input="touched.username = true"
                   :class="['w-full px-3 py-2.5 rounded-lg text-sm focus:outline-none focus:ring-1 transition-colors', fieldClass(usernameError)]"
                   :style="fieldStyle(usernameError)" />
            <p v-if="usernameError" class="mt-1 text-xs text-red-500">{{ usernameError }}</p>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.setup.newPassword') }}</label>
            <input v-model="password" type="password" :placeholder="t('admin.setup.newPasswordPlaceholder')" @input="touched.password = true"
                   :class="['w-full px-3 py-2.5 rounded-lg text-sm focus:outline-none focus:ring-1 transition-colors', fieldClass(passwordError)]"
                   :style="fieldStyle(passwordError)" />
            <p v-if="passwordError" class="mt-1 text-xs text-red-500">{{ passwordError }}</p>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.setup.confirmPassword') }}</label>
            <input v-model="confirmPassword" type="password" :placeholder="t('admin.setup.confirmPasswordPlaceholder')" @input="touched.confirm = true"
                   :class="['w-full px-3 py-2.5 rounded-lg text-sm focus:outline-none focus:ring-1 transition-colors', fieldClass(confirmError)]"
                   :style="fieldStyle(confirmError)" />
            <p v-if="confirmError" class="mt-1 text-xs text-red-500">{{ confirmError }}</p>
          </div>
          <button type="submit" :disabled="loading || !canSubmit"
                  class="w-full py-2.5 bg-emerald-500 hover:bg-emerald-600 disabled:bg-gray-300 dark:disabled:bg-gray-700 text-white font-medium rounded-lg text-sm transition-colors">
            {{ loading ? t('admin.setup.submitting') : t('admin.setup.submit') }}
          </button>
        </form>
      </div>

      <div class="text-center mt-6">
        <a href="/" class="text-sm transition-colors" style="color: var(--text-color); opacity: 0.4;">{{ t('common.backToStatus') }}</a>
      </div>
    </div>
    <Toast />
  </div>
</template>
