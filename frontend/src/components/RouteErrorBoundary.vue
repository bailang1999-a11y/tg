<template>
  <slot v-if="!message" />
  <div v-else class="route-error-panel">
    <div>
      <div class="text-xs uppercase tracking-[0.16em] text-steel">Runtime Guard</div>
      <h2 class="mt-2 text-2xl font-black text-white">页面加载失败</h2>
      <p class="mt-3 text-sm leading-6 text-steel">{{ message }}</p>
    </div>
    <div class="mt-5 flex flex-wrap gap-2">
      <GlassButton variant="primary" size="sm" @click="reload">重新加载</GlassButton>
      <GlassButton variant="secondary" size="sm" @click="clearAndReload">清理缓存后刷新</GlassButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onErrorCaptured, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import GlassButton from './GlassButton.vue'

const route = useRoute()
const message = ref('')

onErrorCaptured((err) => {
  message.value = err instanceof Error ? err.message : '当前页面发生运行时错误。'
  return false
})

watch(
  () => route.fullPath,
  () => {
    message.value = ''
  }
)

function reload() {
  window.location.reload()
}

function clearAndReload() {
  localStorage.removeItem('codex3_route_cache_buster')
  window.location.reload()
}
</script>

<style scoped>
.route-error-panel {
  min-height: 18rem;
  border: 1px solid rgba(248, 113, 113, 0.32);
  border-radius: 8px;
  background:
    linear-gradient(145deg, rgba(127, 29, 29, 0.24), rgba(8, 13, 28, 0.94)),
    rgba(15, 23, 42, 0.86);
  padding: 1.4rem;
  box-shadow: 0 22px 60px rgba(0, 0, 0, 0.28);
}
</style>
