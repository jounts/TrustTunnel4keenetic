<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{ authMode: string }>()
const emit = defineEmits<{ (e: 'authenticated'): void }>()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function doLogin() {
  error.value = ''
  loading.value = true

  try {
    if (props.authMode === 'local') {
      const creds = btoa(`${username.value}:${password.value}`)
      const resp = await fetch('/api/auth/check', {
        headers: { Authorization: `Basic ${creds}` },
      })
      const data = await resp.json()
      if (!data.authenticated) {
        error.value = 'Неверное имя пользователя или пароль'
        return
      }
      localStorage.setItem('tt_basic_auth', creds)
      emit('authenticated')
    } else {
      const resp = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: username.value, password: password.value }),
      })
      const data = await resp.json()
      if (!data.ok) {
        error.value = data.error || 'Ошибка авторизации'
        return
      }
      emit('authenticated')
    }
  } catch {
    error.value = 'Не удалось подключиться к серверу'
  } finally {
    loading.value = false
  }
}
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
        <p class="text-sm text-center text-gray-500 dark:text-gray-400 mb-6">
          Используйте учётные данные роутера
        </p>

        <form @submit.prevent="doLogin" class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Имя пользователя</label>
            <input
              v-model="username"
              type="text"
              autocomplete="username"
              required
              class="w-full px-3 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-brand-500 focus:border-brand-500 outline-none transition"
              placeholder="admin"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Пароль</label>
            <input
              v-model="password"
              type="password"
              autocomplete="current-password"
              required
              class="w-full px-3 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-brand-500 focus:border-brand-500 outline-none transition"
            />
          </div>

          <div v-if="error" class="text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 rounded-lg px-3 py-2">
            {{ error }}
          </div>

          <button
            type="submit"
            :disabled="loading"
            class="w-full py-2.5 rounded-lg bg-brand-600 hover:bg-brand-700 text-white font-medium transition disabled:opacity-50"
          >
            <span v-if="loading">Проверка...</span>
            <span v-else>Войти</span>
          </button>
        </form>
      </div>
    </div>
  </div>
</template>
