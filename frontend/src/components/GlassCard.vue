<template>
  <section :class="cardClass">
    <slot />
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    padding?: 'none' | 'sm' | 'md' | 'lg'
    tone?: 'default' | 'cyan' | 'violet'
  }>(),
  {
    padding: 'md',
    tone: 'default'
  }
)

const cardClass = computed(() => [
  'app-card',
  props.padding === 'sm' ? 'p-4' : '',
  props.padding === 'md' ? 'p-5 lg:p-6' : '',
  props.padding === 'lg' ? 'p-6 lg:p-7' : '',
  props.padding === 'none' ? 'p-0' : '',
  props.tone === 'violet' ? 'shadow-violet' : '',
  props.tone === 'cyan' ? 'shadow-neon' : '',
  props.tone === 'default' ? 'shadow-glass' : ''
])
</script>
