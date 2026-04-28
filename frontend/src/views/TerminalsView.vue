<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">账号管理</h1>
        <p class="page-subtitle">账号状态和在线状态已拆开：状态表示账号是否可用，心跳表示当前在线/离线与最后在线时间。</p>
      </div>
      <div class="page-actions">
        <GlassButton variant="secondary" :loading="loading" @click="load">刷新</GlassButton>
        <GlassButton variant="secondary" @click="view = view === 'table' ? 'card' : 'table'">{{ view === 'table' ? '卡片视图' : '列表视图' }}</GlassButton>
        <GlassButton variant="primary" :loading="checking && checkScope === 'all'" @click="check('all')">一键检测账号状态</GlassButton>
      </div>
    </div>

    <div class="grid flex-1 gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
      <GlassCard class="flex min-h-[68vh] flex-col overflow-hidden">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div class="text-xs uppercase tracking-[0.16em] text-steel">Terminal Workspace</div>
            <h2 class="mt-2 text-xl font-black">终端列表</h2>
            <p class="mt-2 text-sm text-steel">点击任意一行可高亮并在右侧查看详情，列表本身已按你要求切成标准化双行信息块。</p>
          </div>
          <GroupSelect v-model="groupID" :groups="groups" :loading="groupLoading" @create="createGroup" />
        </div>

        <div class="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)]">
          <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
            <div class="text-sm text-steel">搜索账号</div>
            <input v-model="keyword" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" placeholder="手机号 / 昵称 / 个性签名 / 个人频道" />
          </label>

          <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">在线筛选</div>
              <div class="mt-3 flex flex-wrap gap-2">
                <button
                  v-for="item in statusFilters"
                  :key="item.value"
                  type="button"
                  class="terminal-filter-chip"
                  :class="{ 'terminal-filter-chip-active': statusFilter === item.value }"
                  @click="statusFilter = item.value"
                >
                  {{ item.label }}
                </button>
              </div>
            </div>

            <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">接入筛选</div>
              <div class="mt-3 flex flex-wrap gap-2">
                <button
                  v-for="item in accessFilters"
                  :key="item.value"
                  type="button"
                  class="terminal-filter-chip"
                  :class="{ 'terminal-filter-chip-active': accessFilter === item.value }"
                  @click="accessFilter = item.value"
                >
                  {{ item.label }}
                </button>
              </div>
            </div>

            <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">风险筛选</div>
              <div class="mt-3 flex flex-wrap gap-2">
                <button
                  v-for="item in riskFilters"
                  :key="item.value"
                  type="button"
                  class="terminal-filter-chip"
                  :class="{ 'terminal-filter-chip-active': riskFilter === item.value }"
                  @click="riskFilter = item.value"
                >
                  {{ item.label }}
                </button>
              </div>
            </div>

            <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">排序</div>
              <div class="mt-3 flex flex-wrap gap-2">
                <button
                  v-for="item in sortModes"
                  :key="item.value"
                  type="button"
                  class="terminal-filter-chip"
                  :class="{ 'terminal-filter-chip-active': sortMode === item.value }"
                  @click="sortMode = item.value"
                >
                  {{ item.label }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="terminal-check-panel mt-5">
          <div class="min-w-0 flex-1">
            <div class="text-xs uppercase tracking-[0.16em] text-steel">Account Status Check</div>
            <h3 class="mt-2 text-lg font-black">一键检测账号状态</h3>
            <p class="mt-2 text-sm text-steel">真实读取 Session / TData，更新手机号、昵称、个性签名、个人频道、在线状态、心跳和风控状态。</p>
          </div>

          <div class="terminal-check-actions">
            <GlassButton size="sm" variant="primary" :loading="checking && checkScope === 'all'" @click="check('all')">检测全部账号</GlassButton>
            <GlassButton size="sm" variant="secondary" :disabled="!groupID" :loading="checking && checkScope === 'group'" @click="check('group')">检测当前分组</GlassButton>
            <GlassButton size="sm" variant="secondary" :disabled="!selectedTerminal" :loading="checking && checkScope === 'terminal'" @click="check('terminal')">检测选中账号</GlassButton>
          </div>

          <div class="terminal-check-summary">
            <div class="terminal-check-stat">
              <span>检测范围</span>
              <strong>{{ checkScopeText(checkScope || lastCheckScope) }}</strong>
            </div>
            <div class="terminal-check-stat">
              <span>总数</span>
              <strong>{{ checkSummary?.total ?? '-' }}</strong>
            </div>
            <div class="terminal-check-stat">
              <span>在线</span>
              <strong class="text-neon">{{ checkSummary?.online ?? '-' }}</strong>
            </div>
            <div class="terminal-check-stat">
              <span>离线</span>
              <strong class="text-ice">{{ checkSummary?.offline ?? '-' }}</strong>
            </div>
            <div class="terminal-check-stat">
              <span>异常</span>
              <strong class="text-danger">{{ checkSummary?.abnormal ?? '-' }}</strong>
            </div>
            <div class="terminal-check-stat">
              <span>风控档位</span>
              <strong>{{ riskPolicyPresetText }}</strong>
            </div>
          </div>
        </div>

        <div v-if="selectedTerminal" class="terminal-selected-card mt-5">
          <div class="terminal-selected-main">
            <img class="terminal-avatar-lg" :src="selectedTerminal.avatar_url || fallbackAvatar" alt="头像" @error="useFallbackAvatar" />
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-3">
                <div class="truncate text-xl font-black">{{ selectedTerminal.nickname || displayPhone(selectedTerminal) || '未命名终端' }}</div>
                <StatusBadge
                  :status="accountStatus(selectedTerminal)"
                  :label="accountStatusText(selectedTerminal)"
                />
              </div>
              <div class="mt-2 text-sm text-steel">{{ displayPhone(selectedTerminal) || '未识别手机号' }}</div>
              <div class="mt-2 line-clamp-2 text-sm text-steel">{{ selectedTerminal.bio || '未设置个性签名' }}</div>
            </div>
          </div>

          <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <div class="terminal-detail-chip">
              <div class="terminal-detail-label">归属地</div>
              <div class="terminal-detail-value">{{ locationText(selectedTerminal.origin_country, selectedTerminal.origin_flag, '未知') }}</div>
            </div>
            <div class="terminal-detail-chip">
              <div class="terminal-detail-label">个人频道</div>
              <div class="terminal-detail-value">{{ selectedTerminal.channel_name || channelName(selectedTerminal.homepage) || '未设置' }}</div>
            </div>
            <div class="terminal-detail-chip">
              <div class="terminal-detail-label">心跳</div>
              <div class="terminal-detail-value">{{ onlineHeartbeat(selectedTerminal).primary }}</div>
            </div>
            <div class="terminal-detail-chip">
              <div class="terminal-detail-label">绑定 IP</div>
              <div class="terminal-detail-value">{{ networkPrimary(selectedTerminal) }}</div>
            </div>
          </div>
        </div>

        <div v-if="view === 'table'" class="mt-5 flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border border-white/10 bg-black/10">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-white/8 px-4 py-3">
            <div class="text-sm text-steel">
              当前分组：{{ currentGroupName }}，本页 {{ pagedTerminals.length }} / 筛选 {{ filteredTerminals.length }} / 总数 {{ terminals.length }} 个终端
            </div>
            <div class="flex flex-wrap items-center justify-end gap-2">
              <div class="text-sm text-steel">本页已勾选 {{ selectedVisibleCount }} 个账号</div>
              <GlassButton size="sm" variant="ghost" @click="selectHighRiskVisible">勾选本页高风险</GlassButton>
              <GlassButton size="sm" variant="ghost" @click="selectLowRiskVisible">勾选本页低风险</GlassButton>
              <GlassButton size="sm" variant="ghost" :disabled="!selectedTerminalIDs.length" @click="clearTerminalSelection">清空勾选</GlassButton>
              <GlassButton
                size="sm"
                variant="ghost"
                :disabled="!selectedTerminalIDs.length"
                :loading="batchOperating === 'reduce_limits'"
                @click="runBatchOperation('reduce_limits')"
              >
                批量降频 50%
              </GlassButton>
              <GlassButton
                size="sm"
                variant="ghost"
                :disabled="!selectedTerminalIDs.length"
                :loading="batchOperating === 'clear_cooldown'"
                @click="runBatchOperation('clear_cooldown')"
              >
                批量解除冷却
              </GlassButton>
              <GlassButton
                size="sm"
                variant="ghost"
                :disabled="!selectedTerminalIDs.length"
                :loading="batchOperating === 'clear_expired_restrictions'"
                @click="runBatchOperation('clear_expired_restrictions')"
              >
                批量清理过期
              </GlassButton>
            </div>
          </div>

          <div class="min-h-0 flex-1 overflow-auto scrollbar-thin">
            <table class="terminal-table w-full min-w-[1780px] text-left text-sm">
              <thead>
                <tr>
                  <th class="terminal-th terminal-th-center">
                    <input
                      class="terminal-checkbox"
                      type="checkbox"
                      :checked="allVisibleSelected"
                      :indeterminate="isSelectionIndeterminate"
                      aria-label="全选当前账号"
                      @click.stop
                      @change="toggleAllVisible"
                    />
                  </th>
                  <th class="terminal-th terminal-th-center">序号</th>
                  <th class="terminal-th">手机号</th>
                  <th class="terminal-th terminal-th-center">归属地</th>
                  <th class="terminal-th terminal-th-center">头像</th>
                  <th class="terminal-th">昵称 / 个性签名</th>
                  <th class="terminal-th">个人频道</th>
                  <th class="terminal-th terminal-th-center">心跳</th>
                  <th class="terminal-th terminal-th-center">状态</th>
                  <th class="terminal-th terminal-th-center">接入</th>
                  <th class="terminal-th terminal-th-center">绑定 IP</th>
                  <th class="terminal-th terminal-th-center">成功 / 失败 / 总数</th>
                  <th class="terminal-th terminal-th-center">风控状态</th>
                  <th class="terminal-th terminal-th-center">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(item, index) in pagedTerminals"
                  :key="item.id"
                  class="terminal-row"
                  :class="{ 'terminal-row-active': selectedTerminalID === item.id }"
                  @click="selectTerminal(item.id)"
                >
                  <td class="terminal-td terminal-td-center">
                    <input
                      class="terminal-checkbox"
                      type="checkbox"
                      :checked="isTerminalSelected(item.id)"
                      :aria-label="`选择账号 ${displayPhone(item) || item.nickname || terminalRowNumber(index)}`"
                      @click.stop
                      @change="toggleTerminalSelection(item.id, $event)"
                    />
                  </td>
                  <td class="terminal-td terminal-td-center font-semibold text-white">{{ terminalRowNumber(index) }}</td>
                  <td class="terminal-td">
                    <div class="terminal-stack">
                      <div class="font-semibold text-white">{{ displayPhone(item) || '-' }}</div>
                      <div class="terminal-muted">{{ item.phone || '未识别号码' }}</div>
                    </div>
                  </td>
                  <td class="terminal-td terminal-td-center">
                    <div class="terminal-stack terminal-stack-center">
                      <div class="font-semibold text-white">{{ locationText(item.origin_country, item.origin_flag, '未知') }}</div>
                      <div class="terminal-muted">{{ phonePrefix(item) }}</div>
                    </div>
                  </td>
                  <td class="terminal-td terminal-td-center">
                    <img class="terminal-avatar" :src="item.avatar_url || fallbackAvatar" alt="头像" @error="useFallbackAvatar" />
                  </td>
                  <td class="terminal-td">
                    <div class="terminal-stack">
                      <div class="font-semibold text-white">{{ item.nickname || '未设置昵称' }}</div>
                      <div class="terminal-muted line-clamp-2">{{ item.bio || '未设置个性签名' }}</div>
                    </div>
                  </td>
                  <td class="terminal-td">
                    <div v-if="item.homepage" class="terminal-stack">
                      <div class="font-semibold text-white">{{ item.channel_name || channelName(item.homepage) }}</div>
                      <a class="truncate text-neon hover:underline" :href="normalizeChannelURL(item.homepage)" target="_blank" rel="noreferrer">
                        {{ normalizeChannelURL(item.homepage) }}
                      </a>
                    </div>
                    <div v-else class="terminal-stack">
                      <div class="font-semibold text-white">未设置</div>
                      <div class="terminal-muted">暂无个人频道</div>
                    </div>
                  </td>
                  <td class="terminal-td terminal-td-center">
                    <div class="terminal-stack terminal-stack-center">
                      <div class="font-semibold text-white">{{ onlineHeartbeat(item).primary }}</div>
                      <div class="terminal-muted">{{ onlineHeartbeat(item).secondary }}</div>
                    </div>
                  </td>
                  <td class="terminal-td terminal-td-center">
                    <div class="terminal-stack terminal-stack-center">
                      <StatusBadge :status="accountStatus(item)" :label="accountStatusText(item)" />
                      <div class="terminal-muted">{{ accountStatusHelp(item) }}</div>
                    </div>
                  </td>
                  <td class="terminal-td terminal-td-center">
                    <div class="terminal-stack terminal-stack-center">
                      <div class="font-semibold text-white">{{ accessTypeText(item.access_type) }}</div>
                      <div class="terminal-muted">{{ item.group_id ? currentGroupNameByID(item.group_id) : '未分组' }}</div>
                    </div>
                  </td>
                  <td class="terminal-td terminal-td-center">
                    <div class="terminal-stack terminal-stack-center">
                      <div class="font-semibold text-white">{{ networkPrimary(item) }}</div>
                      <div class="terminal-muted">{{ networkSecondary(item) }}</div>
                    </div>
                  </td>
                  <td class="terminal-td terminal-td-center">
                    <div class="terminal-stack terminal-stack-center">
                      <div class="font-semibold text-white">{{ item.total_success }} / {{ item.total_failed }} / {{ totalRuns(item) }}</div>
                      <div class="terminal-muted">今日 {{ item.today_success }} / {{ item.today_failed }}</div>
                    </div>
                  </td>
                  <td class="terminal-td terminal-td-center">
                    <div class="terminal-stack terminal-stack-center">
                      <div class="flex flex-wrap items-center justify-center gap-2">
                        <span class="status-pill" :data-tone="riskTone(item)">
                          {{ item.risk_status || '正常' }}
                        </span>
                        <span class="status-pill" :data-tone="riskScoreTone(riskScoreBadgeText(item))">{{ riskScoreBadgeText(item) }}</span>
                      </div>
                      <div class="terminal-muted">{{ riskSecondaryText(item) }}</div>
                      <div class="terminal-muted">{{ riskMetaText(item) }}</div>
                    </div>
                  </td>
                  <td class="terminal-td terminal-td-center">
                    <div class="terminal-row-actions">
                      <button
                        class="terminal-row-action terminal-row-action-primary"
                        type="button"
                        :disabled="checking || deletingTerminalID === item.id"
                        @click.stop="check('terminal', item.id)"
                      >
                        {{ checking && checkScope === 'terminal' && checkingTerminalID === item.id ? '检测中' : '检测' }}
                      </button>
                      <button
                        class="terminal-row-action terminal-row-action-danger"
                        type="button"
                        :disabled="checking || deletingTerminalID === item.id"
                        @click.stop="deleteTerminal(item)"
                      >
                        {{ deletingTerminalID === item.id ? '删除中' : '删除' }}
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>

            <div v-if="!filteredTerminals.length" class="grid min-h-[18rem] place-items-center text-sm text-steel">
              当前筛选下暂无终端
            </div>
          </div>
          <div v-if="terminalPageCount > 1" class="flex flex-wrap items-center justify-end gap-2 border-t border-white/8 px-4 py-3 text-sm text-steel">
            <span>第 {{ terminalPage }} / {{ terminalPageCount }} 页</span>
            <GlassButton size="sm" variant="secondary" :disabled="terminalPage <= 1" @click="terminalPage--">上一页</GlassButton>
            <GlassButton size="sm" variant="secondary" :disabled="terminalPage >= terminalPageCount" @click="terminalPage++">下一页</GlassButton>
          </div>
        </div>

        <div v-else class="mt-5 flex min-h-0 flex-1 flex-col gap-4">
          <div class="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">
            <GlassCard
              v-for="(item, index) in pagedTerminals"
              :key="item.id"
              class="cursor-pointer transition-transform hover:-translate-y-0.5"
              :class="{ 'ring-1 ring-ice/35': selectedTerminalID === item.id }"
              @click="selectTerminal(item.id)"
            >
              <div class="flex items-start gap-3">
                <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-white/8 text-sm font-bold text-white">{{ terminalRowNumber(index) }}</div>
                <img class="terminal-avatar-lg" :src="item.avatar_url || fallbackAvatar" alt="头像" @error="useFallbackAvatar" />
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <div class="truncate text-lg font-black">{{ item.nickname || displayPhone(item) || `终端 ${terminalRowNumber(index)}` }}</div>
                    <StatusBadge :status="accountStatus(item)" :label="accountStatusText(item)" />
                  </div>
                  <div class="mt-2 text-sm text-steel">{{ displayPhone(item) || '未识别手机号' }}</div>
                  <div class="mt-2 line-clamp-2 text-sm text-steel">{{ item.bio || '未设置个性签名' }}</div>
                </div>
              </div>

              <div class="mt-5 grid gap-3 text-sm">
                <div class="terminal-detail-chip">
                  <div class="terminal-detail-label">个人频道</div>
                  <div v-if="item.homepage" class="mt-2 space-y-1">
                    <div class="font-semibold text-white">{{ item.channel_name || channelName(item.homepage) }}</div>
                    <a class="text-neon hover:underline" :href="normalizeChannelURL(item.homepage)" target="_blank" rel="noreferrer">
                      {{ normalizeChannelURL(item.homepage) }}
                    </a>
                  </div>
                  <div v-else class="mt-2 text-steel">未设置</div>
                </div>

                <div class="grid gap-3 sm:grid-cols-2">
                  <div class="terminal-detail-chip">
                    <div class="terminal-detail-label">归属地</div>
                    <div class="terminal-detail-value">{{ locationText(item.origin_country, item.origin_flag, '未知') }}</div>
                  </div>
                  <div class="terminal-detail-chip">
                    <div class="terminal-detail-label">心跳</div>
                    <div class="terminal-detail-value">{{ onlineHeartbeat(item).primary }}</div>
                  </div>
                  <div class="terminal-detail-chip">
                    <div class="terminal-detail-label">绑定 IP</div>
                    <div class="terminal-detail-value">{{ networkPrimary(item) }}</div>
                  </div>
                  <div class="terminal-detail-chip">
                    <div class="terminal-detail-label">风控状态</div>
                    <div class="terminal-detail-value">{{ accountStatusText(item) }}</div>
                    <div class="mt-2 flex flex-wrap gap-2">
                      <span class="status-pill" :data-tone="riskScoreTone(riskScoreBadgeText(item))">{{ riskScoreBadgeText(item) }}</span>
                      <span class="text-xs text-steel">{{ riskMetaText(item) }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </GlassCard>

            <div v-if="!filteredTerminals.length" class="col-span-full rounded-2xl border border-dashed border-white/10 py-12 text-center text-sm text-steel">
              当前筛选下暂无终端
            </div>
          </div>
          <div v-if="terminalPageCount > 1" class="flex flex-wrap items-center justify-end gap-2 text-sm text-steel">
            <span>第 {{ terminalPage }} / {{ terminalPageCount }} 页</span>
            <GlassButton size="sm" variant="secondary" :disabled="terminalPage <= 1" @click="terminalPage--">上一页</GlassButton>
            <GlassButton size="sm" variant="secondary" :disabled="terminalPage >= terminalPageCount" @click="terminalPage++">下一页</GlassButton>
          </div>
        </div>
      </GlassCard>

      <div class="flex flex-col gap-4 xl:sticky xl:top-24 xl:self-start">
        <GlassCard tone="cyan">
          <div class="text-xs uppercase tracking-[0.16em] text-steel">Live Stats</div>
          <h2 class="mt-2 text-xl font-black">终端概览</h2>
          <div class="mt-3 flex flex-wrap items-center gap-2 text-sm text-steel">
            <span class="status-pill" :data-tone="riskPolicyTone">{{ riskPolicyPresetText }}</span>
            <span>{{ riskPolicyHelpText }}</span>
          </div>
          <div class="mt-5 grid gap-3">
            <div v-for="card in overviewCards" :key="card.label" class="metric-card app-card p-4" :data-tone="card.tone">
              <div class="text-sm text-steel">{{ card.label }}</div>
              <div class="mt-2 text-2xl font-black">{{ card.value }}</div>
              <div class="mt-2 text-sm text-steel">{{ card.help }}</div>
            </div>
          </div>
        </GlassCard>

        <GlassCard>
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-steel">Risk Queue</div>
              <h2 class="mt-2 text-xl font-black">优先处理</h2>
            </div>
            <div class="text-sm text-steel">{{ riskBoardLoading ? '更新中' : `${prioritizedRiskTerminals.length} 个` }}</div>
          </div>

          <div v-if="prioritizedRiskTerminals.length" class="mt-5 space-y-3">
            <button
              v-for="item in prioritizedRiskTerminals"
              :key="item.id"
              type="button"
              class="w-full rounded-lg border border-white/10 bg-white/5 p-3 text-left transition hover:bg-white/8"
              @click="selectTerminal(item.id)"
            >
              <div class="flex items-center justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-semibold text-white">{{ item.nickname || displayPhone(item) || '未命名终端' }}</div>
                  <div class="mt-1 text-xs text-steel">{{ displayPhone(item) || '未识别手机号' }}</div>
                </div>
                <span class="status-pill" :data-tone="riskScoreTone(riskScoreBadgeText(item))">{{ riskScoreBadgeText(item) }}</span>
              </div>
              <div class="mt-2 text-xs text-steel">{{ riskMetaText(item) }}</div>
            </button>
          </div>
          <div v-else class="mt-5 text-sm text-steel">
            {{ riskBoardLoading ? '正在整理风控优先队列…' : '当前筛选下没有需要优先处理的风险账号。' }}
          </div>
        </GlassCard>

        <GlassCard>
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-steel">Backup Queue</div>
              <h2 class="mt-2 text-xl font-black">可接替账号</h2>
            </div>
            <div class="text-sm text-steel">{{ riskBoardLoading ? '更新中' : `${backupTerminals.length} 个` }}</div>
          </div>

          <div v-if="backupTerminals.length" class="mt-5 space-y-3">
            <button
              v-for="item in backupTerminals"
              :key="item.id"
              type="button"
              class="w-full rounded-lg border border-white/10 bg-white/5 p-3 text-left transition hover:bg-white/8"
              @click="selectTerminal(item.id)"
            >
              <div class="flex items-center justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-semibold text-white">{{ item.nickname || displayPhone(item) || '未命名终端' }}</div>
                  <div class="mt-1 text-xs text-steel">{{ displayPhone(item) || '未识别手机号' }}</div>
                </div>
                <span class="status-pill" :data-tone="riskScoreTone(riskScoreBadgeText(item))">{{ riskScoreBadgeText(item) }}</span>
              </div>
              <div class="mt-2 text-xs text-steel">{{ backupMetaText(item) }}</div>
            </button>
          </div>
          <div v-else class="mt-5 text-sm text-steel">
            {{ riskBoardLoading ? '正在整理替补队列…' : '当前筛选下没有明显更适合作为替补的账号。' }}
          </div>
        </GlassCard>

        <GlassCard>
          <div class="text-xs uppercase tracking-[0.16em] text-steel">Selected Terminal</div>
          <h2 class="mt-2 text-xl font-black">当前终端</h2>

          <div v-if="selectedTerminal" class="mt-5 space-y-4">
            <div class="flex gap-3">
              <img class="terminal-avatar-lg" :src="selectedTerminal.avatar_url || fallbackAvatar" alt="头像" @error="useFallbackAvatar" />
              <div class="min-w-0 flex-1">
                <div class="truncate text-lg font-black">{{ selectedTerminal.nickname || displayPhone(selectedTerminal) || '未命名终端' }}</div>
                <div class="mt-1 text-sm text-steel">{{ displayPhone(selectedTerminal) || '未识别手机号' }}</div>
                <div class="mt-2 line-clamp-2 text-sm text-steel">{{ selectedTerminal.bio || '未设置个性签名' }}</div>
              </div>
            </div>

            <div class="grid gap-3">
              <div class="terminal-detail-chip">
                <div class="terminal-detail-label">个人频道</div>
                <div v-if="selectedTerminal.homepage" class="mt-2 space-y-1">
                  <div class="font-semibold text-white">{{ selectedTerminal.channel_name || channelName(selectedTerminal.homepage) }}</div>
                  <a class="text-neon hover:underline" :href="normalizeChannelURL(selectedTerminal.homepage)" target="_blank" rel="noreferrer">
                    {{ normalizeChannelURL(selectedTerminal.homepage) }}
                  </a>
                </div>
                <div v-else class="mt-2 text-steel">未设置</div>
              </div>

              <div class="grid gap-3 sm:grid-cols-2">
                <div class="terminal-detail-chip">
                  <div class="terminal-detail-label">归属地</div>
                  <div class="terminal-detail-value">{{ locationText(selectedTerminal.origin_country, selectedTerminal.origin_flag, '未知') }}</div>
                </div>
                <div class="terminal-detail-chip">
                  <div class="terminal-detail-label">心跳</div>
                  <div class="terminal-detail-value">{{ onlineHeartbeat(selectedTerminal).primary }}</div>
                </div>
                <div class="terminal-detail-chip">
                  <div class="terminal-detail-label">接入</div>
                  <div class="terminal-detail-value">{{ accessTypeText(selectedTerminal.access_type) }}</div>
                </div>
                <div class="terminal-detail-chip">
                  <div class="terminal-detail-label">绑定 IP</div>
                  <div class="terminal-detail-value">{{ networkPrimary(selectedTerminal) }}</div>
                </div>
                <div class="terminal-detail-chip">
                  <div class="terminal-detail-label">成功 / 失败 / 总数</div>
                  <div class="terminal-detail-value">{{ selectedTerminal.total_success }} / {{ selectedTerminal.total_failed }} / {{ totalRuns(selectedTerminal) }}</div>
                </div>
                <div class="terminal-detail-chip">
                  <div class="terminal-detail-label">风控状态</div>
                  <div class="terminal-detail-value">{{ accountStatusText(selectedTerminal) }}</div>
                </div>
                <div class="terminal-detail-chip">
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div class="terminal-detail-label">冷却状态</div>
                      <div class="terminal-detail-value">{{ cooldownText(selectedTerminal) }}</div>
                    </div>
                    <GlassButton
                      v-if="selectedTerminal.sleep_until"
                      size="sm"
                      variant="ghost"
                      :loading="clearingCooldown"
                      @click="clearCooldown"
                    >
                      解除冷却
                    </GlassButton>
                  </div>
                </div>
              </div>

              <div class="terminal-detail-chip">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div class="terminal-detail-label">账号维度限额</div>
                    <div class="mt-1 text-sm text-steel">0 表示不限额。保存后会立即作用到私信和加群执行链路。</div>
                  </div>
                  <GlassButton size="sm" variant="primary" :loading="savingTerminalLimits" @click="saveTerminalLimits">保存限额</GlassButton>
                </div>

                <div class="mt-4 grid gap-3 sm:grid-cols-2">
                  <label class="flex flex-col gap-2 rounded-lg border border-white/10 bg-white/5 p-3 text-sm text-steel">
                    <span>每小时私信数</span>
                    <input v-model.number="terminalLimitForm.dm_hourly_limit" type="number" min="0" class="min-h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm text-white" />
                    <small>已用 {{ selectedTerminal.dm_hourly_count }}，重置 {{ formatResetAt(selectedTerminal.dm_hourly_reset_at) }}</small>
                  </label>
                  <label class="flex flex-col gap-2 rounded-lg border border-white/10 bg-white/5 p-3 text-sm text-steel">
                    <span>每天私信数</span>
                    <input v-model.number="terminalLimitForm.dm_daily_limit" type="number" min="0" class="min-h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm text-white" />
                    <small>已用 {{ selectedTerminal.dm_daily_count }}，重置 {{ formatResetAt(selectedTerminal.dm_daily_reset_at) }}</small>
                  </label>
                  <label class="flex flex-col gap-2 rounded-lg border border-white/10 bg-white/5 p-3 text-sm text-steel">
                    <span>每小时加群数</span>
                    <input v-model.number="terminalLimitForm.join_hourly_limit" type="number" min="0" class="min-h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm text-white" />
                    <small>已用 {{ selectedTerminal.join_hourly_count }}，重置 {{ formatResetAt(selectedTerminal.join_hourly_reset_at) }}</small>
                  </label>
                  <label class="flex flex-col gap-2 rounded-lg border border-white/10 bg-white/5 p-3 text-sm text-steel">
                    <span>每天加群数</span>
                    <input v-model.number="terminalLimitForm.join_daily_limit" type="number" min="0" class="min-h-11 rounded-lg border border-white/10 bg-black/20 px-3 text-sm text-white" />
                    <small>已用 {{ selectedTerminal.join_daily_count }}，重置 {{ formatResetAt(selectedTerminal.join_daily_reset_at) }}</small>
                  </label>
                </div>
              </div>

              <div class="terminal-detail-chip">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div class="terminal-detail-label">近 24 小时风控概览</div>
                    <div class="mt-1 text-sm text-steel">按冷却、限制命中和当前额度使用情况，快速判断这个账号是不是开始变危险。</div>
                  </div>
                  <div class="text-sm text-steel">{{ riskStatsLoading ? '统计中' : riskScoreText(terminalRiskStats) }}</div>
                </div>

                <div v-if="terminalRiskStats" class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                  <div class="rounded-lg border border-white/10 bg-white/5 p-3">
                    <div class="text-xs uppercase tracking-[0.14em] text-steel">风险等级</div>
                    <div class="mt-2 flex items-center gap-2">
                      <span class="status-pill" :data-tone="riskScoreTone(terminalRiskStats.risk_score)">{{ riskScoreText(terminalRiskStats) }}</span>
                      <span class="text-xs text-steel">{{ terminalRiskStats.cooldown_active ? `冷却到 ${formatDate(terminalRiskStats.cooldown_until || null)}` : '当前无全局冷却' }}</span>
                    </div>
                  </div>
                  <div class="rounded-lg border border-white/10 bg-white/5 p-3">
                    <div class="text-xs uppercase tracking-[0.14em] text-steel">24h 命中</div>
                    <div class="mt-2 text-lg font-semibold text-white">{{ terminalRiskStats.failure_24h_total }}</div>
                    <div class="mt-1 text-xs text-steel">私信 {{ terminalRiskStats.restriction_24h_dm }} / 加群 {{ terminalRiskStats.restriction_24h_join }}</div>
                  </div>
                  <div class="rounded-lg border border-white/10 bg-white/5 p-3">
                    <div class="text-xs uppercase tracking-[0.14em] text-steel">限制记录</div>
                    <div class="mt-2 text-lg font-semibold text-white">{{ terminalRiskStats.active_restriction_count }}</div>
                    <div class="mt-1 text-xs text-steel">生效中，已过期 {{ terminalRiskStats.expired_restriction_count }} 条</div>
                  </div>
                  <div class="rounded-lg border border-white/10 bg-white/5 p-3">
                    <div class="text-xs uppercase tracking-[0.14em] text-steel">私信额度</div>
                    <div class="mt-2 text-sm text-white">小时 {{ quotaUsageText(selectedTerminal.dm_hourly_count, terminalRiskStats.dm_hourly_limit, terminalRiskStats.dm_hourly_usage) }}</div>
                    <div class="mt-1 text-sm text-white">当天 {{ quotaUsageText(selectedTerminal.dm_daily_count, terminalRiskStats.dm_daily_limit, terminalRiskStats.dm_daily_usage) }}</div>
                  </div>
                  <div class="rounded-lg border border-white/10 bg-white/5 p-3">
                    <div class="text-xs uppercase tracking-[0.14em] text-steel">加群额度</div>
                    <div class="mt-2 text-sm text-white">小时 {{ quotaUsageText(selectedTerminal.join_hourly_count, terminalRiskStats.join_hourly_limit, terminalRiskStats.join_hourly_usage) }}</div>
                    <div class="mt-1 text-sm text-white">当天 {{ quotaUsageText(selectedTerminal.join_daily_count, terminalRiskStats.join_daily_limit, terminalRiskStats.join_daily_usage) }}</div>
                  </div>
                  <div class="rounded-lg border border-white/10 bg-white/5 p-3">
                    <div class="text-xs uppercase tracking-[0.14em] text-steel">运营建议</div>
                    <div class="mt-2 text-sm text-steel">{{ riskAdviceText(terminalRiskStats) }}</div>
                  </div>
                </div>
                <div v-else class="mt-4 text-sm text-steel">
                  {{ riskStatsLoading ? '正在计算近 24 小时风控统计…' : '当前还没有可展示的风控统计。' }}
                </div>
              </div>

              <div class="terminal-detail-chip">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div class="terminal-detail-label">限制记录</div>
                    <div class="mt-1 text-sm text-steel">这里会显示这个账号最近命中的目标级限制，系统后续会自动跳过这些组合。</div>
                  </div>
                  <div class="text-sm text-steel">{{ restrictionLoading ? '读取中' : `${terminalRestrictions.length} 条` }}</div>
                </div>

                <div class="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto]">
                  <div class="grid gap-3 md:grid-cols-2">
                    <div class="rounded-lg border border-white/10 bg-white/5 p-3">
                      <div class="text-xs uppercase tracking-[0.14em] text-steel">状态筛选</div>
                      <div class="mt-3 flex flex-wrap gap-2">
                        <button
                          v-for="item in restrictionStateFilters"
                          :key="item.value"
                          type="button"
                          class="terminal-filter-chip"
                          :class="{ 'terminal-filter-chip-active': restrictionStateFilter === item.value }"
                          @click="restrictionStateFilter = item.value"
                        >
                          {{ item.label }}
                        </button>
                      </div>
                    </div>
                    <div class="rounded-lg border border-white/10 bg-white/5 p-3">
                      <div class="text-xs uppercase tracking-[0.14em] text-steel">动作筛选</div>
                      <div class="mt-3 flex flex-wrap gap-2">
                        <button
                          v-for="item in restrictionActionFilters"
                          :key="item.value"
                          type="button"
                          class="terminal-filter-chip"
                          :class="{ 'terminal-filter-chip-active': restrictionActionFilter === item.value }"
                          @click="restrictionActionFilter = item.value"
                        >
                          {{ item.label }}
                        </button>
                      </div>
                    </div>
                  </div>

                  <div class="flex flex-wrap items-start justify-end gap-2">
                    <GlassButton
                      size="sm"
                      variant="ghost"
                      :loading="clearingRestrictionScope === 'expired'"
                      @click="clearRestrictionBatch('expired')"
                    >
                      清理已过期
                    </GlassButton>
                    <GlassButton
                      size="sm"
                      variant="ghost"
                      :loading="clearingRestrictionScope === 'filtered'"
                      :disabled="!terminalRestrictions.length"
                      @click="clearRestrictionBatch('filtered')"
                    >
                      清空当前筛选
                    </GlassButton>
                  </div>
                </div>

                <div v-if="terminalRestrictions.length" class="mt-4 space-y-3">
                  <div
                    v-for="item in terminalRestrictions"
                    :key="item.id"
                    class="rounded-lg border border-white/10 bg-white/5 p-3"
                  >
                    <div class="flex flex-wrap items-center justify-between gap-2">
                      <div class="font-semibold text-white">{{ item.action_text }} · {{ restrictionTargetLabel(item) }}</div>
                      <span class="status-pill" :data-tone="item.active ? 'danger' : 'info'">{{ item.active ? '生效中' : '已过期' }}</span>
                    </div>
                    <div class="mt-2 text-sm text-steel">{{ item.reason || '未记录原因' }}</div>
                    <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-steel">
                      <span>失败次数 {{ item.fail_count }}</span>
                      <span>冷却至 {{ item.cooldown_until ? formatDate(item.cooldown_until) : '未设置' }}</span>
                      <span>最近失败 {{ item.last_failed_at ? formatDate(item.last_failed_at) : '未记录' }}</span>
                    </div>
                    <div class="mt-3 flex justify-end">
                      <GlassButton
                        size="sm"
                        variant="ghost"
                        :loading="clearingRestrictionID === item.id"
                        @click="clearRestriction(item)"
                      >
                        解除限制
                      </GlassButton>
                    </div>
                  </div>
                </div>
                <div v-else class="mt-4 text-sm text-steel">
                  {{ restrictionLoading ? '正在读取限制记录…' : '当前账号还没有记录到目标级限制。' }}
                </div>
              </div>
            </div>
          </div>

          <div v-else class="grid min-h-[18rem] place-items-center rounded-2xl border border-dashed border-white/10 text-sm text-steel">
            当前没有可选终端
          </div>
        </GlassCard>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, watch } from 'vue'
import { api, type Group, type SystemSettings, type Terminal, type TerminalCheckSummary, type TerminalRestriction, type TerminalRiskStats, type TerminalRiskBoardItem } from '../api/client'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import GroupSelect from '../components/GroupSelect.vue'
import { useUiStore } from '../stores/ui'

const ui = useUiStore()
const fallbackAvatar = 'https://images.unsplash.com/photo-1500648767791-00dcc994a43e?auto=format&fit=crop&w=160&q=80'
const view = ref<'table' | 'card'>('table')
const groupID = ref('')
const keyword = ref('')
const statusFilter = ref<'all' | 'online' | 'offline' | 'abnormal'>('all')
const accessFilter = ref<'all' | 'session' | 'data'>('all')
const terminalPage = ref(1)
const terminalPageSize = 10
const selectedTerminalID = ref('')
const selectedTerminalIDs = ref<string[]>([])
const groups = ref<Group[]>([])
const terminals = ref<Terminal[]>([])
const checking = ref(false)
type CheckScope = 'all' | 'group' | 'terminal'
const checkScope = ref<CheckScope | ''>('')
const lastCheckScope = ref<CheckScope | ''>('')
const checkingTerminalID = ref('')
const deletingTerminalID = ref('')
const loading = ref(false)
const groupLoading = ref(false)
const checkSummary = ref<TerminalCheckSummary | null>(null)
const savingTerminalLimits = ref(false)
const restrictionLoading = ref(false)
const riskStatsLoading = ref(false)
const riskBoardLoading = ref(false)
const batchOperating = ref<'reduce_limits' | 'clear_cooldown' | 'clear_expired_restrictions' | ''>('')
const clearingCooldown = ref(false)
const clearingRestrictionID = ref('')
const clearingRestrictionScope = ref<'expired' | 'filtered' | ''>('')
const terminalRestrictions = ref<TerminalRestriction[]>([])
const terminalRiskStats = ref<TerminalRiskStats | null>(null)
const terminalRiskBoard = ref<Record<string, TerminalRiskBoardItem>>({})
const systemSettings = ref<SystemSettings | null>(null)
const restrictionStateFilter = ref<'all' | 'active' | 'expired'>('all')
const restrictionActionFilter = ref<'all' | 'dm' | 'join'>('all')
const riskFilter = ref<'all' | 'high' | 'medium' | 'low'>('all')
const sortMode = ref<'default' | 'risk_desc' | 'cooldown_first' | 'quota_desc'>('risk_desc')
const terminalLimitForm = ref({
  dm_hourly_limit: 0,
  dm_daily_limit: 0,
  join_hourly_limit: 0,
  join_daily_limit: 0
})

const statusFilters = [
  { label: '全部在线状态', value: 'all' as const },
  { label: '在线', value: 'online' as const },
  { label: '离线', value: 'offline' as const },
  { label: '异常', value: 'abnormal' as const }
]

const accessFilters = [
  { label: '全部接入', value: 'all' as const },
  { label: 'Session', value: 'session' as const },
  { label: 'TData', value: 'data' as const }
]

const riskFilters = [
  { label: '全部风险', value: 'all' as const },
  { label: '高风险', value: 'high' as const },
  { label: '中风险', value: 'medium' as const },
  { label: '低风险', value: 'low' as const }
]

const sortModes = [
  { label: '风险优先', value: 'risk_desc' as const },
  { label: '冷却优先', value: 'cooldown_first' as const },
  { label: '额度优先', value: 'quota_desc' as const },
  { label: '默认顺序', value: 'default' as const }
]

const restrictionStateFilters = [
  { label: '全部', value: 'all' as const },
  { label: '生效中', value: 'active' as const },
  { label: '已过期', value: 'expired' as const }
]

const restrictionActionFilters = [
  { label: '全部动作', value: 'all' as const },
  { label: '私信', value: 'dm' as const },
  { label: '加群', value: 'join' as const }
]

const StatusBadge = defineComponent({
  props: {
    status: { type: String, required: true },
    label: { type: String, default: '' }
  },
  setup(props) {
    return () => h('span', { class: 'status-pill', 'data-tone': statusTone(props.status) }, props.label || statusText(props.status))
  }
})

const currentGroupName = computed(() => {
  if (!groupID.value) return '全部分组'
  return groups.value.find((group) => group.id === groupID.value)?.name || '未知分组'
})

const groupNameMap = computed(() => new Map(groups.value.map((group) => [group.id, group.name])))

const filteredTerminals = computed(() => {
  const search = keyword.value.trim().toLowerCase()
  const matched = terminals.value.filter((item) => {
    if (groupID.value && item.group_id !== groupID.value) return false
    if (statusFilter.value !== 'all' && (item.status || '').toLowerCase() !== statusFilter.value) return false
    if (accessFilter.value !== 'all' && item.access_type !== accessFilter.value) return false
    if (!matchesRiskFilter(item)) return false

    if (!search) return true

    const haystack = [
      item.nickname,
      item.bio,
      item.phone,
      item.phone_display,
      item.homepage,
      item.channel_name,
      item.origin_country,
      item.exit_ip,
      item.exit_country,
      item.status_text,
      item.account_status_text,
      item.online_status_text,
      item.risk_status
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()

    return haystack.includes(search)
  })

  if (sortMode.value === 'default') {
    return matched
  }

  return [...matched].sort((a, b) => compareTerminalRisk(a, b))
})

const terminalPageCount = computed(() => Math.max(1, Math.ceil(filteredTerminals.value.length / terminalPageSize)))
const pagedTerminals = computed(() => {
  const start = (terminalPage.value - 1) * terminalPageSize
  return filteredTerminals.value.slice(start, start + terminalPageSize)
})
const selectedTerminal = computed(() => filteredTerminals.value.find((item) => item.id === selectedTerminalID.value) || null)
const visibleTerminalIDs = computed(() => pagedTerminals.value.map((item) => item.id))
const selectedVisibleCount = computed(() => visibleTerminalIDs.value.filter((id) => selectedTerminalIDs.value.includes(id)).length)
const allVisibleSelected = computed(() => visibleTerminalIDs.value.length > 0 && selectedVisibleCount.value === visibleTerminalIDs.value.length)
const isSelectionIndeterminate = computed(() => selectedVisibleCount.value > 0 && !allVisibleSelected.value)

const overviewCards = computed(() => {
  const items = filteredTerminals.value
  const online = items.filter((item) => item.status === 'online').length
  const offline = items.filter((item) => item.status === 'offline').length
  const normal = items.filter((item) => accountStatus(item) === 'normal').length
  const abnormal = items.filter((item) => accountStatus(item) === 'abnormal').length
  const channels = items.filter((item) => !!item.homepage).length
  const boundIP = items.filter((item) => !!item.exit_ip).length
  const highRisk = items.filter((item) => terminalRiskLevel(item) === 'high').length

  return [
    { label: '终端总数', value: items.length, help: `当前分组：${currentGroupName.value}`, tone: 'info' },
    { label: '高风险账号', value: highRisk, help: '优先查看冷却中或限制命中频繁的账号', tone: 'danger' },
    { label: '账号正常', value: normal, help: '可读取资料且授权未异常', tone: 'success' },
    { label: '当前在线', value: online, help: 'Telegram 返回在线状态', tone: 'cyan' },
    { label: '当前离线', value: offline, help: '账号正常但不在线', tone: 'warning' },
    { label: '账号异常', value: abnormal, help: '优先排查授权与会话文件', tone: 'danger' },
    { label: '已配置频道', value: channels, help: '上方频道名，下方频道链接', tone: 'cyan' },
    { label: '已绑定出口', value: boundIP, help: '展示为出口 IP 与归属地', tone: 'info' }
  ]
})

const prioritizedRiskTerminals = computed(() =>
  filteredTerminals.value
    .filter((item) => terminalRiskLevel(item) !== 'low')
    .slice(0, 6)
)

const backupTerminals = computed(() =>
  [...filteredTerminals.value]
    .filter((item) => isBackupCandidate(item))
    .sort((a, b) => terminalMaxQuotaUsage(a) - terminalMaxQuotaUsage(b))
    .slice(0, 6)
)

const riskPolicyPresetText = computed(() => {
  const risk = systemSettings.value?.risk_control
  if (!risk?.auto_bypass_high_risk) {
    return '风控避让关闭'
  }
  if (risk.auto_bypass_active_restrictions === 2 && risk.auto_bypass_failures_24h === 6) {
    return '当前策略：保守'
  }
  if (risk.auto_bypass_active_restrictions === 3 && risk.auto_bypass_failures_24h === 10) {
    return '当前策略：平衡'
  }
  if (risk.auto_bypass_active_restrictions === 5 && risk.auto_bypass_failures_24h === 16) {
    return '当前策略：激进'
  }
  return '当前策略：自定义'
})

const riskPolicyTone = computed(() => {
  if (!systemSettings.value?.risk_control.auto_bypass_high_risk) return 'info'
  if (riskPolicyPresetText.value.includes('保守')) return 'warning'
  if (riskPolicyPresetText.value.includes('激进')) return 'success'
  return 'cyan'
})

const riskPolicyHelpText = computed(() => {
  const risk = systemSettings.value?.risk_control
  if (!risk) {
    return '风控策略读取中'
  }
  if (!risk.auto_bypass_high_risk) {
    return '系统当前只记录风险，不会在调度时自动避让高风险账号。'
  }
  return `生效中限制 ${risk.auto_bypass_active_restrictions} 条 / 24h 命中 ${risk.auto_bypass_failures_24h} 次后自动避让。`
})

watch([groupID, keyword, statusFilter, accessFilter, riskFilter, sortMode], () => {
  terminalPage.value = 1
})

watch(terminalPageCount, (count) => {
  if (terminalPage.value > count) terminalPage.value = count
})

watch(
  filteredTerminals,
  (items) => {
    if (items.length === 0) {
      selectedTerminalID.value = ''
      return
    }
    if (!items.some((item) => item.id === selectedTerminalID.value)) {
      selectedTerminalID.value = items[0].id
    }
    const visibleIDs = new Set(items.map((item) => item.id))
    selectedTerminalIDs.value = selectedTerminalIDs.value.filter((id) => visibleIDs.has(id))
  },
  { immediate: true }
)

watch(
  selectedTerminal,
  (item) => {
    terminalLimitForm.value = {
      dm_hourly_limit: item?.dm_hourly_limit ?? 0,
      dm_daily_limit: item?.dm_daily_limit ?? 0,
      join_hourly_limit: item?.join_hourly_limit ?? 0,
      join_daily_limit: item?.join_daily_limit ?? 0
    }
  },
  { immediate: true }
)

watch(
  () => selectedTerminal.value?.id || '',
  async (id) => {
    restrictionStateFilter.value = 'all'
    restrictionActionFilter.value = 'all'
    if (!id) {
      terminalRiskStats.value = null
      terminalRestrictions.value = []
      return
    }
    await Promise.all([loadTerminalRiskStats(id), loadTerminalRestrictions(id)])
  },
  { immediate: true }
)

watch([restrictionStateFilter, restrictionActionFilter], async () => {
  if (!selectedTerminal.value?.id) {
    terminalRestrictions.value = []
    return
  }
  await loadTerminalRestrictions(selectedTerminal.value.id)
})

async function load() {
  loading.value = true
  riskBoardLoading.value = true
  try {
    const [groupData, terminalData, riskBoardData, settingsData] = await Promise.all([
      api.groups('terminal'),
      api.terminals(),
      api.terminalRiskBoard(),
      api.systemSettings()
    ])
    groups.value = groupData
    terminals.value = terminalData
    terminalRiskBoard.value = Object.fromEntries(riskBoardData.map((item) => [item.terminal_id, item]))
    systemSettings.value = settingsData
  } catch (err) {
    ui.toast({
      title: '账号列表读取失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    loading.value = false
    riskBoardLoading.value = false
  }
}

async function createGroup(name: string) {
  groupLoading.value = true
  try {
    await api.createGroup('terminal', name)
    groups.value = await api.groups('terminal')
  } catch (err) {
    ui.toast({
      title: '终端分组创建失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    groupLoading.value = false
  }
}

async function check(scope: CheckScope, terminalID = '') {
  const targetID = scope === 'terminal' ? terminalID || selectedTerminalID.value : ''
  const targetTerminal = targetID ? terminals.value.find((item) => item.id === targetID) || null : null

  if (scope === 'group' && !groupID.value) {
    ui.toast({
      title: '请选择终端分组',
      message: '检测当前分组前，需要先在右上角选择一个分组。',
      tone: 'warning'
    })
    return
  }

  if (scope === 'terminal' && !targetID) {
    ui.toast({
      title: '请选择账号',
      message: '检测选中账号前，需要先点击列表中的一个账号。',
      tone: 'warning'
    })
    return
  }

  if (scope === 'terminal') {
    selectedTerminalID.value = targetID
    checkingTerminalID.value = targetID
  }
  checking.value = true
  checkScope.value = scope
  try {
    const result = await api.checkTerminals({
      groupID: scope === 'group' ? groupID.value : '',
      terminalID: scope === 'terminal' ? targetID : ''
    })
    checkSummary.value = result.summary
    lastCheckScope.value = scope
    await load()
    ui.toast({
      title: '账号状态检测完成',
      message: `${checkScopeText(scope, targetTerminal)}：在线 ${result.summary.online}，离线 ${result.summary.offline}，异常 ${result.summary.abnormal}，资料已重新拉取。`,
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '状态检查失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    checking.value = false
    checkScope.value = ''
    checkingTerminalID.value = ''
  }
}

function selectTerminal(id: string) {
  selectedTerminalID.value = id
}

function isTerminalSelected(id: string) {
  return selectedTerminalIDs.value.includes(id)
}

function toggleTerminalSelection(id: string, event: Event) {
  const input = event.target as HTMLInputElement
  if (input.checked) {
    if (!selectedTerminalIDs.value.includes(id)) {
      selectedTerminalIDs.value = [...selectedTerminalIDs.value, id]
    }
    return
  }
  selectedTerminalIDs.value = selectedTerminalIDs.value.filter((item) => item !== id)
}

function toggleAllVisible(event: Event) {
  const input = event.target as HTMLInputElement
  const visibleIDs = new Set(visibleTerminalIDs.value)
  if (input.checked) {
    const selected = new Set(selectedTerminalIDs.value)
    for (const id of visibleIDs) {
      selected.add(id)
    }
    selectedTerminalIDs.value = Array.from(selected)
    return
  }
  selectedTerminalIDs.value = selectedTerminalIDs.value.filter((id) => !visibleIDs.has(id))
}

function selectHighRiskVisible() {
  const selected = new Set(selectedTerminalIDs.value)
  for (const item of pagedTerminals.value) {
    if (terminalRiskLevel(item) === 'high') {
      selected.add(item.id)
    }
  }
  selectedTerminalIDs.value = Array.from(selected)
}

function selectLowRiskVisible() {
  const selected = new Set(selectedTerminalIDs.value)
  for (const item of pagedTerminals.value) {
    if (isBackupCandidate(item)) {
      selected.add(item.id)
    }
  }
  selectedTerminalIDs.value = Array.from(selected)
}

function clearTerminalSelection() {
  selectedTerminalIDs.value = []
}

function terminalRowNumber(index: number) {
  return (terminalPage.value - 1) * terminalPageSize + index + 1
}

async function deleteTerminal(item: Terminal) {
  const accepted = await ui.confirm({
    title: '删除账号',
    message: `确定删除 ${item.nickname || displayPhone(item) || '这个账号'} 吗？删除后列表中将不再显示该账号。`,
    confirmText: '删除',
    cancelText: '取消',
    tone: 'error'
  })
  if (!accepted) return

  deletingTerminalID.value = item.id
  try {
    await api.deleteTerminal(item.id)
    selectedTerminalIDs.value = selectedTerminalIDs.value.filter((id) => id !== item.id)
    if (selectedTerminalID.value === item.id) {
      selectedTerminalID.value = ''
    }
    await load()
    ui.toast({
      title: '账号已删除',
      message: item.nickname || displayPhone(item) || '账号已从列表删除',
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '账号删除失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    deletingTerminalID.value = ''
  }
}

async function runBatchOperation(action: 'reduce_limits' | 'clear_cooldown' | 'clear_expired_restrictions') {
  if (!selectedTerminalIDs.value.length) {
    return
  }
  const label =
    action === 'reduce_limits'
      ? '批量降频 50%'
      : action === 'clear_cooldown'
        ? '批量解除冷却'
        : '批量清理过期限制'
  const message =
    action === 'reduce_limits'
      ? `系统会把所选 ${selectedTerminalIDs.value.length} 个账号的私信和加群限额整体下调到当前的 50%。确认继续吗？`
      : action === 'clear_cooldown'
        ? `系统会尝试解除所选 ${selectedTerminalIDs.value.length} 个账号的全局冷却。确认继续吗？`
        : `系统会清理所选 ${selectedTerminalIDs.value.length} 个账号的过期目标级限制记录。确认继续吗？`
  const accepted = await ui.confirm({
    title: label,
    message,
    confirmText: '执行',
    tone: 'warning'
  })
  if (!accepted) {
    return
  }

  batchOperating.value = action
  try {
    const result = await api.batchTerminals({
      ids: selectedTerminalIDs.value,
      action,
      multiplier: action === 'reduce_limits' ? 0.5 : undefined
    })
    const okCount = result.results.filter((item) => item.ok).length
    const failCount = result.results.length - okCount
    await Promise.all([
      load(),
      selectedTerminal.value?.id ? loadTerminalRiskStats(selectedTerminal.value.id) : Promise.resolve(),
      selectedTerminal.value?.id ? loadTerminalRestrictions(selectedTerminal.value.id) : Promise.resolve()
    ])
    ui.toast({
      title: `${label}完成`,
      message: `成功 ${okCount} 个，失败 ${failCount} 个。`,
      tone: failCount ? 'warning' : 'success'
    })
  } catch (err) {
    ui.toast({
      title: `${label}失败`,
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    batchOperating.value = ''
  }
}

async function saveTerminalLimits() {
  if (!selectedTerminal.value) {
    return
  }
  savingTerminalLimits.value = true
  try {
    const updated = await api.updateTerminalLimits(selectedTerminal.value.id, {
      dm_hourly_limit: normalizeLimitInput(terminalLimitForm.value.dm_hourly_limit),
      dm_daily_limit: normalizeLimitInput(terminalLimitForm.value.dm_daily_limit),
      join_hourly_limit: normalizeLimitInput(terminalLimitForm.value.join_hourly_limit),
      join_daily_limit: normalizeLimitInput(terminalLimitForm.value.join_daily_limit)
    })
    terminals.value = terminals.value.map((item) => (item.id === updated.id ? updated : item))
    terminalLimitForm.value = {
      dm_hourly_limit: updated.dm_hourly_limit ?? 0,
      dm_daily_limit: updated.dm_daily_limit ?? 0,
      join_hourly_limit: updated.join_hourly_limit ?? 0,
      join_daily_limit: updated.join_daily_limit ?? 0
    }
    await Promise.all([loadRiskBoard(), loadTerminalRiskStats(updated.id)])
    ui.toast({
      title: '账号限额已更新',
      message: `${updated.nickname || displayPhone(updated) || '当前账号'} 的私信/加群限额已生效。`,
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '账号限额保存失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    savingTerminalLimits.value = false
  }
}

async function loadTerminalRiskStats(id: string) {
  riskStatsLoading.value = true
  try {
    terminalRiskStats.value = await api.terminalRiskStats(id)
  } catch {
    terminalRiskStats.value = null
  } finally {
    riskStatsLoading.value = false
  }
}

async function loadRiskBoard() {
  riskBoardLoading.value = true
  try {
    const items = await api.terminalRiskBoard()
    terminalRiskBoard.value = Object.fromEntries(items.map((item) => [item.terminal_id, item]))
  } catch {
    terminalRiskBoard.value = {}
  } finally {
    riskBoardLoading.value = false
  }
}

async function loadTerminalRestrictions(id: string) {
  restrictionLoading.value = true
  try {
    terminalRestrictions.value = await api.terminalRestrictions(id, {
      state: restrictionStateFilter.value,
      action: restrictionActionFilter.value
    })
  } catch {
    terminalRestrictions.value = []
  } finally {
    restrictionLoading.value = false
  }
}

async function clearCooldown() {
  if (!selectedTerminal.value?.id || !selectedTerminal.value.sleep_until) {
    return
  }
  const accepted = await ui.confirm({
    title: '解除账号冷却',
    message: '解除后这个账号会立即重新参与发送和加群选号。确认继续吗？',
    confirmText: '解除',
    tone: 'warning'
  })
  if (!accepted) {
    return
  }

  clearingCooldown.value = true
  try {
    await api.clearTerminalCooldown(selectedTerminal.value.id)
    await load()
    await Promise.all([loadTerminalRiskStats(selectedTerminal.value.id), loadTerminalRestrictions(selectedTerminal.value.id)])
    ui.toast({
      title: '账号冷却已解除',
      message: `${selectedTerminal.value.nickname || displayPhone(selectedTerminal.value) || '当前账号'} 已恢复可调度状态。`,
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '解除冷却失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    clearingCooldown.value = false
  }
}

async function clearRestriction(item: TerminalRestriction) {
  if (!selectedTerminal.value?.id || !item.id) {
    return
  }
  const accepted = await ui.confirm({
    title: '解除目标限制',
    message: `解除后，这个账号会重新尝试 ${item.action_text} 目标 ${restrictionTargetLabel(item)}。确认继续吗？`,
    confirmText: '解除',
    tone: 'warning'
  })
  if (!accepted) {
    return
  }

  clearingRestrictionID.value = item.id
  try {
    await api.deleteTerminalRestriction(selectedTerminal.value.id, item.id)
    await Promise.all([loadRiskBoard(), loadTerminalRiskStats(selectedTerminal.value.id), loadTerminalRestrictions(selectedTerminal.value.id)])
    ui.toast({
      title: '限制已解除',
      message: `${item.action_text} · ${restrictionTargetLabel(item)} 已从限制列表移除。`,
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '解除限制失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    clearingRestrictionID.value = ''
  }
}

async function clearRestrictionBatch(mode: 'expired' | 'filtered') {
  if (!selectedTerminal.value?.id) {
    return
  }
  const accepted = await ui.confirm({
    title: mode === 'expired' ? '批量清理已过期限制' : '批量清空当前筛选',
    message: mode === 'expired'
      ? '系统会删除这个账号下所有已过期的目标级限制记录，仍在生效中的限制不会受影响。确认继续吗？'
      : `系统会删除当前筛选到的 ${terminalRestrictions.value.length} 条限制记录。确认继续吗？`,
    confirmText: '清理',
    tone: 'warning'
  })
  if (!accepted) {
    return
  }

  clearingRestrictionScope.value = mode
  try {
    const result = await api.clearTerminalRestrictions(selectedTerminal.value.id, {
      mode,
      state: restrictionStateFilter.value,
      action: restrictionActionFilter.value
    })
    await Promise.all([loadRiskBoard(), loadTerminalRiskStats(selectedTerminal.value.id), loadTerminalRestrictions(selectedTerminal.value.id)])
    ui.toast({
      title: '批量清理完成',
      message: `已删除 ${result.deleted_count} 条限制记录。`,
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '批量清理失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    clearingRestrictionScope.value = ''
  }
}

function checkScopeText(scope: CheckScope | '', terminal: Terminal | null = null) {
  if (scope === 'all') return '全部账号'
  if (scope === 'group') return currentGroupName.value
  if (scope === 'terminal') {
    return terminal?.nickname || (terminal ? displayPhone(terminal) : '') || '选中账号'
  }
  return '未检测'
}

function statusText(status: string) {
  const normalized = (status || '').toLowerCase()
  if (normalized === 'online') return '在线'
  if (normalized === 'offline') return '离线'
  if (normalized === 'abnormal') return '异常'
  if (normalized === 'checking') return '检测中'
  if (normalized === 'pending') return '待处理'
  if (normalized === 'queued') return '排队中'
  if (normalized === 'running') return '执行中'
  if (normalized === 'paused') return '已暂停'
  if (normalized === 'failed') return '失败'
  if (normalized === 'success') return '成功'
  return status || '未知'
}

function statusTone(status: string) {
  const normalized = (status || '').toLowerCase()
  if (normalized === 'online' || normalized === 'success' || normalized === 'normal' || normalized === 'healthy') return 'success'
  if (normalized === 'abnormal' || normalized === 'failed') return 'danger'
  if (normalized === 'checking' || normalized === 'pending' || normalized === 'queued' || normalized === 'running' || normalized === 'warning') return 'warning'
  return 'info'
}

function accountStatus(item: Terminal) {
  if (isProfileRestricted(item)) return 'abnormal'
  if (item.account_status) return item.account_status
  if ((item.status || '').toLowerCase() === 'abnormal') return 'abnormal'
  const risk = (item.risk_status || '').trim()
  const ban = (item.ban_status || '').trim()
  if (ban && ban !== '正常') return 'abnormal'
  if (risk && risk !== '正常') return 'warning'
  return 'normal'
}

function accountStatusText(item: Terminal) {
  if (isProfileRestricted(item)) return '资料受限'
  return item.account_status_text || (accountStatus(item) === 'normal' ? '正常' : item.risk_status || item.ban_status || '异常')
}

function accountStatusHelp(item: Terminal) {
  if (isProfileRestricted(item)) return 'Telegram 已冻结，资料修改不可用'
  if (item.sleep_until) return `冷却到 ${formatDate(item.sleep_until)}`
  if (accountStatus(item) === 'normal') return '授权正常'
  if ((item.status || '').toLowerCase() === 'abnormal') return '会话需处理'
  return '请查看风控'
}

function riskTone(item: Terminal) {
  const risk = (item.risk_status || '').toLowerCase()
  const ban = (item.ban_status || '').toLowerCase()
  if (/冻结|受限|禁|ban|封|risk|异常/.test(risk) || /冻结|受限|禁|ban|封/.test(ban)) return 'danger'
  if (/重新导入|观察|告警|sleep/.test(risk)) return 'warning'
  return 'success'
}

function isProfileRestricted(item: Terminal) {
  const text = `${item.risk_status || ''} ${item.ban_status || ''}`.toLowerCase()
  return /冻结|受限|frozen/.test(text)
}

function riskSecondaryText(item: Terminal) {
  if (item.sleep_until) return `冷却至 ${formatDate(item.sleep_until)}`
  if (isProfileRestricted(item)) return '资料修改：已受限'
  return `封禁：${item.ban_status || '正常'}`
}

function displayPhone(item: Terminal) {
  return item.phone_display || item.phone || ''
}

function phonePrefix(item: Terminal) {
  const phone = displayPhone(item)
  if (!phone) return '号码区号未识别'
  const [prefix] = phone.split(' ')
  return prefix.startsWith('+') ? `号码区号 ${prefix}` : '号码区号未识别'
}

function normalizeChannelURL(value: string) {
  const trimmed = (value || '').trim()
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  if (trimmed.startsWith('@')) return `https://t.me/${trimmed.slice(1)}`
  if (trimmed.startsWith('t.me/')) return `https://${trimmed}`
  return `https://t.me/${trimmed.replace(/^\/+/, '')}`
}

function channelName(value: string) {
  const trimmed = (value || '').trim()
  if (!trimmed) return ''
  if (trimmed.startsWith('@')) return trimmed

  const normalized = normalizeChannelURL(trimmed)
  const match = normalized.match(/t\.me\/([^/?#]+)/i)
  return match ? `@${match[1]}` : trimmed
}

function accessTypeText(value: string) {
  if (value === 'session') return 'Session'
  if (value === 'data') return 'TData'
  return value || '未识别'
}

function currentGroupNameByID(id: string) {
  return groupNameMap.value.get(id) || '未知分组'
}

function locationText(country: string, flag: string, fallback = '未识别') {
  const label = (country || '').trim()
  const icon = (flag || '').trim()
  if (!label && !icon) return fallback
  return [label || fallback, icon].filter(Boolean).join(' ')
}

function heartbeatInfo(value: string | null) {
  if (!value) {
    return { primary: '未收到心跳', secondary: '等待下一次同步' }
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return { primary: value, secondary: '时间格式异常' }
  }

  const diff = Date.now() - date.getTime()
  const minute = 60 * 1000
  const hour = 60 * minute
  const day = 24 * hour

  if (diff < minute) {
    return { primary: '刚刚在线', secondary: formatDate(value) }
  }
  if (diff < hour) {
    return { primary: `${Math.max(1, Math.floor(diff / minute))} 分钟前`, secondary: formatDate(value) }
  }
  if (diff < day) {
    return { primary: `${Math.max(1, Math.floor(diff / hour))} 小时前`, secondary: formatDate(value) }
  }
  return { primary: `${Math.max(1, Math.floor(diff / day))} 天前`, secondary: formatDate(value) }
}

function onlineHeartbeat(item: Terminal) {
  const status = (item.online_status || item.status || '').toLowerCase()
  const heartbeat = heartbeatInfo(item.last_online_at)
  if (status === 'online') {
    return { primary: '在线', secondary: heartbeat.secondary }
  }
  if (status === 'offline') {
    return { primary: '离线', secondary: item.last_online_at ? `最后在线 ${formatDate(item.last_online_at)}` : '未返回最后在线' }
  }
  return { primary: item.online_status_text || '未确认', secondary: heartbeat.secondary }
}

function networkPrimary(item: Terminal) {
  return item.exit_ip || '未绑定'
}

function networkSecondary(item: Terminal) {
  if (!item.exit_ip) return '未绑定'
  return locationText(item.exit_country, item.exit_flag, '出口归属地未识别')
}

function totalRuns(item: Terminal) {
  return (item.total_success || 0) + (item.total_failed || 0)
}

function cooldownText(item: Terminal) {
  if (!item.sleep_until) return '未冷却'
  return `冷却至 ${formatDate(item.sleep_until)}`
}

function riskScoreText(item: TerminalRiskStats | null) {
  if (!item) return '未计算'
  return `风险${item.risk_score || '低'}`
}

function riskScoreTone(score: string) {
  if (score.includes('高')) return 'danger'
  if (score.includes('中')) return 'warning'
  return 'success'
}

function quotaUsageText(count: number, limit: number, usage: number) {
  if (!limit) return `${count} / 不限额`
  return `${count} / ${limit} (${usage}%)`
}

function riskAdviceText(item: TerminalRiskStats) {
  if (item.cooldown_active) {
    return '账号刚命中过限流，先让它休息，恢复后再逐步放量。'
  }
  if (item.active_restriction_count >= 3 || item.failure_24h_total >= 10) {
    return '这个号最近命中比较密，建议暂时降频，优先切到更干净的账号。'
  }
  if (item.active_restriction_count > 0 || item.failure_24h_total >= 3) {
    return '已经出现一些目标级限制，建议收紧发送节奏并观察下一轮命中。'
  }
  if (item.dm_hourly_usage >= 80 || item.join_hourly_usage >= 80 || item.dm_daily_usage >= 80 || item.join_daily_usage >= 80) {
    return '额度已经接近上限，继续放量前最好先看一下其他账号的分担情况。'
  }
  return '这个账号目前比较平稳，可以继续保持现有节奏。'
}

function riskBoardItem(item: Terminal) {
  return terminalRiskBoard.value[item.id] || null
}

function terminalRiskLevel(item: Terminal) {
  const score = riskBoardItem(item)?.risk_score || ''
  if (score === '高') return 'high'
  if (score === '中') return 'medium'
  return 'low'
}

function matchesRiskFilter(item: Terminal) {
  if (riskFilter.value === 'all') return true
  return terminalRiskLevel(item) === riskFilter.value
}

function riskSortWeight(item: Terminal) {
  const board = riskBoardItem(item)
  const scoreWeight = board?.risk_score === '高' ? 3000 : board?.risk_score === '中' ? 2000 : 1000
  const activeWeight = (board?.active_restriction_count || 0) * 100
  const failureWeight = Number(board?.failure_24h_total || 0) * 10
  const cooldownWeight = board?.cooldown_active ? 500 : 0
  const quotaWeight = Math.max(board?.dm_hourly_usage || 0, board?.join_hourly_usage || 0, board?.dm_daily_usage || 0, board?.join_daily_usage || 0)
  return scoreWeight + activeWeight + failureWeight + cooldownWeight + quotaWeight
}

function compareTerminalRisk(a: Terminal, b: Terminal) {
  if (sortMode.value === 'cooldown_first') {
    const cooldownGap = Number(Boolean(riskBoardItem(b)?.cooldown_active)) - Number(Boolean(riskBoardItem(a)?.cooldown_active))
    if (cooldownGap !== 0) return cooldownGap
  }
  if (sortMode.value === 'quota_desc') {
    const quotaGap = terminalMaxQuotaUsage(b) - terminalMaxQuotaUsage(a)
    if (quotaGap !== 0) return quotaGap
  }
  const riskGap = riskSortWeight(b) - riskSortWeight(a)
  if (riskGap !== 0) return riskGap
  return (b.total_failed || 0) - (a.total_failed || 0)
}

function terminalMaxQuotaUsage(item: Terminal) {
  const board = riskBoardItem(item)
  if (!board) return 0
  return Math.max(board.dm_hourly_usage, board.dm_daily_usage, board.join_hourly_usage, board.join_daily_usage)
}

function riskScoreBadgeText(item: Terminal) {
  return `风险${riskBoardItem(item)?.risk_score || '低'}`
}

function riskMetaText(item: Terminal) {
  const board = riskBoardItem(item)
  if (!board) return '风控统计待更新'
  const parts = [`24h 命中 ${board.failure_24h_total}`]
  if (board.active_restriction_count > 0) {
    parts.push(`生效中 ${board.active_restriction_count}`)
  }
  if (board.cooldown_active) {
    parts.push(`冷却到 ${formatDate(board.cooldown_until || null)}`)
  }
  return parts.join(' · ')
}

function isBackupCandidate(item: Terminal) {
  const board = riskBoardItem(item)
  if (!board) return false
  if (terminalRiskLevel(item) !== 'low') return false
  if (board.cooldown_active) return false
  if (board.active_restriction_count > 0) return false
  if (accountStatus(item) !== 'normal') return false
  return true
}

function backupMetaText(item: Terminal) {
  const board = riskBoardItem(item)
  if (!board) return '风控统计待更新'
  return [
    `24h 命中 ${board.failure_24h_total}`,
    `最高额度占用 ${terminalMaxQuotaUsage(item)}%`,
    item.last_message_at ? `最近发送 ${formatDate(item.last_message_at)}` : '最近尚未发送'
  ].join(' · ')
}

function restrictionTargetLabel(item: TerminalRestriction) {
  return (item.target_value || '').trim() || '未记录目标'
}

function normalizeLimitInput(value: number | string | null | undefined) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) return 0
  return Math.min(100000, Math.floor(parsed))
}

function formatResetAt(value: string | null | undefined) {
  if (!value) return '待触发'
  return formatDate(value)
}

function formatDate(value: string | null) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function useFallbackAvatar(event: Event) {
  const image = event.target as HTMLImageElement
  if (image.src !== fallbackAvatar) {
    image.src = fallbackAvatar
  }
}

onMounted(load)
</script>
