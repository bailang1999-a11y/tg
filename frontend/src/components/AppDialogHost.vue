<template>
  <Teleport to="body">
    <Transition name="dialog">
      <div v-if="store.confirmState" class="modal-backdrop" @click.self="store.settleConfirm(false)">
        <div class="modal-card p-6 lg:p-7">
          <div class="flex items-start gap-4">
            <div class="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-white/8">
              <span class="status-dot" :class="toneClass"></span>
            </div>
            <div class="min-w-0 flex-1">
              <h2 class="text-xl font-black text-white">{{ store.confirmState.title }}</h2>
              <p class="mt-3 text-sm leading-7 text-steel">{{ store.confirmState.message }}</p>
            </div>
          </div>

          <div class="mt-7 flex flex-wrap justify-end gap-3">
            <GlassButton variant="ghost" @click="store.settleConfirm(false)">{{ store.confirmState.cancelText }}</GlassButton>
            <GlassButton :variant="store.confirmState.tone === 'error' ? 'danger' : 'primary'" @click="store.settleConfirm(true)">
              {{ store.confirmState.confirmText }}
            </GlassButton>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import GlassButton from './GlassButton.vue'
import { useUiStore } from '../stores/ui'

const store = useUiStore()

const toneClass = computed(() => {
  const tone = store.confirmState?.tone
  if (tone === 'success') return 'bg-success'
  if (tone === 'warning') return 'bg-amber'
  if (tone === 'error') return 'bg-danger'
  return 'bg-neon'
})
</script>
