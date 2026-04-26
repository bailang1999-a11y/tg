<template>
  <GlassCard>
    <div :class="props.gridClass">
      <slot />
    </div>
  </GlassCard>
</template>

<script setup lang="ts">
import GlassCard from './GlassCard.vue'

const props = withDefaults(
  defineProps<{
    gridClass?: string
  }>(),
  {
    gridClass: 'grid gap-3 md:grid-cols-4'
  }
)
</script>
