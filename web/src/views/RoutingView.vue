<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useApi, type RoutingInfo, type ModeInfo } from '@/composables/useApi'

const api = useApi()
const routingInfo = ref<RoutingInfo | null>(null)
const modeInfo = ref<ModeInfo | null>(null)
const domains = ref('')
const saving = ref(false)
const savingDomains = ref(false)
const updatingNets = ref(false)
const message = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const enabled = ref(false)
const homeCountry = ref('RU')
const dnsPort = ref(5354)
const dnsUpstream = ref('1.1.1.1')

const isTunMode = computed(() => modeInfo.value?.mode === 'tun')

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
  { code: 'KG', name: 'Кыргызстан' },
  { code: 'TJ', name: 'Таджикистан' },
  { code: 'TM', name: 'Туркменистан' },
]

function showMessage(text: string, type: 'success' | 'error') {
  message.value = { text, type }
  setTimeout(() => { message.value = null }, 4000)
}

async function loadData() {
  const [ri, mi, dom] = await Promise.all([
    api.getRouting(),
    api.getMode(),
    api.getRoutingDomains(),
  ])
  if (ri) {
    routingInfo.value = ri
    enabled.value = ri.config.sr_enabled === 'yes'
    homeCountry.value = ri.config.sr_home_country || 'RU'
    dnsPort.value = ri.config.sr_dns_port || 5354
    dnsUpstream.value = ri.config.sr_dns_upstream || '1.1.1.1'
  }
  if (mi) modeInfo.value = mi
  if (dom) domains.value = dom.domains
}

async function saveConfig() {
  saving.value = true
  const result = await api.putRouting({
    sr_enabled: enabled.value ? 'yes' : 'no',
    sr_home_country: homeCountry.value,
    sr_dns_port: dnsPort.value,
    sr_dns_upstream: dnsUpstream.value,
  })
  saving.value = false
  if (result) {
    showMessage('Настройки сохранены', 'success')
    await loadData()
  } else {
    showMessage(api.error.value || 'Ошибка сохранения', 'error')
  }
}

async function saveDomains() {
  savingDomains.value = true
  const result = await api.putRoutingDomains({ domains: domains.value })
  savingDomains.value = false
  if (result) {
    showMessage('Список доменов обновлён', 'success')
  } else {
    showMessage(api.error.value || 'Ошибка сохранения', 'error')
  }
}

async function updateNets() {
  updatingNets.value = true
  const result = await api.updateRoutingNets()
  updatingNets.value = false
  if (result) {
    showMessage('GeoIP-списки обновлены', 'success')
    await loadData()
  } else {
    showMessage(api.error.value || 'Ошибка обновления', 'error')
  }
}

onMounted(loadData)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold">Smart Routing</h1>
      <button
        @click="loadData"
        class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
        title="Обновить"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      </button>
    </div>

    <!-- Message -->
    <div
      v-if="message"
      :class="[
        'rounded-lg px-4 py-3 text-sm',
        message.type === 'success' ? 'bg-green-50 text-green-800 dark:bg-green-900/20 dark:text-green-300' : 'bg-red-50 text-red-800 dark:bg-red-900/20 dark:text-red-300'
      ]"
    >
      {{ message.text }}
    </div>

    <!-- Non-TUN mode warning -->
    <div v-if="!isTunMode" class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg px-4 py-3 text-sm text-yellow-800 dark:text-yellow-300">
      Smart Routing доступен только в режиме TUN. Текущий режим: <strong>{{ modeInfo?.mode || '...' }}</strong>
    </div>

    <!-- Config -->
    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold mb-4">Настройки</h2>
      <div class="space-y-4">
        <div class="flex items-center justify-between">
          <div>
            <p class="font-medium">Включить Smart Routing</p>
            <p class="text-sm text-gray-500 dark:text-gray-400">GeoIP-маршрутизация: домашний трафик напрямую, зарубежный через туннель</p>
          </div>
          <button
            @click="enabled = !enabled"
            :class="[
              'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
              enabled ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'
            ]"
          >
            <span
              :class="[
                'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                enabled ? 'translate-x-6' : 'translate-x-1'
              ]"
            />
          </button>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium mb-1">Домашняя страна</label>
            <select v-model="homeCountry" class="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm">
              <option v-for="c in countries" :key="c.code" :value="c.code">{{ c.name }} ({{ c.code }})</option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1">DNS Upstream</label>
            <input v-model="dnsUpstream" type="text" class="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm" placeholder="1.1.1.1" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1">DNS порт</label>
            <input v-model.number="dnsPort" type="number" class="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm" placeholder="5354" />
          </div>
        </div>

        <button
          @click="saveConfig"
          :disabled="saving"
          class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 text-sm transition-colors"
        >
          {{ saving ? 'Сохранение...' : 'Сохранить' }}
        </button>
      </div>
    </div>

    <!-- Domains -->
    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold mb-2">Домены через туннель</h2>
      <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
        IP-адреса, полученные при разрешении этих доменов, будут направляться через туннель, даже если они попадают в диапазон домашней страны (один домен на строку).
      </p>
      <textarea
        v-model="domains"
        rows="8"
        class="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm font-mono"
        placeholder="netflix.com&#10;youtube.com"
      />
      <button
        @click="saveDomains"
        :disabled="savingDomains"
        class="mt-3 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 text-sm transition-colors"
      >
        {{ savingDomains ? 'Сохранение...' : 'Сохранить домены' }}
      </button>
    </div>

    <!-- Stats -->
    <div v-if="routingInfo?.stats" class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6">
      <h2 class="text-lg font-semibold mb-4">Статистика</h2>
      <div class="grid grid-cols-2 sm:grid-cols-3 gap-4 text-sm">
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">CIDR домашней страны</p>
          <p class="font-medium text-lg">{{ routingInfo.stats.domestic_entries }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">IP через туннель (DNS)</p>
          <p class="font-medium text-lg">{{ routingInfo.stats.tunnel_entries }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">dnsmasq</p>
          <p :class="['font-medium', routingInfo.stats.dnsmasq_running ? 'text-green-600 dark:text-green-400' : 'text-red-500']">
            {{ routingInfo.stats.dnsmasq_running ? 'Работает' : 'Остановлен' }}
          </p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Firewall backend</p>
          <p class="font-medium">{{ routingInfo.stats.fw_backend }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">NDMS</p>
          <p class="font-medium">{{ routingInfo.stats.ndms_major ? 'NDMS ' + routingInfo.stats.ndms_major : 'N/A' }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">Обновление GeoIP</p>
          <p class="font-medium">{{ routingInfo.stats.nets_updated || 'Не обновлялось' }}</p>
        </div>
      </div>
      <button
        @click="updateNets"
        :disabled="updatingNets"
        class="mt-4 px-4 py-2 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 text-sm transition-colors"
      >
        {{ updatingNets ? 'Обновление...' : 'Обновить GeoIP-списки' }}
      </button>
    </div>
  </div>
</template>
