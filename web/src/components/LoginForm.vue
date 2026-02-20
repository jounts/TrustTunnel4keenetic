<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { checkAuth } from '../composables/useApi'

const props = defineProps<{ authMode: string }>()
const emit = defineEmits<{ (e: 'authenticated'): void }>()

const polling = ref(false)
const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)
const routerOpened = ref(false)

const routerUrl = `${window.location.protocol}//${window.location.hostname}`

function openRouterLogin() {
  window.open(routerUrl, '_blank')
  routerOpened.value = true
  startPolling()
}

function startPolling() {
  if (pollTimer.value) return
  polling.value = true
  pollTimer.value = setInterval(async () => {
    const result = await checkAuth()
    if (result.ok) {
      stopPolling()
      emit('authenticated')
    }
  }, 2000)
}

function stopPolling() {
  polling.value = false
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = null
  }
}

async function retryAuth() {
  const result = await checkAuth()
  if (result.ok) {
    emit('authenticated')
  }
}

onMounted(() => {
  // If NDM mode, start polling immediately in case user is already logged in on router
  if (props.authMode === 'ndm') {
    retryAuth()
  }
})

onUnmounted(stopPolling)
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900 px-4">
    <div class="w-full max-w-sm">
      <div class="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
        <div class="flex justify-center mb-6">
          <div class="w-14 h-14 rounded-xl bg-brand-600 flex items-center justify-center">
            <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </div>
        </div>

        <h2 class="text-xl font-bold text-center text-gray-900 dark:text-white mb-1">TrustTunnel Manager</h2>

        <!-- NDM auth mode: redirect to router -->
        <template v-if="authMode === 'ndm'">
          <p class="text-sm text-center text-gray-500 dark:text-gray-400 mb-6">
            Для доступа необходимо войти в веб-интерфейс роутера
          </p>

          <button
            @click="openRouterLogin"
            class="w-full py-2.5 rounded-lg bg-brand-600 hover:bg-brand-700 text-white font-medium transition flex items-center justify-center gap-2"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
            </svg>
            Войти через роутер
          </button>

          <div v-if="routerOpened" class="mt-4">
            <div v-if="polling" class="flex items-center justify-center gap-2 text-sm text-gray-500 dark:text-gray-400">
              <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
              Ожидание авторизации...
            </div>
          </div>

          <button
            @click="retryAuth"
            class="w-full mt-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 text-sm hover:bg-gray-50 dark:hover:bg-gray-700 transition"
          >
            Проверить доступ
          </button>
        </template>

        <!-- Local auth mode: username/password form -->
        <template v-else>
          <p class="text-sm text-center text-gray-500 dark:text-gray-400 mb-6">
            Введите имя пользователя и пароль
          </p>
          <LocalLoginForm @authenticated="emit('authenticated')" />
        </template>
      </div>
    </div>
  </div>
</template>
