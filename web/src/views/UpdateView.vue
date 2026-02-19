<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi, type UpdateInfo } from '@/composables/useApi'

const api = useApi()
const updateInfo = ref<UpdateInfo | null>(null)
const installing = ref(false)
const installStatus = ref('')
const installResult = ref<string | null>(null)

onMounted(async () => {
  updateInfo.value = await api.checkUpdate()
})

async function checkForUpdates() {
  installResult.value = null
  updateInfo.value = await api.checkUpdate()
}

async function doInstall() {
  installing.value = true
  installResult.value = null
  installStatus.value = 'Скачивание (~5 МБ), это может занять несколько минут...'
  const result = await api.installUpdate()
  installStatus.value = ''
  if (result) {
    installResult.value = result.message || 'Обновлено'
    await checkForUpdates()
  } else {
    installResult.value = api.error.value || 'Ошибка обновления'
  }
  installing.value = false
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold">Обновление</h1>
      <button
        @click="checkForUpdates"
        :disabled="api.loading.value"
        class="px-4 py-2 rounded-lg text-sm font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors"
      >
        Проверить обновления
      </button>
    </div>

    <div v-if="updateInfo" class="space-y-4">
      <!-- Client version -->
      <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-lg font-semibold">TrustTunnel Client</h2>
            <div class="mt-2 space-y-1 text-sm">
              <p>Текущая: <span class="font-mono font-medium">{{ updateInfo.client_current_version }}</span></p>
              <p>Доступна: <span class="font-mono font-medium">{{ updateInfo.client_latest_version || '—' }}</span></p>
            </div>
          </div>
          <div>
            <span
              v-if="updateInfo.client_update_available"
              class="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400"
            >
              Доступно обновление
            </span>
            <span
              v-else
              class="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400"
            >
              Актуальная версия
            </span>
          </div>
        </div>

        <div v-if="updateInfo.client_update_available" class="mt-4">
          <button
            @click="doInstall"
            :disabled="installing"
            class="inline-flex items-center px-4 py-2 rounded-lg text-sm font-medium text-white bg-brand-600 hover:bg-brand-700 disabled:opacity-50 transition-colors"
          >
            <svg v-if="installing" class="w-4 h-4 mr-2 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
            </svg>
            {{ installing ? 'Установка...' : 'Установить обновление' }}
          </button>
          <p v-if="installStatus" class="mt-2 text-xs text-gray-500 dark:text-gray-400">
            {{ installStatus }}
          </p>
        </div>
      </div>

      <!-- Manager version -->
      <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-lg font-semibold">TrustTunnel Manager</h2>
            <div class="mt-2 space-y-1 text-sm">
              <p>Текущая: <span class="font-mono font-medium">{{ updateInfo.manager_current_version }}</span></p>
              <p>Доступна: <span class="font-mono font-medium">{{ updateInfo.manager_latest_version || '—' }}</span></p>
            </div>
          </div>
          <span
            v-if="updateInfo.manager_update_available"
            class="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400"
          >
            Доступно обновление
          </span>
          <span
            v-else
            class="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400"
          >
            Актуальная версия
          </span>
        </div>
      </div>
    </div>

    <div v-else-if="api.loading.value" class="text-sm text-gray-500">Проверка обновлений...</div>

    <p v-if="installResult" class="text-sm" :class="api.error.value ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'">
      {{ installResult }}
    </p>
    <p v-if="api.error.value && !installResult" class="text-red-600 dark:text-red-400 text-sm">{{ api.error.value }}</p>
  </div>
</template>
