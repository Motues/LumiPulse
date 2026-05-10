<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'
import { useAuth } from '../stores/auth'
import { siteName, siteIcon } from '../composables/useSiteConfig'
import { useToast } from '../composables/useToast'
import Toast from '../components/dashboard/Toast.vue'

const router = useRouter()
const { setToken } = useAuth()
const { show: toast } = useToast()

const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const loading = ref(false)

async function handleSetup() {
  if (!username.value || !password.value) {
    toast('请输入用户名和密码')
    return
  }
  if (username.value.length < 2) {
    toast('用户名至少需要2个字符')
    return
  }
  if (password.value.length < 4) {
    toast('密码至少需要4个字符')
    return
  }
  if (password.value !== confirmPassword.value) {
    toast('两次输入的密码不一致')
    return
  }

  loading.value = true
  try {
    await api.setup(username.value, password.value)
    setToken('')
    toast('设置成功，请使用新账号重新登录', 'success')
    setTimeout(() => router.push('/login'), 2000)
  } catch (e: any) {
    toast(e.message || '设置失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-[#f7f8fa] dark:bg-gray-950 flex items-center justify-center px-4">
    <div class="w-full max-w-sm">
      <div class="text-center mb-8">
        <div class="flex items-center justify-center gap-2 mb-2">
          <img v-if="siteIcon" :src="siteIcon" class="w-8 h-8 object-contain" />
          <img v-else src="/assets/logo.svg" class="w-8 h-8 object-contain" />
          <span class="text-2xl font-bold tracking-tight text-gray-900 dark:text-gray-100">{{ siteName }}</span>
        </div>
        <p class="text-sm text-gray-500 dark:text-gray-400">首次使用，请设置管理员账号</p>
      </div>

      <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 p-6">
        <form @submit.prevent="handleSetup" class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">新用户名</label>
            <input v-model="username" type="text" placeholder="请输入新用户名"
                   class="w-full px-3 py-2.5 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">新密码</label>
            <input v-model="password" type="password" placeholder="请输入新密码"
                   class="w-full px-3 py-2.5 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">确认密码</label>
            <input v-model="confirmPassword" type="password" placeholder="请再次输入新密码"
                   class="w-full px-3 py-2.5 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500" />
          </div>
          <button type="submit" :disabled="loading"
                  class="w-full py-2.5 bg-emerald-500 hover:bg-emerald-600 disabled:bg-gray-300 dark:disabled:bg-gray-700 text-white font-medium rounded-lg text-sm transition-colors">
            {{ loading ? '设置中...' : '保存设置' }}
          </button>
        </form>
      </div>

      <div class="text-center mt-6">
        <a href="/" class="text-sm text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors">返回状态页</a>
      </div>
    </div>
    <Toast />
  </div>
</template>
