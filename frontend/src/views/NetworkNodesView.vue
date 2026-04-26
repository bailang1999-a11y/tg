<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">网络节点</h1>
        <p class="page-subtitle">导入 socks5 / http 代理、分组归档和批量测试集中在同一个页面里，导入区和节点列表尽量吃满横向空间。</p>
      </div>
      <div class="page-actions">
        <GlassButton variant="secondary" :loading="loading" @click="load">刷新</GlassButton>
        <GlassButton variant="primary" :loading="testing" @click="batchTest">批量测试</GlassButton>
      </div>
    </div>

    <GlassCard>
      <div class="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_360px]">
        <div>
          <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 class="font-bold">导入代理</h2>
              <p class="mt-1 text-sm text-steel">一行一个代理；没有前缀时按默认协议处理</p>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-sm text-steel">默认协议</span>
              <select v-model="form.default_protocol" class="min-h-11 rounded-lg px-3 text-sm">
                <option value="socks5">socks5</option>
                <option value="http">http</option>
              </select>
            </div>
          </div>
          <textarea
            v-model="form.content"
            class="min-h-[24rem] w-full resize-y rounded-2xl p-3.5 text-sm leading-6 text-white"
            placeholder="socks5://1.2.3.4:1080&#10;http://user:pass@5.6.7.8:8080&#10;9.9.9.9:1080&#10;8.8.8.8:1080:user:pass"
          ></textarea>
          <div class="mt-3 flex flex-wrap items-center gap-3">
            <input ref="fileInput" class="hidden" type="file" accept=".txt,.csv,.list" @change="readFile" />
            <GlassButton variant="secondary" @click="fileInput?.click()">从文件读取</GlassButton>
            <GlassButton variant="primary" :disabled="!form.content.trim() || importing" :loading="importing" @click="importNodes">导入节点</GlassButton>
            <GlassButton variant="ghost" :disabled="!form.content.trim() || importing" @click="form.content = ''">清空文本</GlassButton>
          </div>
        </div>

        <div class="space-y-4">
          <div class="app-card p-4">
            <h3 class="font-bold">导入到分组</h3>
            <div class="mt-3 space-y-3">
              <label class="flex items-center gap-2 text-sm">
                <input v-model="targetMode" type="radio" value="existing" />
                <span>已有分组</span>
              </label>
              <select
                v-model="form.group_id"
                class="min-h-11 w-full rounded-lg px-3 text-sm"
                :disabled="targetMode !== 'existing'"
              >
                <option value="">不分组</option>
                <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
              </select>
              <label class="flex items-center gap-2 text-sm">
                <input v-model="targetMode" type="radio" value="new" />
                <span>新建分组</span>
              </label>
              <input
                v-model="form.new_group_name"
                class="min-h-11 w-full rounded-lg px-3 text-sm disabled:opacity-50"
                :disabled="targetMode !== 'new'"
                placeholder="输入新分组名称"
              />
            </div>
          </div>
          <div class="app-card p-4 text-sm leading-6 text-steel">
            <div class="font-bold text-white">支持格式</div>
            <p class="mt-2">`socks5://ip:port`、`http://ip:port`、`user:pass@ip:port`、`ip:port:user:pass`、`ip:port`。</p>
          </div>
        </div>
      </div>
    </GlassCard>

    <GlassCard v-if="summary">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-bold">导入结果</h2>
          <p class="mt-1 text-sm text-steel">目标分组：{{ summary.group_name || '不分组' }}</p>
        </div>
        <span class="status-pill" data-tone="success">成功 {{ summary.success }}</span>
      </div>
      <div class="grid gap-3 sm:grid-cols-4">
        <div v-for="card in resultCards" :key="card.label" class="metric-card app-card p-4" :data-tone="card.accent">
          <div class="text-sm text-steel">{{ card.label }}</div>
          <div class="mt-2 text-2xl font-black" :class="card.tone">{{ card.value }}</div>
        </div>
      </div>
      <div class="mt-5 max-h-80 overflow-auto rounded-2xl border border-white/10 scrollbar-thin">
        <div v-for="item in summary.items" :key="`${item.line}-${item.status}`" class="grid grid-cols-[1fr_100px_120px_1fr] gap-3 border-b border-white/8 px-3 py-2 text-sm">
          <span class="min-w-0 truncate">{{ item.line }}</span>
          <span>{{ item.protocol || '-' }}</span>
          <span :class="statusClass(item.status)">{{ item.status }}</span>
          <span class="min-w-0 truncate text-steel">{{ item.reason || item.address || '-' }}</span>
        </div>
      </div>
    </GlassCard>

    <GlassCard class="flex-1">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-3">
          <GroupSelect v-model="filterGroupID" :groups="groups" :loading="groupLoading" @create="createGroup" />
          <input v-model="nodeKeyword" class="min-h-10 rounded-lg px-3 text-sm" placeholder="搜索 IP / 协议 / 地区 / 状态" />
        </div>
        <span class="status-pill" data-tone="info">显示 {{ pagedNodes.length }} / {{ filteredNodes.length }}，总数 {{ nodes.length }}</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[960px] text-left text-sm">
          <thead class="text-steel">
            <tr>
              <th class="py-2">编号</th>
              <th>IP</th>
              <th>端口</th>
              <th>协议</th>
              <th>用户名</th>
              <th>延迟</th>
              <th>地理位置</th>
              <th>绑定终端数</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="node in pagedNodes" :key="node.id" class="border-t border-white/8">
              <td class="py-3 font-semibold">{{ node.code }}</td>
              <td>{{ node.ip }}</td>
              <td>{{ node.port }}</td>
              <td>{{ displayProtocol(node.protocol) }}</td>
              <td>{{ node.username || '-' }}</td>
              <td>{{ node.latency_ms ? `${node.latency_ms}ms` : '-' }}</td>
              <td>{{ node.country || '未知' }} {{ node.flag }}</td>
              <td>{{ node.bound_terminals }}</td>
              <td><span class="status-pill" :data-tone="nodeTone(node.status || 'untested')">{{ node.status || 'untested' }}</span></td>
            </tr>
          </tbody>
        </table>
        <div v-if="!filteredNodes.length" class="py-8 text-center text-sm text-steel">暂无网络节点</div>
      </div>
      <div v-if="nodePageCount > 1" class="mt-4 flex flex-wrap items-center justify-end gap-2 text-sm text-steel">
        <span>第 {{ nodePage }} / {{ nodePageCount }} 页</span>
        <GlassButton size="sm" variant="secondary" :disabled="nodePage <= 1" @click="nodePage--">上一页</GlassButton>
        <GlassButton size="sm" variant="secondary" :disabled="nodePage >= nodePageCount" @click="nodePage++">下一页</GlassButton>
      </div>
    </GlassCard>

    <p v-if="error" class="rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import GroupSelect from '../components/GroupSelect.vue'
