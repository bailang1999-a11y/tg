<template>
  <Teleport to="body">
    <div v-if="store.toasts.length" class="toast-stack">
      <div v-for="toast in store.toasts" :key="toast.id" class="toast-card" :data-tone="toast.tone">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div v-if="toast.title" class="text-sm font-semibold text-white">{{ toast.title }}</div>
            <div class="text-sm leading-6 text-steel">{{ toast.message }}</div>
          </div>
          <button class="action-link h-8 min-h-0 shrink-0 px-3 py-0 text-xs" type="button" @click="store.dismissToast(toast.id)">关闭</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useUiStore } from '../stores/ui'

const store = useUiStore()
</script>
