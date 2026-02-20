<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AppLayout from './components/AppLayout.vue'
import LoginForm from './components/LoginForm.vue'
import { clearAuth } from './composables/useApi'

const authChecked = ref(false)
const needsLogin = ref(false)

async function checkAuth() {
  const creds = localStorage.getItem('tt_auth')
  const headers: HeadersInit = creds ? { Authorization: `Basic ${creds}` } : {}

  try {
    const resp = await fetch('/api/status', { headers })
    needsLogin.value = resp.status === 401
  } catch {
    needsLogin.value = false
  }
  authChecked.value = true
}

function onLogin() {
  needsLogin.value = false
}

function onLogout() {
  clearAuth()
  needsLogin.value = true
}

onMounted(checkAuth)
</script>

<template>
  <template v-if="!authChecked" />
  <LoginForm v-else-if="needsLogin" @login="onLogin" />
  <AppLayout v-else @logout="onLogout">
    <router-view />
  </AppLayout>
</template>