import { api, type Group, type NetworkNode, type ProxyImportSummary } from '../api/client'

const nodes = ref<NetworkNode[]>([])
const groups = ref<Group[]>([])
const summary = ref<ProxyImportSummary | null>(null)
const filterGroupID = ref('')
const nodeKeyword = ref('')
const nodePage = ref(1)
const nodePageSize = 100
const targetMode = ref<'existing' | 'new'>('existing')
const loading = ref(false)
const importing = ref(false)
const testing = ref(false)
const groupLoading = ref(false)
const error = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const form = reactive({
  content: '',
  default_protocol: 'socks5',
  group_id: '',
  new_group_name: ''
})

const resultCards = computed(() => {
  const data = summary.value
  if (!data) return []
  return [
    { label: '成功', value: data.success, tone: 'text-neon', accent: 'success' },
    { label: '失败', value: data.failed, tone: 'text-danger', accent: 'danger' },
    { label: '重复', value: data.duplicate, tone: 'text-amber', accent: 'warning' },
    { label: '跳过', value: data.skipped, tone: 'text-steel', accent: 'info' }
  ]
})
const filteredNodes = computed(() => {
  const keyword = nodeKeyword.value.trim().toLowerCase()
  if (!keyword) return nodes.value
  return nodes.value.filter((node) => {
    return [
      node.code,
      node.ip,
      node.port,
      node.protocol,
      node.username,
      node.country,
      node.flag,
      node.status
    ].some((value) => String(value ?? '').toLowerCase().includes(keyword))
  })
})
const nodePageCount = computed(() => Math.max(1, Math.ceil(filteredNodes.value.length / nodePageSize)))
const pagedNodes = computed(() => {
  const start = (nodePage.value - 1) * nodePageSize
  return filteredNodes.value.slice(start, start + nodePageSize)
})

async function load() {
  loading.value = true
  try {
    const [groupData, nodeData] = await Promise.all([api.groups('network'), api.networkNodes(filterGroupID.value)])
    groups.value = groupData
    nodes.value = nodeData
  } finally {
    loading.value = false
  }
}

async function createGroup(name: string) {
  groupLoading.value = true
  try {
    await api.createGroup('network', name)
    groups.value = await api.groups('network')
  } finally {
    groupLoading.value = false
  }
}

async function importNodes() {
  importing.value = true
  error.value = ''
  summary.value = null
  try {
    const payload = {
      content: form.content,
      default_protocol: form.default_protocol,
      group_id: targetMode.value === 'existing' ? form.group_id : '',
      new_group_name: targetMode.value === 'new' ? form.new_group_name.trim() : ''
    }
    summary.value = await api.importNetworkNodes(payload)
    form.content = ''
    form.new_group_name = ''
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '导入失败'
  } finally {
    importing.value = false
  }
}

async function batchTest() {
  testing.value = true
  try {
    await api.createTask('/api/v1/network-nodes/test')
  } finally {
    testing.value = false
  }
}

async function readFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  form.content = await file.text()
  input.value = ''
}

function displayProtocol(protocol: string) {
  return protocol
}

function statusClass(status: string) {
  if (status === 'success') return 'text-neon'
  if (status === 'failed') return 'text-danger'
  if (status === 'duplicate') return 'text-amber'
  return 'text-steel'
}

function nodeTone(status: string) {
  if (status === 'online' || status === 'ok' || status === 'available') return 'success'
  if (status === 'failed' || status === 'error' || status === 'blocked') return 'danger'
  if (status === 'testing' || status === 'pending') return 'warning'
  return 'info'
}

watch(filterGroupID, load)
watch([filterGroupID, nodeKeyword], () => {
  nodePage.value = 1
})
watch(nodePageCount, (count) => {
  if (nodePage.value > count) nodePage.value = count
})
watch(targetMode, (mode) => {
  if (mode === 'existing') {
    form.new_group_name = ''
  } else {
    form.group_id = ''
  }
})

onMounted(load)
</script>
