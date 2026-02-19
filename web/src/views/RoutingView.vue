<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useApi, type RoutingConfig, type RoutingStats } from '@/composables/useApi'

const api = useApi()

const config = ref<RoutingConfig>({
  sr_enabled: 'no',
  sr_home_country: 'RU',
  sr_dns_port: 5354,
  sr_dns_upstream: '1.1.1.1',
})
const stats = ref<RoutingStats | null>(null)
const domainsText = ref('')
const saved = ref(false)
const domainsSaved = ref(false)
const netsUpdating = ref(false)
const netsUpdated = ref(false)

const isEnabled = computed({
  get: () => config.value.sr_enabled === 'yes',
  set: (val: boolean) => { config.value.sr_enabled = val ? 'yes' : 'no' },
})

const countries = [
  { code: 'RU', name: 'Россия' },
  { code: 'UA', name: 'Украина' },
  { code: 'BY', name: 'Беларусь' },
  { code: 'KZ', name: 'Казахстан' },
  { code: 'UZ', name: 'Узбекистан' },
  { code: 'GE', name: 'Грузия' },
  { code: 'AM', name: 'Армения' },
  { code: 'AZ', name: 'Азербайджан' },
  { code: 'MD', name: 'Молдова' },
  { code: 'KG', name: 'Киргизия' },
  { code: 'TJ', name: 'Таджикистан' },
  { code: 'TM', name: 'Туркменистан' },
  { code: 'LV', name: 'Латвия' },
  { code: 'LT', name: 'Литва' },
  { code: 'EE', name: 'Эстония' },
  { code: 'TR', name: 'Турция' },
  { code: 'DE', name: 'Германия' },
  { code: 'US', name: 'США' },
]

onMounted(async () => {
  await loadData()
})

async function loadData() {
  const routing = await api.getRouting()
  if (routing) {
    config.value = routing.config
    stats.value = routing.stats
  }

  const domains = await api.getRoutingDomains()
  if (domains) {
    domainsText.value = domains.domains.join('\n')
  }
}

async function saveConfig() {
  const result = await api.putRouting(config.value)
  if (result) {
    saved.value = true
    setTimeout(() => { saved.value = false }, 3000)
  }
}

async function saveDomains() {
  const domains = domainsText.value
    .split('\n')
    .map(d => d.trim())
    .filter(d => d && !d.startsWith('#'))

  const result = await api.putRoutingDomains({ domains })
  if (result) {
    domainsSaved.value = true
    setTimeout(() => { domainsSaved.value = false }, 3000)
  }
}

async function updateNets() {
  netsUpdating.value = true
  const result = await api.updateRoutingNets()
  netsUpdating.value = false
  if (result) {
    netsUpdated.value = true
    setTimeout(() => { netsUpdated.value = false }, 3000)
    await loadData()
  }
}

