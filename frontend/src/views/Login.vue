<template>
  <div class="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-100 to-gray-200 px-4">
    <div class="w-full max-w-md">
      <!-- Logo -->
      <div class="mb-8 text-center">
        <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-xl bg-indigo-600 text-xl font-bold text-white shadow-lg shadow-indigo-200 overflow-hidden">
          <img v-if="siteIcon" :src="siteIcon" class="w-14 h-14 object-contain" />
          <span v-else>LP</span>
        </div>
        <h1 class="mt-4 text-2xl font-bold text-gray-900">{{ siteName }}</h1>
        <p class="mt-1 text-sm text-gray-500">监控面板 · 管理员登录</p>
      </div>

      <!-- Form -->
      <div class="rounded-2xl bg-white p-8 shadow-xl shadow-gray-200/50">
        <form @submit.prevent="handleLogin">
          <div class="space-y-5">
            <div>
              <label class="block text-sm font-medium text-gray-700">用户名</label>
              <input
                v-model="form.username"
                type="text"
                autocomplete="username"
                class="mt-1.5 block w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm placeholder-gray-400 shadow-sm transition-colors focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                placeholder="请输入用户名"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">密码</label>
              <input
                v-model="form.password"
                type="password"
                autocomplete="current-password"
                class="mt-1.5 block w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm placeholder-gray-400 shadow-sm transition-colors focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                placeholder="请输入密码"
              />
            </div>
          </div>

          <p v-if="error" class="mt-3 text-sm text-red-500">{{ error }}</p>

          <button
            type="submit"
            :disabled="loading"
            class="mt-6 flex w-full items-center justify-center rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
          >
            <svg v-if="loading" class="-ml-1 mr-2 h-4 w-4 animate-spin text-white" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
            {{ loading ? '登录中...' : '登 录' }}
          </button>
        </form>
      </div>

      <p class="mt-6 text-center text-xs text-gray-400">{{ siteName }} v0.1.0</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { siteName, siteIcon } from '../composables/useSiteConfig'

const router = useRouter()
const loading = ref(false)
const error = ref('')

const form = reactive({
  username: '',
  password: '',
})

function handleLogin() {
  error.value = ''

  if (!form.username || !form.password) {
    error.value = '请输入用户名和密码'
    return
  }

  loading.value = true

  // Simulate login
  setTimeout(() => {
    loading.value = false
    router.push('/dashboard')
  }, 800)
}
</script>
