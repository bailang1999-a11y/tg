<template>
  <div class="flex flex-wrap items-center gap-2.5">
    <select
      class="min-h-11 min-w-[11rem] rounded-lg px-3 text-sm"
      :value="modelValue"
      @change="$emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
    >
      <option value="">全部分组</option>
      <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
    </select>
    <input
      v-model="name"
      class="min-h-11 w-40 rounded-lg px-3 text-sm"
      placeholder="新建分组"
    />
    <GlassButton size="sm" :loading="loading" @click="create">新建</GlassButton>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import GlassButton from './GlassButton.vue'
import type { Group } from '../api/client'

const props = defineProps<{
  modelValue: string
  groups: Group[]
  loading?: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [value: string]
  create: [name: string]
}>()

const name = ref('')

function create() {
  if (!name.value.trim() || props.loading) return
  emit('create', name.value.trim())
  name.value = ''
}
</script>
