<template>
  <main class="relative flex min-h-screen items-center justify-center overflow-hidden px-4 py-8 lg:px-8">
    <img
      class="absolute inset-0 h-full w-full object-cover opacity-35"
      src="https://images.unsplash.com/photo-1518709268805-4e9042af2176?auto=format&fit=crop&w=1800&q=80"
      alt="控制台背景"
    />
    <div class="absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(79,172,254,0.22),transparent_35%),linear-gradient(180deg,rgba(11,15,25,0.38),rgba(11,15,25,0.86))]"></div>

    <div class="relative z-10 grid w-full max-w-6xl gap-6 lg:grid-cols-[1.08fr_0.92fr]">
      <section class="glass-panel hidden rounded-[28px] p-7 lg:flex lg:flex-col lg:justify-between">
        <div>
          <div class="inline-flex items-center gap-3 rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm text-steel">
            <span class="status-dot bg-neon"></span>
            <span>Operations Console</span>
          </div>
          <div class="mt-7 max-w-xl">
            <h1 class="text-5xl font-black leading-[1.02] text-white">让导入、终端、任务和资料流转在一个控制台里。</h1>
            <p class="mt-5 text-base leading-8 text-steel">
              多账号接入、节点分组、目标池、通知工作流与资料修改，统一收拢到同一套玻璃化工作台。
            </p>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-3">
          <div class="app-card p-4">
            <div class="text-xs uppercase tracking-[0.16em] text-steel">Import</div>
            <div class="mt-3 text-xl font-black text-white">Session / TData</div>
          </div>
          <div class="app-card p-4">
            <div class="text-xs uppercase tracking-[0.16em] text-steel">Profiles</div>
            <div class="mt-3 text-xl font-black text-white">昵称 / 签名 / 频道 / 头像</div>
          </div>
          <div class="app-card p-4">
            <div class="text-xs uppercase tracking-[0.16em] text-steel">Tasks</div>
            <div class="mt-3 text-xl font-black text-white">真实状态与日志回流</div>
          </div>
        </div>
      </section>

      <section class="flex items-center justify-center">
        <GlassCard class="w-full max-w-lg" padding="lg" tone="cyan">
          <div class="mb-8">
            <div class="text-xs font-semibold uppercase tracking-[0.16em] text-steel">Secure Access</div>
            <div class="mt-3 text-3xl font-black text-white">登录运营控制台</div>
            <p class="mt-3 text-sm leading-7 text-steel">输入账号信息后直接进入控制台，现有功能和数据保持不变。</p>
          </div>
          <form class="space-y-4" @submit.prevent="submit">
            <label class="block">
              <span class="text-sm text-steel">用户名</span>
              <input v-model="username" class="mt-2 min-h-12 w-full rounded-lg px-3.5 text-white" />
            </label>
            <label class="block">
              <span class="text-sm text-steel">密码</span>
              <input v-model="password" type="password" class="mt-2 min-h-12 w-full rounded-lg px-3.5 text-white" />
            </label>
            <p v-if="error" class="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger">{{ error }}</p>
            <GlassButton class="w-full" type="submit" variant="primary" size="lg" :loading="loading">进入控制台</GlassButton>
          </form>
          <div class="mt-5 rounded-2xl border border-white/8 bg-white/5 px-4 py-3 text-xs leading-6 text-steel">
            请使用管理员分配的生产账号登录。上线环境不要保留默认口令。
          </div>
        </GlassCard>
      </section>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { useUserStore } from '../stores/user'

const store = useUserStore()
const router = useRouter()
const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

async function submit() {
  loading.value = true
  error.value = ''
  try {
    await store.login(username.value, password.value)
    await router.push('/dashboard')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>
