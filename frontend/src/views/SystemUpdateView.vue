<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">版本更新</h1>
        <p class="page-subtitle">检查 GitHub Release，并通过 Docker Compose 重新构建当前部署。</p>
      </div>
      <div class="page-actions">
        <span class="status-pill" :data-tone="versionInfo?.update_available ? 'warning' : 'success'">
          {{ versionInfo?.update_available ? '有新版本' : '已是最新' }}
        </span>
        <GlassButton variant="secondary" :loading="loading" @click="loadVersion">检查更新</GlassButton>
        <GlassButton variant="primary" :loading="updating" :disabled="!versionInfo?.update_enabled" @click="startUpdate">一键更新</GlassButton>
      </div>
    </div>

    <div class="grid flex-1 gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
      <GlassCard>
        <div class="grid gap-4 md:grid-cols-2">
          <div class="metric-card app-card p-5" data-tone="info">
            <div class="text-sm text-steel">当前版本</div>
            <div class="mt-3 text-3xl font-black text-white">v{{ versionInfo?.current_version || '读取中' }}</div>
          </div>
          <div class="metric-card app-card p-5" :data-tone="versionInfo?.update_available ? 'warning' : 'success'">
            <div class="text-sm text-steel">最新版本</div>
            <div class="mt-3 text-3xl font-black text-white">v{{ versionInfo?.latest_version || '读取中' }}</div>
            <a v-if="versionInfo?.latest_url" class="mt-3 inline-block text-sm text-neon hover:underline" :href="versionInfo.latest_url" target="_blank" rel="noreferrer">查看发布说明</a>
          </div>
        </div>

        <div class="mt-5 rounded-2xl border border-white/10 bg-white/5 px-5 py-4 text-sm leading-7 text-steel">
          {{ updateHelpText }}
        </div>

        <div v-if="resultMessage" class="mt-4 rounded-2xl border border-neon/20 bg-neon/10 px-5 py-4 text-sm leading-7 text-cyan-100">
          {{ resultMessage }}
        </div>
      </GlassCard>

      <GlassCard>
        <div class="text-xs uppercase tracking-[0.16em] text-steel">Deploy Check</div>
        <h2 class="mt-2 text-xl font-black">更新后确认</h2>
        <div class="mt-5 space-y-3 text-sm leading-7 text-steel">
          <p>更新完成后访问 `/version.json`，返回版本号即代表前端容器已经运行到新版本。</p>
          <p>如果仍然返回 HTML，说明 1Panel 还在运行旧前端镜像，需要手动删除旧 `tg-frontend` 镜像后重建。</p>
        </div>
      </GlassCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type SystemVersion } from '../api/client'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { useUiStore } from '../stores/ui'

const ui = useUiStore()
const loading = ref(false)
const updating = ref(false)
const resultMessage = ref('')
const versionInfo = ref<SystemVersion | null>(null)

const updateHelpText = computed(() => {
  if (!versionInfo.value) return '正在读取当前部署版本。'
  if (!versionInfo.value.update_enabled) return '当前编排未启用一键更新。请先使用包含 tg-updater 的新版 docker-compose.yml 重建一次。'
  if (versionInfo.value.update_available) return '检测到新版本，可以点击一键更新。更新期间服务会短暂重启。'
  return '当前部署已经是最新版本，也可以手动检查 GitHub Release。'
})

async function loadVersion() {
  loading.value = true
  resultMessage.value = ''
  try {
    versionInfo.value = await api.systemVersion()
    ui.toast({
      title: versionInfo.value.update_available ? '发现新版本' : '当前已是最新版本',
      message: `当前 v${versionInfo.value.current_version}，最新 v${versionInfo.value.latest_version}`,
      tone: versionInfo.value.update_available ? 'warning' : 'success'
    })
  } catch (err) {
    ui.toast({
      title: '版本检查失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    loading.value = false
  }
}

async function startUpdate() {
  updating.value = true
  resultMessage.value = ''
  try {
    const result = await api.startSystemUpdate()
    resultMessage.value = result.message || '更新任务已启动，请稍后刷新页面。'
    ui.toast({
      title: '更新已启动',
      message: resultMessage.value,
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '启动更新失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    updating.value = false
  }
}

onMounted(loadVersion)
</script>
