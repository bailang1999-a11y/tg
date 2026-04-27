<template>
  <main class="login-page relative flex min-h-screen items-center justify-center overflow-hidden px-4 py-8 lg:px-8">
    <img
      class="login-bg absolute inset-0 h-full w-full object-cover"
      src="https://images.unsplash.com/photo-1611605698335-8b1569810432?auto=format&fit=crop&w=2000&q=85"
      alt="TG 营销助手背景"
    />
    <div class="login-overlay absolute inset-0"></div>

    <div class="relative z-10 grid w-full max-w-6xl items-center gap-8 lg:grid-cols-[1fr_440px]">
      <section class="login-brand">
        <div class="brand-mark-row">
          <div class="brand-mark">TG</div>
          <div>
            <div class="brand-kicker">Telegram Growth Console</div>
            <div class="brand-name">TG 营销助手</div>
          </div>
        </div>

        <h1>TG 营销助手</h1>
        <p>账号导入、目标群组、私信群发、监听设置和资料修改，统一进入同一个后台。</p>

        <div class="brand-strip" aria-label="核心模块">
          <span>账号管理</span>
          <span>私信群发</span>
          <span>监听设置</span>
          <span>版本更新</span>
        </div>
      </section>

      <section class="flex items-center justify-center">
        <GlassCard class="login-card w-full" padding="lg" tone="cyan">
          <div class="login-card-head">
            <div class="login-card-icon">TG</div>
            <div>
              <div class="text-xs font-semibold uppercase tracking-[0.16em] text-steel">Secure Access</div>
              <div class="mt-2 text-3xl font-black text-white">登录 TG 营销助手</div>
            </div>
          </div>
          <form class="space-y-4" @submit.prevent="submit">
            <label class="block">
              <span class="text-sm text-steel">用户名</span>
              <input v-model="username" autocomplete="username" class="mt-2 min-h-12 w-full rounded-lg px-3.5 text-white" placeholder="请输入用户名" />
            </label>
            <label class="block">
              <span class="text-sm text-steel">密码</span>
              <input v-model="password" autocomplete="current-password" type="password" class="mt-2 min-h-12 w-full rounded-lg px-3.5 text-white" placeholder="请输入密码" />
            </label>
            <p v-if="error" class="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger">{{ error }}</p>
            <GlassButton class="w-full" type="submit" variant="primary" size="lg" :loading="loading">进入后台</GlassButton>
          </form>
          <div class="login-note mt-5 rounded-2xl border border-white/8 bg-white/5 px-4 py-3 text-xs leading-6 text-steel">
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

<style scoped>
.login-page {
  background: #050a12;
}

.login-bg {
  opacity: 0.46;
  filter: saturate(1.08) contrast(1.05);
}

.login-overlay {
  background:
    linear-gradient(90deg, rgba(4, 10, 18, 0.94) 0%, rgba(7, 14, 24, 0.74) 44%, rgba(6, 11, 20, 0.88) 100%),
    radial-gradient(circle at 70% 24%, rgba(41, 211, 255, 0.18), transparent 30%),
    radial-gradient(circle at 28% 76%, rgba(92, 255, 108, 0.13), transparent 28%);
}

.login-brand {
  color: white;
  min-width: 0;
}

.brand-mark-row {
  display: inline-flex;
  align-items: center;
  gap: 1rem;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  border-radius: 1.25rem;
  padding: 0.85rem 1rem;
  box-shadow: 0 18px 60px rgba(0, 0, 0, 0.26);
}

.brand-mark,
.login-card-icon {
  display: grid;
  place-items: center;
  width: 3.25rem;
  height: 3.25rem;
  border-radius: 1rem;
  background: linear-gradient(135deg, #45f06a, #2fc8ff);
  color: #05111c;
  font-weight: 950;
  box-shadow: 0 18px 44px rgba(47, 200, 255, 0.26);
}

.brand-kicker {
  color: rgba(207, 220, 238, 0.72);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.brand-name {
  margin-top: 0.2rem;
  font-size: 1.05rem;
  font-weight: 900;
}

.login-brand h1 {
  margin-top: 2.2rem;
  max-width: 680px;
  font-size: clamp(3.4rem, 8vw, 6.7rem);
  line-height: 0.98;
  font-weight: 950;
  letter-spacing: 0;
}

.login-brand p {
  margin-top: 1.4rem;
  max-width: 620px;
  color: rgba(207, 220, 238, 0.82);
  font-size: 1.08rem;
  line-height: 2;
}

.brand-strip {
  margin-top: 2rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.brand-strip span {
  border: 1px solid rgba(255, 255, 255, 0.11);
  background: rgba(7, 18, 30, 0.62);
  border-radius: 999px;
  padding: 0.65rem 0.95rem;
  color: rgba(231, 238, 248, 0.88);
  font-size: 0.9rem;
  font-weight: 800;
}

.login-card {
  max-width: 440px;
  border-radius: 1.35rem;
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.38);
}

.login-card-head {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 2rem;
}

.login-card :deep(input) {
  border-color: rgba(255, 255, 255, 0.13);
  background: rgba(8, 18, 32, 0.72);
}

.login-card :deep(input:focus) {
  border-color: rgba(45, 216, 255, 0.72);
  box-shadow: 0 0 0 3px rgba(45, 216, 255, 0.12);
}

.login-note {
  background: rgba(255, 255, 255, 0.045);
}

@media (max-width: 1023px) {
  .login-overlay {
    background:
      linear-gradient(180deg, rgba(4, 10, 18, 0.88) 0%, rgba(4, 10, 18, 0.94) 100%),
      radial-gradient(circle at 50% 18%, rgba(41, 211, 255, 0.18), transparent 34%);
  }

  .login-brand {
    text-align: left;
  }

  .login-brand h1 {
    font-size: clamp(2.9rem, 13vw, 4.6rem);
  }
}
</style>
