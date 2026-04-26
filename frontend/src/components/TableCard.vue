<template>
  <GlassCard class="min-h-0">
    <div v-if="!props.empty" class="overflow-x-auto">
      <slot />
    </div>
    <div v-else class="py-10 text-center text-sm text-steel">
      {{ props.emptyText }}
    </div>
  </GlassCard>
</template>

<script setup lang="ts">
import GlassCard from './GlassCard.vue'

const props = withDefaults(
  defineProps<{
    empty?: boolean
    emptyText?: string
  }>(),
  {
    empty: false,
    emptyText: '暂无数据'
  }
)
</script>
