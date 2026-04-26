<template>
  <div class="app-shell text-white">
    <aside class="app-sidebar hidden lg:flex">
      <div class="sidebar-brand">
        <div class="flex items-start gap-3">
          <div class="sidebar-brand-mark">C3</div>
          <div class="min-w-0">
            <div class="text-lg font-black text-white">Codex3</div>
            <div class="mt-1 text-sm text-steel">多账号自动化运营平台</div>
          </div>
        </div>
        <div class="grid gap-2 text-xs text-steel">
          <div class="flex items-center justify-between gap-3 rounded-xl border border-white/8 bg-white/5 px-3 py-2">
            <span>当前租户</span>
            <span class="max-w-[8.5rem] truncate text-white">{{ store.user?.username || '未登录' }}</span>
          </div>
          <div class="flex items-center justify-between gap-3 rounded-xl border border-white/8 bg-white/5 px-3 py-2">
            <span>身份</span>
            <span class="status-pill" data-tone="info">{{ store.user?.role || 'guest' }}</span>
          </div>
        </div>
      </div>
      <div class="sidebar-section-label">Console</div>
      <nav class="space-y-1.5">
        <RouterLink
          v-for="item in visibleNav"
          :key="item.path"
          :to="item.path"
          custom
          v-slot="{ href, navigate, isActive }"
        >
          <a :href="href" :class="['nav-link', { 'nav-link-active': isActive }]" @click="navigate">
            <span class="nav-icon">{{ item.icon }}</span>
            <span>{{ item.label }}</span>
          </a>
        </RouterLink>
      </nav>
      <div class="mt-auto pt-5">
        <GlassButton class="w-full" variant="ghost" @click="logout">退出登录</GlassButton>
      </div>
    </aside>

    <main class="app-main lg:pl-[17rem]">
      <header class="app-header">
        <div class="app-header-inner py-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div class="text-xs uppercase text-steel">Operations Console</div>
              <div class="mt-1 break-all text-sm font-semibold text-white">{{ store.user?.username || '未登录' }}</div>
            </div>
            <div class="flex flex-wrap items-center gap-2.5">
              <span class="status-pill" :data-tone="store.user?.role === 'admin' ? 'success' : 'info'">{{ store.user?.role || 'guest' }}</span>
              <GlassButton variant="secondary" size="sm" @click="logout">退出</GlassButton>
            </div>
          </div>
          <div class="mt-4 flex gap-2 overflow-x-auto pb-1 lg:hidden">
            <RouterLink
              v-for="item in visibleNav"
              :key="item.path"
              :to="item.path"
              custom
              v-slot="{ href, navigate, isActive }"
            >
              <a :href="href" :class="['nav-link shrink-0', { 'nav-link-active': isActive }]" @click="navigate">
                <span class="nav-icon">{{ item.icon }}</span>
                <span>{{ item.label }}</span>
              </a>
            </RouterLink>
          </div>
        </div>
      </header>
      <div class="app-content app-content-inner">
        <RouteErrorBoundary>
          <RouterView />
        </RouteErrorBoundary>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import GlassButton from './GlassButton.vue'
import RouteErrorBoundary from './RouteErrorBoundary.vue'

const store = useUserStore()
const router = useRouter()

const nav = [
  { label: 'Dashboard', path: '/dashboard', icon: '⌂' },
  { label: '账号管理', path: '/terminals', icon: '☏' },
  { label: '导入中心', path: '/import', icon: '⇪' },
  { label: '网络节点', path: '/network', icon: '◈' },
  { label: '目标池', path: '/targets', icon: '◎' },
  { label: '通知工作流', path: '/workflow', icon: '✦' },
  { label: '资料与素材', path: '/profile-assets', icon: '▣' },
  { label: '任务', path: '/tasks', icon: '☷' },
  { label: '日志', path: '/logs', icon: '☰' },
  { label: '主动触达', path: '/outreach-sync', icon: '◌' },
  { label: '监听私信', path: '/direct-messages', icon: '✉' },
  { label: 'Bot 配置', path: '/bot-settings', icon: '✺', admin: true },
  { label: 'Bot 用户看板', path: '/bot-users', icon: '☻', admin: true },
  { label: '监听矩阵', path: '/listener-admin', icon: '⟡', admin: true },
  { label: '用户管理', path: '/users', icon: '♟', admin: true },
  { label: '系统设置', path: '/settings', icon: '⚙', admin: true }
]

const visibleNav = computed(() => nav.filter((item) => !item.admin || store.user?.role === 'admin'))

function logout() {
  store.logout()
  router.push('/login')
}
</script>
