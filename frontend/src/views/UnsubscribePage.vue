<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client'
import type { ServiceSummary } from '../api/types'
import PublicHeader from '../components/PublicHeader.vue'
import PublicFooter from '../components/PublicFooter.vue'
import { useI18n } from '../composables/useI18n'

/**
 * 订阅自助管理页（邮件底部的退订链接落地页）。
 *
 * 链接形如 /unsubscribe?email=...&token=...，token 是后端用密钥对邮箱做的 HMAC 签名，
 * 因此链接无法被猜到、也无法用来退订别人的邮箱。
 * 页面支持「调整订阅偏好」与「一键退订」两种操作。
 */
const route = useRoute()
const { t } = useI18n()

const email = computed(() => String(route.query.email || '').trim())
const token = computed(() => String(route.query.token || '').trim())

const loading = ref(true)
const error = ref('')
const saving = ref(false)
const done = ref<'saved' | 'unsubscribed' | ''>('')

const services = ref<ServiceSummary[]>([])
const selected = ref<number[]>([])
/** 订阅范围：全部服务（后端用空列表表示）或指定服务 */
const mode = ref<'all' | 'some'>('all')
/** 全选按钮：勾上表示订阅全部 */
const allSelected = computed(() => mode.value === 'all')

function setAll(checked: boolean) {
  mode.value = checked ? 'all' : 'some'
  if (checked) selected.value = []
  done.value = ''
}

function toggle(id: number) {
  mode.value = 'some'
  const idx = selected.value.indexOf(id)
  if (idx >= 0) selected.value.splice(idx, 1)
  else selected.value.push(id)
  done.value = ''
}

async function load() {
  if (!email.value || !token.value) {
    error.value = t('unsubscribe.invalidLink')
    loading.value = false
    return
  }
  try {
    const [subRes, sumRes] = await Promise.all([
      api.getSubscription(email.value, token.value),
      api.getSummary(),
    ])
    selected.value = subRes.data.services || []
    mode.value = selected.value.length > 0 ? 'some' : 'all'
    services.value = sumRes.data.services || []
  } catch (e: any) {
    error.value = e.message || t('unsubscribe.invalidLink')
  } finally {
    loading.value = false
  }
}

async function savePreferences() {
  saving.value = true
  try {
    await api.updateSubscription(email.value, token.value, mode.value === 'all' ? [] : selected.value)
    done.value = 'saved'
  } catch (e: any) {
    error.value = e.message || t('common.loadFailed')
  } finally {
    saving.value = false
  }
}

async function unsubscribe() {
  if (!confirm(t('unsubscribe.confirm'))) return
  saving.value = true
  try {
    await api.unsubscribe(email.value, token.value)
    done.value = 'unsubscribed'
  } catch (e: any) {
    error.value = e.message || t('common.loadFailed')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <PublicHeader :on-subscribe="() => {}" />

  <main class="max-w-[720px] mx-auto px-6 py-10">
    <div v-if="loading" class="text-center py-20" style="color: var(--text-color); opacity: 0.4;">
      {{ t('common.loading') }}
    </div>

    <template v-else>
      <h1 class="text-2xl font-bold mb-2" style="color: var(--text-color);">{{ t('unsubscribe.title') }}</h1>
      <p class="text-sm mb-6" style="color: var(--text-color); opacity: 0.5;">
        {{ t('unsubscribe.subtitle', { email }) }}
      </p>

      <div v-if="error" class="rounded-lg p-4 mb-6 text-sm text-red-500" style="border: 1px solid var(--button-border-color);">
        {{ error }}
      </div>

      <div
        v-if="done === 'unsubscribed'"
        class="rounded-lg p-6 text-center"
        style="border: 1px solid var(--button-border-color);"
      >
        <p class="text-base font-medium mb-1" style="color: var(--text-color);">{{ t('unsubscribe.doneUnsubscribed') }}</p>
        <p class="text-sm" style="color: var(--text-color); opacity: 0.5;">{{ t('unsubscribe.doneUnsubscribedHint') }}</p>
      </div>

      <template v-else-if="!error">
        <div v-if="done === 'saved'" class="rounded-lg p-3 mb-4 text-sm text-emerald-600 dark:text-emerald-400" style="border: 1px solid var(--button-border-color);">
          {{ t('unsubscribe.saved') }}
        </div>

        <div class="rounded-lg p-6 mb-6" style="border: 1px solid var(--button-border-color);">
          <h2 class="font-bold mb-1" style="color: var(--text-color);">{{ t('unsubscribe.preferences') }}</h2>
          <p class="text-xs mb-4" style="color: var(--text-color); opacity: 0.45;">{{ t('unsubscribe.preferencesHint') }}</p>

          <label class="flex items-center gap-3 py-2.5 cursor-pointer">
            <input
              type="radio"
              name="scope"
              class="accent-emerald-500"
              :checked="allSelected"
              @change="setAll(true)"
            />
            <span class="text-sm font-medium" style="color: var(--text-color);">{{ t('unsubscribe.allServices') }}</span>
          </label>
          <label class="flex items-center gap-3 py-2.5 cursor-pointer">
            <input
              type="radio"
              name="scope"
              class="accent-emerald-500"
              :checked="!allSelected"
              @change="setAll(false)"
            />
            <span class="text-sm font-medium" style="color: var(--text-color);">{{ t('unsubscribe.someServices') }}</span>
          </label>

          <div v-if="!allSelected" class="divide-y pt-1" style="border-color: var(--button-border-color);">
            <label
              v-for="svc in services"
              :key="svc.id"
              class="flex items-center gap-3 py-2.5 pl-6 cursor-pointer"
            >
              <input
                type="checkbox"
                class="accent-emerald-500"
                :checked="selected.includes(svc.id)"
                @change="toggle(svc.id)"
              />
              <span class="text-sm" style="color: var(--text-color);">{{ svc.name }}</span>
            </label>
            <p v-if="services.length === 0" class="py-3 pl-6 text-sm" style="color: var(--text-color); opacity: 0.4;">
              {{ t('unsubscribe.noServices') }}
            </p>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3 mt-6">
            <button
              @click="savePreferences"
              :disabled="saving"
              class="px-4 py-2 bg-emerald-500 hover:bg-emerald-600 disabled:opacity-40 text-white text-sm font-medium rounded-lg transition-colors"
            >{{ saving ? t('common.loading') : t('unsubscribe.save') }}</button>
            <button
              @click="unsubscribe"
              :disabled="saving"
              class="px-4 py-2 text-sm font-medium rounded-lg transition-colors text-red-500 border border-red-300/60 hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-40"
            >{{ t('unsubscribe.action') }}</button>
          </div>
        </div>
      </template>
    </template>
  </main>

  <PublicFooter />
</template>
