<script setup lang="ts">
import { ref } from 'vue'
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
const loading = ref(false)

async function handleLogin() {
  if (!username.value || !password.value) {
    toast(t('admin.login.credentialsRequired'))
    return
  }
  loading.value = true
  try {
    const res = await api.login(username.value, password.value)
    setToken(res.data.token)
    if (res.data.needsSetup) {
      router.push('/setup')
    } else {
      router.push('/dashboard')
    }
  } catch (e: any) {
    toast(e.message || t('admin.login.failed'))
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
        <p class="text-sm" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.login.subtitle') }}</p>
      </div>

      <div class="rounded-xl p-6" style="border: 1px solid var(--button-border-color);">
        <form @submit.prevent="handleLogin" class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.login.username') }}</label>
            <input v-model="username" type="text" :placeholder="t('admin.login.usernamePlaceholder')"
                   class="w-full px-3 py-2.5 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
                   style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.login.password') }}</label>
            <input v-model="password" type="password" :placeholder="t('admin.login.passwordPlaceholder')"
                   class="w-full px-3 py-2.5 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
                   style="border: 1px solid var(--button-border-color); background-color: var(--bg-color); color: var(--text-color);" />
          </div>
          <button type="submit" :disabled="loading"
                  class="w-full py-2.5 bg-emerald-500 hover:bg-emerald-600 disabled:bg-gray-300 dark:disabled:bg-gray-700 text-white font-medium rounded-lg text-sm transition-colors">
            {{ loading ? t('admin.login.submitting') : t('admin.login.submit') }}
          </button>
        </form>
      </div>

      <div class="text-center mt-6">
        <a href="/" class="text-sm transition-opacity opacity-40 hover:opacity-100" style="color: var(--text-color);">{{ t('common.backToStatus') }}</a>
      </div>
    </div>
    <Toast />
  </div>
</template>