function formatDate(iso: string): string {
  if (!iso) return 'нет данных'
  try {
    return new Date(iso).toLocaleString('ru-RU')
  } catch {
    return iso
  }
}
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold">Маршрутизация</h1>

    <!-- Enable / Disable -->
    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-lg font-semibold">Smart Routing</h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
            Трафик к ресурсам в домашней стране идёт напрямую, остальной — через туннель.
          </p>
        </div>
        <button
          @click="isEnabled = !isEnabled"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2',
            isEnabled ? 'bg-brand-600' : 'bg-gray-300 dark:bg-gray-600'
          ]"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              isEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>
      <div v-if="isEnabled" class="mt-4 p-3 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800">
        <p class="text-sm text-amber-700 dark:text-amber-400">
          Smart Routing работает только в режиме TUN. Убедитесь, что режим работы — TUN.
        </p>
      </div>
    </div>

    <!-- Country & DNS settings -->
    <div v-if="isEnabled" class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold mb-4">Параметры</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Домашняя страна</label>
          <select
            v-model="config.sr_home_country"
            class="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
          >
            <option v-for="c in countries" :key="c.code" :value="c.code">
              {{ c.name }} ({{ c.code }})
            </option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">DNS Upstream</label>
          <input
            v-model="config.sr_dns_upstream"
            type="text"
            placeholder="1.1.1.1"
            class="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">DNS порт</label>
          <input
            v-model.number="config.sr_dns_port"
            type="number"
            min="1"
            max="65535"
            class="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
          />
        </div>
      </div>

      <div class="mt-4 flex items-center gap-3">
        <button
          @click="saveConfig"
          :disabled="api.loading.value"
          class="inline-flex items-center px-4 py-2 rounded-lg text-sm font-medium text-white bg-brand-600 hover:bg-brand-700 disabled:opacity-50 transition-colors"
        >
          Сохранить
        </button>
        <p v-if="saved" class="text-sm text-green-600 dark:text-green-400">Настройки сохранены</p>
      </div>

      <div class="mt-4 p-3 rounded-lg bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800">
        <p class="text-sm text-blue-700 dark:text-blue-400">
          После включения добавьте в настройках DNS роутера сервер <code class="font-mono bg-blue-100 dark:bg-blue-800 px-1 rounded">&lt;IP роутера&gt;:{{ config.sr_dns_port }}</code>
        </p>
      </div>
    </div>

    <!-- Domain list -->
    <div v-if="isEnabled" class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <div class="flex items-center justify-between mb-2">
        <h2 class="text-lg font-semibold">Домены через туннель</h2>
        <button
          @click="saveDomains"
          :disabled="api.loading.value"
          class="inline-flex items-center px-4 py-2 rounded-lg text-sm font-medium text-white bg-brand-600 hover:bg-brand-700 disabled:opacity-50 transition-colors"
        >
          Сохранить
        </button>
      </div>
      <p class="text-sm text-gray-500 dark:text-gray-400 mb-3">
        Домены из этого списка всегда идут через туннель, даже если IP находится в домашней стране (решает проблему CDN). По одному домену на строку.
      </p>
      <textarea
        v-model="domainsText"
        rows="10"
        placeholder="youtube.com&#10;googlevideo.com&#10;netflix.com"
        class="w-full font-mono text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg p-3 focus:ring-2 focus:ring-brand-500 focus:border-brand-500 resize-y"
        spellcheck="false"
      />
      <p v-if="domainsSaved" class="mt-2 text-sm text-green-600 dark:text-green-400">Список доменов сохранён, dnsmasq перезагружен</p>
    </div>

    <!-- Stats & GeoIP update -->
    <div v-if="isEnabled && stats" class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold mb-4">Статистика</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">Домашних подсетей (ipset)</label>
          <p class="font-medium text-lg">{{ stats.domestic_entries.toLocaleString() }}</p>
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">Туннельных IP (DNS)</label>
          <p class="font-medium text-lg">{{ stats.tunnel_entries.toLocaleString() }}</p>
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">dnsmasq</label>
          <p :class="['font-medium', stats.dnsmasq_running ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400']">
            {{ stats.dnsmasq_running ? 'Работает' : 'Остановлен' }}
          </p>
        </div>
        <div>
          <label class="block text-xs text-gray-500 dark:text-gray-400 mb-1">GeoIP обновлён</label>
          <p class="font-medium">{{ formatDate(stats.nets_updated_at) }}</p>
        </div>
      </div>

      <div class="mt-4 flex items-center gap-3">
        <button
          @click="updateNets"
          :disabled="netsUpdating"
          class="inline-flex items-center px-4 py-2 rounded-lg text-sm font-medium text-white bg-brand-600 hover:bg-brand-700 disabled:opacity-50 transition-colors"
        >
          <svg v-if="netsUpdating" class="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
          Обновить GeoIP-списки
        </button>
        <p v-if="netsUpdated" class="text-sm text-green-600 dark:text-green-400">Списки обновлены</p>
      </div>
    </div>

    <p v-if="api.error.value" class="text-red-600 dark:text-red-400 text-sm">{{ api.error.value }}</p>
  </div>
</template>
