<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{ (e: 'authenticated'): void }>()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function doLogin() {
  error.value = ''
  loading.value = true

  const creds = btoa(`${username.value}:${password.value}`)

  try {
    const resp = await fetch('/api/status', {
      headers: { Authorization: `Basic ${creds}` },
    })

    if (resp.status === 401) {
      error.value = 'Неверное имя пользователя или пароль'
      return
    }

    if (!resp.ok) {
      error.value = `Ошибка сервера: ${resp.status}`
      return
    }

    localStorage.setItem('tt_basic_auth', creds)
    emit('authenticated')
  } catch {
    error.value = 'Не удалось подключиться к серверу'
  } finally {
    loading.value = false
  }
}
</script>

<template>
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
</template>
