<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import { useToast } from '../../composables/useToast'

const { show: toast } = useToast()

const currentUsername = ref('')
const loading = ref(true)
const saving = ref(false)

const form = ref({
  oldPassword: '',
  newUsername: '',
  newPassword: '',
  confirmPassword: '',
})

const usernameError = computed(() => {
  if (!form.value.newUsername) return ''
  return form.value.newUsername.length < 2 ? '用户名至少需要2个字符' : ''
})

const passwordError = computed(() => {
  if (!form.value.newPassword) return ''
  return form.value.newPassword.length < 4 ? '密码至少需要4个字符' : ''
})

const confirmError = computed(() => {
  if (!form.value.confirmPassword || !form.value.newPassword) return ''
  return form.value.newPassword !== form.value.confirmPassword ? '两次输入的密码不一致' : ''
})

const canSave = computed(() => {
  if (!form.value.oldPassword) return false
  if (usernameError.value) return false
  if (passwordError.value) return false
  if (confirmError.value) return false
  return !!form.value.newUsername || !!form.value.newPassword
})

async function load() {
  loading.value = true
  try {
    const res = await api.getCurrentUser()
    currentUsername.value = res.data.username
  } catch (e: any) {
    toast(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!canSave.value) return
  saving.value = true
  try {
    const payload: { oldPassword: string; newUsername?: string; newPassword?: string } = { oldPassword: form.value.oldPassword }
    if (form.value.newUsername) payload.newUsername = form.value.newUsername
    if (form.value.newPassword) payload.newPassword = form.value.newPassword

    await api.updateProfile(payload)
    toast('保存成功，下次登录时请使用新的凭证', 'success')
    if (form.value.newUsername) {
      currentUsername.value = form.value.newUsername
      form.value.newUsername = ''
    }
    form.value.oldPassword = ''
    form.value.newPassword = ''
    form.value.confirmPassword = ''
  } catch (e: any) {
    toast(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div v-if="loading" class="text-center py-12 text-gray-400 dark:text-gray-500">加载中...</div>

    <template v-else>
      <!-- Current User -->
      <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm p-6 mb-6">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-2">当前用户</h3>
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-full bg-emerald-500 text-white flex items-center justify-center font-bold text-sm">
            {{ currentUsername.charAt(0).toUpperCase() }}
          </div>
          <div>
            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ currentUsername }}</div>
            <div class="text-xs text-gray-400 dark:text-gray-500">管理员</div>
          </div>
        </div>
      </div>

      <!-- Change Form -->
      <div class="bg-white dark:bg-gray-900 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm p-6">
        <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100 mb-4">修改凭证</h3>
        <div class="space-y-4 max-w-md">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">当前密码 *</label>
            <input v-model="form.oldPassword" type="password" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
          </div>

          <div class="border-t border-gray-100 dark:border-gray-800 pt-4">
            <div class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-3">修改用户名（可选）</div>
            <div>
              <input v-model="form.newUsername" type="text" placeholder="新用户名" class="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:border-emerald-500 dark:bg-gray-800 dark:text-gray-100" />
              <p v-if="usernameError" class="text-xs text-red-500 mt-1">{{ usernameError }}</p>
            </div>
          </div>

          <div class="border-t border-gray-100 dark:border-gray-800 pt-4">
            <div class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-3">修改密码（可选）</div>
            <div class="space-y-3">
              <div>
                <input v-model="form.newPassword" type="password" placeholder="新密码" class="w-full px-3 py-2 border rounded-lg text-sm focus:outline-none dark:bg-gray-800 dark:text-gray-100" :class="passwordError ? 'border-red-300 dark:border-red-700 focus:border-red-500' : form.newPassword && !passwordError ? 'border-emerald-300 dark:border-emerald-700 focus:border-emerald-500' : 'border-gray-200 dark:border-gray-700 focus:border-emerald-500'" />
                <p v-if="passwordError" class="text-xs text-red-500 mt-1">{{ passwordError }}</p>
              </div>
              <div>
                <input v-model="form.confirmPassword" type="password" placeholder="再次输入新密码" class="w-full px-3 py-2 border rounded-lg text-sm focus:outline-none dark:bg-gray-800 dark:text-gray-100" :class="confirmError ? 'border-red-300 dark:border-red-700 focus:border-red-500' : form.newPassword && form.confirmPassword ? 'border-emerald-300 dark:border-emerald-700 focus:border-emerald-500' : 'border-gray-200 dark:border-gray-700 focus:border-emerald-500'" />
                <p v-if="confirmError" class="text-xs text-red-500 mt-1">{{ confirmError }}</p>
              </div>
            </div>
          </div>

          <div class="pt-2">
            <button
              @click="save"
              :disabled="!canSave || saving"
              class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:bg-gray-300 dark:disabled:bg-gray-700 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            >
              {{ saving ? '保存中...' : '保存修改' }}
            </button>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
