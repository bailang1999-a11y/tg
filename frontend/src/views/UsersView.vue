<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">用户管理</h1>
        <p class="page-subtitle">管理员创建租户用户并控制启用状态，创建表单和用户列表按更完整的工作区布局展开。</p>
      </div>
      <div class="page-actions">
        <span class="status-pill" data-tone="info">共 {{ users.length }} 个用户</span>
      </div>
    </div>

    <GlassCard>
      <form class="grid gap-3 xl:grid-cols-[1fr_1fr_1fr_160px_1fr_140px_auto]" @submit.prevent="create">
        <input v-model="form.username" class="min-h-11 rounded-lg px-3" placeholder="用户名" />
        <input v-model="form.email" class="min-h-11 rounded-lg px-3" placeholder="邮箱" />
        <input v-model="form.password" type="password" class="min-h-11 rounded-lg px-3" placeholder="初始密码" />
        <select v-model="form.role" class="min-h-11 rounded-lg px-3">
          <option value="user">普通用户</option>
          <option value="admin">管理员</option>
        </select>
        <input v-model="form.telegram_user_id" class="min-h-11 rounded-lg px-3" placeholder="绑定 Telegram ID" />
        <input v-model.number="form.trial_days" min="0" type="number" class="min-h-11 rounded-lg px-3" placeholder="试用天数" />
        <GlassButton type="submit" variant="primary" :loading="loading">创建</GlassButton>
      </form>
      <p v-if="error" class="mt-3 text-sm text-danger">{{ error }}</p>
    </GlassCard>

    <GlassCard class="flex-1">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[780px] text-left text-sm">
          <thead class="text-steel">
            <tr>
              <th class="py-2">用户名</th>
              <th>邮箱</th>
              <th>角色</th>
              <th>状态</th>
              <th>Telegram ID</th>
              <th>试用到期</th>
              <th>租户 ID</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="user in users" :key="user.id" class="border-t border-white/8">
              <td class="py-3 font-semibold">{{ user.username }}</td>
              <td>{{ user.email }}</td>
              <td>{{ user.role }}</td>
              <td>
                <span class="status-pill" :data-tone="user.status === 'active' ? 'success' : 'danger'">{{ user.status }}</span>
              </td>
              <td>
                <input v-model="bindForms[user.id].telegram_user_id" class="min-h-9 w-36 rounded-lg px-2" placeholder="Telegram ID" />
              </td>
              <td>
                <input v-model.number="bindForms[user.id].trial_days" class="min-h-9 w-24 rounded-lg px-2" min="0" type="number" placeholder="天" />
                <div class="text-xs text-steel">{{ formatDateTime(user.trial_ends_at) }}</div>
              </td>
              <td class="max-w-[240px] truncate text-steel">{{ user.tenant_id }}</td>
              <td>
                <button class="action-link text-ice" @click="toggle(user)">{{ user.status === 'active' ? '禁用' : '启用' }}</button>
                <button class="action-link ml-3 text-emerald-300" @click="bindTelegram(user)">绑定</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!users.length" class="py-8 text-center text-sm text-steel">暂无用户</div>
      </div>
    </GlassCard>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { api, type User } from '../api/client'

const users = ref<User[]>([])
const loading = ref(false)
const error = ref('')
const form = reactive({ username: '', email: '', password: '', role: 'user', telegram_user_id: '', trial_days: 0 })
const bindForms = reactive<Record<string, { telegram_user_id: string; trial_days: number }>>({})

async function load() {
  users.value = await api.users()
  for (const user of users.value) {
    bindForms[user.id] = {
      telegram_user_id: user.telegram_user_id || '',
      trial_days: 0
    }
  }
}

async function create() {
  loading.value = true
  error.value = ''
  try {
    await api.createUser(form)
    Object.assign(form, { username: '', email: '', password: '', role: 'user', telegram_user_id: '', trial_days: 0 })
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '创建失败'
  } finally {
    loading.value = false
  }
}

async function toggle(user: User) {
  await api.updateUserStatus(user.id, user.status === 'active' ? 'disabled' : 'active')
  await load()
}

async function bindTelegram(user: User) {
  await api.bindUserTelegram(user.id, bindForms[user.id])
  await load()
}

function formatDateTime(value?: string | null) {
  if (!value) return '--'
  return new Date(value).toLocaleString()
}

onMounted(load)
</script>
