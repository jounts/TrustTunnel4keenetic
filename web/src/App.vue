<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AppLayout from './components/AppLayout.vue'
import LoginForm from './components/LoginForm.vue'
import { checkAuth, logout } from './composables/useApi'

const authChecked = ref(false)
const needsLogin = ref(false)
const authMode = ref('ndm')

async function doCheckAuth() {
  const result = await checkAuth()
  needsLogin.value = !result.authenticated
  authMode.value = result.authMode
  authChecked.value = true
}

function onAuthenticated() {
  needsLogin.value = false
}

async function onLogout() {
  await logout()
  needsLogin.value = true
}

onMounted(doCheckAuth)
</script>

<template>
  <template v-if="!authChecked" />
  <LoginForm v-else-if="needsLogin" :auth-mode="authMode" @authenticated="onAuthenticated" />
  <AppLayout v-else @logout="onLogout">
    <router-view />
  </AppLayout>
</template>
