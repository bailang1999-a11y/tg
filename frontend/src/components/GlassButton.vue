<template>
  <button
    :class="buttonClass"
    :disabled="disabled || loading"
    :type="type"
  >
    <span v-if="loading" class="h-4 w-4 animate-spin rounded-full border-2 border-ice/30 border-t-ice"></span>
    <slot />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    loading?: boolean
    disabled?: boolean
    type?: 'button' | 'submit' | 'reset'
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'success'
    size?: 'sm' | 'md' | 'lg'
  }>(),
  {
    type: 'button',
    variant: 'secondary',
    size: 'md'
  }
)

const buttonClass = computed(() => [
  'app-btn',
  {
    'app-btn-primary': props.variant === 'primary',
    'app-btn-secondary': props.variant === 'secondary',
    'app-btn-ghost': props.variant === 'ghost',
    'app-btn-danger': props.variant === 'danger',
    'app-btn-success': props.variant === 'success',
    'app-btn-sm': props.size === 'sm',
    'app-btn-lg': props.size === 'lg'
  }
])
</script>
