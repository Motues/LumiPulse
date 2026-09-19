<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../api/client'
import { useToast } from '../../composables/useToast'
import { useI18n } from '../../composables/useI18n'

const { show: toast } = useToast()
const { t } = useI18n()

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
  return form.value.newUsername.length < 2 ? t('admin.user.usernameMin') : ''
})

const passwordError = computed(() => {
  if (!form.value.newPassword) return ''
  return form.value.newPassword.length < 4 ? t('admin.user.passwordMin') : ''
})

const confirmError = computed(() => {
  if (!form.value.confirmPassword || !form.value.newPassword) return ''
  return form.value.newPassword !== form.value.confirmPassword ? t('admin.user.passwordMismatch') : ''
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
    toast(e.message || t('admin.common.loadFailed'))
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
    toast(t('admin.user.saved'), 'success')
    if (form.value.newUsername) {
      currentUsername.value = form.value.newUsername
      form.value.newUsername = ''
    }
    form.value.oldPassword = ''
    form.value.newPassword = ''
    form.value.confirmPassword = ''
  } catch (e: any) {
    toast(e.message || t('admin.common.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div v-if="loading" class="text-center py-12" style="color: var(--text-color); opacity: 0.4;">{{ t('common.loading') }}</div>

    <template v-else>
      <!-- Current User -->
      <div class="rounded-xl p-6 mb-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-2" style="color: var(--text-color);">{{ t('admin.user.currentTitle') }}</h3>
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-full bg-emerald-500 text-white flex items-center justify-center font-bold text-sm">
            {{ currentUsername.charAt(0).toUpperCase() }}
          </div>
          <div>
            <div class="text-sm font-medium" style="color: var(--text-color);">{{ currentUsername }}</div>
            <div class="text-xs" style="color: var(--text-color); opacity: 0.4;">{{ t('admin.user.roleAdmin') }}</div>
          </div>
        </div>
      </div>

      <!-- Change Form -->
      <div class="rounded-xl p-6" style="background-color: var(--bg-color); border: 1px solid var(--button-border-color);">
        <h3 class="text-lg font-bold mb-4" style="color: var(--text-color);">{{ t('admin.user.changeTitle') }}</h3>
        <div class="space-y-4 max-w-md">
          <div>
            <label class="block text-sm font-medium mb-1" style="color: var(--text-color);">{{ t('admin.user.currentPassword') }}</label>
            <input v-model="form.oldPassword" type="password" class="w-full px-3 py-2 rounded-lg text-sm input-field" />
          </div>

          <div class="pt-4" style="border-top: 1px solid var(--button-border-color);">
            <div class="text-sm font-medium mb-3" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.user.changeUsername') }}</div>
            <div>
              <input v-model="form.newUsername" type="text" :placeholder="t('admin.user.newUsernamePlaceholder')" class="w-full px-3 py-2 rounded-lg text-sm input-field" />
              <p v-if="usernameError" class="text-xs text-red-500 mt-1">{{ usernameError }}</p>
            </div>
          </div>

          <div class="pt-4" style="border-top: 1px solid var(--button-border-color);">
            <div class="text-sm font-medium mb-3" style="color: var(--text-color); opacity: 0.5;">{{ t('admin.user.changePassword') }}</div>
            <div class="space-y-3">
              <div>
                <input v-model="form.newPassword" type="password" :placeholder="t('admin.user.newPasswordPlaceholder')" class="w-full px-3 py-2 border rounded-lg text-sm focus:outline-none" style="background-color: var(--bg-color); color: var(--text-color);" :class="passwordError ? 'border-red-300 dark:border-red-700 focus:border-red-500' : form.newPassword && !passwordError ? 'border-emerald-300 dark:border-emerald-700 focus:border-emerald-500' : ''" :style="!passwordError && !(form.newPassword && !passwordError) ? { borderColor: 'var(--button-border-color)' } : {}" />
                <p v-if="passwordError" class="text-xs text-red-500 mt-1">{{ passwordError }}</p>
              </div>
              <div>
                <input v-model="form.confirmPassword" type="password" :placeholder="t('admin.user.confirmPasswordPlaceholder')" class="w-full px-3 py-2 border rounded-lg text-sm focus:outline-none" style="background-color: var(--bg-color); color: var(--text-color);" :class="confirmError ? 'border-red-300 dark:border-red-700 focus:border-red-500' : form.newPassword && form.confirmPassword ? 'border-emerald-300 dark:border-emerald-700 focus:border-emerald-500' : ''" :style="!confirmError && !(form.newPassword && form.confirmPassword) ? { borderColor: 'var(--button-border-color)' } : {}" />
                <p v-if="confirmError" class="text-xs text-red-500 mt-1">{{ confirmError }}</p>
              </div>
            </div>
          </div>

          <div class="pt-2">
            <button
              @click="save"
              :disabled="!canSave || saving"
              class="px-6 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
              :style="(!canSave || saving) ? { opacity: 0.3 } : {}"
            >
              {{ saving ? t('admin.common.saving') : t('admin.user.saveChanges') }}
            </button>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
