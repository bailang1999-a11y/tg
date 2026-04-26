<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">{{ title }}</h1>
        <p class="page-subtitle">{{ subtitle }}</p>
      </div>
      <div class="page-actions">
        <GlassButton variant="primary" :loading="loading" @click="createTask">{{ actionLabel }}</GlassButton>
      </div>
    </div>

    <GlassCard v-if="steps?.length">
      <h2 class="mb-4 font-bold">流程阶段</h2>
      <div class="grid gap-3 md:grid-cols-3">
        <div v-for="(step, index) in steps" :key="step" class="rounded-2xl border border-white/10 bg-white/5 p-4">
          <div class="text-xs text-steel">Step {{ index + 1 }}</div>
          <div class="mt-1 font-semibold">{{ step }}</div>
        </div>
      </div>
    </GlassCard>

    <GlassCard>
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 class="font-bold">数据列表</h2>
        <input class="min-h-11 rounded-lg px-3 text-sm" placeholder="搜索 ID / 名称" />
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[760px] text-left text-sm">
          <thead class="text-steel">
            <tr>
              <th v-for="column in safeColumns" :key="column" class="py-2">{{ column }}</th>
            </tr>
          </thead>
          <tbody>
            <tr class="border-t border-white/8">
              <td v-for="column in safeColumns" :key="column" class="py-4 text-steel">待接入</td>
            </tr>
          </tbody>
        </table>
      </div>
    </GlassCard>

    <p v-if="message" class="rounded-lg border border-neon/30 bg-neon/10 px-4 py-3 text-sm text-neon">{{ message }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { api } from '../api/client'
import { taskDisplayName } from '../utils/taskDisplay'

const props = defineProps<{
  title: string
  subtitle: string
  actionLabel: string
  actionPath: string
  steps?: string[]
  columns?: string[]
}>()

const loading = ref(false)
const message = ref('')
const safeColumns = computed(() => props.columns?.length ? props.columns : ['名称', '类型', '状态', '进度', '创建时间'])

async function createTask() {
  loading.value = true
  message.value = ''
  try {
    const task = await api.createTask(props.actionPath)
    message.value = `任务已创建：${taskDisplayName(task)}`
  } catch (err) {
    message.value = err instanceof Error ? err.message : '操作失败'
  } finally {
    loading.value = false
  }
}
</script>
