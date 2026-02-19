<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

const props = defineProps<{
  lines: string[]
  autoScroll?: boolean
}>()

const container = ref<HTMLElement | null>(null)
const shouldAutoScroll = ref(props.autoScroll !== false)

watch(() => props.lines.length, async () => {
  if (shouldAutoScroll.value && container.value) {
    await nextTick()
    container.value.scrollTop = container.value.scrollHeight
  }
})

function onScroll() {
  if (!container.value) return
  const { scrollTop, scrollHeight, clientHeight } = container.value
  shouldAutoScroll.value = scrollHeight - scrollTop - clientHeight < 50
}
</script>

<template>
  <div
    ref="container"
    @scroll="onScroll"
    class="bg-gray-950 text-green-400 font-mono text-xs leading-5 p-4 rounded-lg overflow-auto max-h-[500px] whitespace-pre-wrap"
  >
    <div v-if="lines.length === 0" class="text-gray-600">Нет данных</div>
    <div v-for="(line, i) in lines" :key="i">{{ line }}</div>
  </div>
</template>
