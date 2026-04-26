import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useUserStore } from '../stores/user'
import LoginView from '../views/LoginView.vue'
import AppShell from '../components/AppShell.vue'
import DashboardView from '../views/DashboardView.vue'
import UsersView from '../views/UsersView.vue'
import TerminalsView from '../views/TerminalsView.vue'
import ImportCenterView from '../views/ImportCenterView.vue'
import NetworkNodesView from '../views/NetworkNodesView.vue'
import TargetPoolView from '../views/TargetPoolView.vue'
import WorkflowView from '../views/WorkflowView.vue'
import ProfileAssetsView from '../views/ProfileAssetsView.vue'
import TaskCenterView from '../views/TaskCenterView.vue'
import LogsCenterView from '../views/LogsCenterView.vue'
import OutreachSyncView from '../views/OutreachSyncView.vue'
import DirectMessagesView from '../views/DirectMessagesView.vue'
import SettingsView from '../views/SettingsView.vue'
import BotSettingsView from '../views/BotSettingsView.vue'
import BotUsersDashboardView from '../views/BotUsersDashboardView.vue'
import ListenerAdminView from '../views/ListenerAdminView.vue'

const routes: RouteRecordRaw[] = [
  { path: '/login', component: LoginView },
  {
    path: '/',
    component: AppShell,
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', component: DashboardView },
      { path: 'users', component: UsersView, meta: { requiresAdmin: true } },
      { path: 'terminals', component: TerminalsView },
      { path: 'import', component: ImportCenterView },
      { path: 'network', component: NetworkNodesView },
      { path: 'targets', component: TargetPoolView },
      { path: 'workflow', component: WorkflowView },
      { path: 'profile-assets', component: ProfileAssetsView },
      { path: 'tasks', component: TaskCenterView },
      { path: 'logs', component: LogsCenterView },
      { path: 'tasks-logs', redirect: '/tasks' },
      { path: 'outreach-sync', component: OutreachSyncView },
      { path: 'direct-messages', component: DirectMessagesView },
      { path: 'bot-settings', component: BotSettingsView, meta: { requiresAdmin: true } },
      { path: 'bot-users', component: BotUsersDashboardView, meta: { requiresAdmin: true } },
      { path: 'listener-admin', component: ListenerAdminView, meta: { requiresAdmin: true } },
      { path: 'settings', component: SettingsView, meta: { requiresAdmin: true } }
    ]
  }
]

export const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to) => {
  const store = useUserStore()
  await store.bootstrap()
  if (to.meta.requiresAuth && !store.token) {
    return '/login'
  }
  if (to.meta.requiresAdmin && store.user?.role !== 'admin') {
    return '/dashboard'
  }
  if (to.path === '/login' && store.token) {
    return '/dashboard'
  }
  return true
})
