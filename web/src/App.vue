<template>
  <template v-if="isLoginPage">
    <router-view />
  </template>
  <template v-else>
    <el-container style="height: 100vh; flex-direction: column">
      <header class="app-header">
        <div class="header-glow-line"></div>
        <div style="display: flex; align-items: center; gap: 12px">
          <el-button text @click="sidebarCollapsed = !sidebarCollapsed" style="font-size: 18px; padding: 4px; color: #94a3b8">
            <el-icon><Fold /></el-icon>
          </el-button>
          <h2>GridSim <span class="version-badge">v{{ version }}</span></h2>
        </div>
        <div style="display: flex; align-items: center; gap: 12px; flex: 1; max-width: 360px; margin: 0 16px;">
          <el-input
            v-model="globalSearch"
            placeholder="搜索实例..."
            :prefix-icon="Search"
            size="small"
            clearable
            @keyup.enter="handleGlobalSearch"
            style="width: 100%"
          />
        </div>
        <div style="display: flex; align-items: center; gap: 12px">
          <el-tag v-if="status" class="header-status-tag">
            运行 <span class="count-up">{{ animatedRunning }}</span> / 总计 <span class="count-up">{{ animatedConfigured }}</span>
          </el-tag>
          <el-dropdown trigger="click" @command="handleUserCommand">
            <el-button text style="color: #94a3b8; font-size: 13px">
              {{ username || '用户' }}
              <el-icon><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>
      <el-container style="height: calc(100vh - var(--header-height))">
        <el-aside class="app-sidebar" :class="{ collapsed: sidebarCollapsed }"
          @mouseenter="sidebarCollapsed = false" @mouseleave="sidebarCollapsed = true">
          <nav class="sidebar-nav">
            <div
              v-for="item in menuItems" :key="item.path"
              class="nav-item"
              :class="{ active: currentRoute === item.path || (item.path === '/monitor' && currentRoute.startsWith('/detail')) }"
              @click="$router.push(item.path)"
            >
              <div class="nav-pill"></div>
              <el-icon :size="18"><component :is="item.icon" /></el-icon>
              <span class="nav-label">{{ item.label }}</span>
            </div>
          </nav>
        </el-aside>
        <el-main class="app-main">
          <router-view v-slot="{ Component }">
            <transition name="slide-fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </el-main>
      </el-container>
    </el-container>
    <CommandPalette />
  </template>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Grid, Setting, Monitor, DataLine, Fold, ArrowDown, Connection, Search } from '@element-plus/icons-vue'
import { getStatus, clearToken, type GlobalStatus } from './api'
import CommandPalette from './components/CommandPalette.vue'

const route = useRoute()
const router = useRouter()
const sidebarCollapsed = ref(true)
const globalSearch = ref('')
const currentRoute = computed(() => {
  const path = route.path
  if (path.startsWith('/detail/')) return '/monitor'
  if (path.startsWith('/microgrid/')) return '/config'
  return path
})

function handleGlobalSearch() {
  const q = globalSearch.value.trim()
  if (!q) return
  router.push({ path: '/config', query: { search: q } })
}
const isLoginPage = computed(() => route.path === '/login')
const status = ref<GlobalStatus | null>(null)
const version = ref('3.0.1')
const username = ref('')

const menuItems = [
  { path: '/dashboard', label: '仪表盘', icon: Grid },
  { path: '/config', label: '配置管理', icon: Setting },
  { path: '/monitor', label: '运行监控', icon: Monitor },
  { path: '/trend', label: '实时趋势', icon: DataLine },
  { path: '/proxy', label: '接口测试', icon: Connection },
]

const animatedRunning = ref(0)
const animatedConfigured = ref(0)

function animateCount(target: number, current: { value: number }) {
  const start = current.value
  const diff = target - start
  if (diff === 0) return
  const duration = 600
  const startTime = performance.now()
  function tick(now: number) {
    const elapsed = now - startTime
    const progress = Math.min(elapsed / duration, 1)
    const eased = 1 - Math.pow(1 - progress, 3)
    current.value = Math.round(start + diff * eased)
    if (progress < 1) requestAnimationFrame(tick)
  }
  requestAnimationFrame(tick)
}

function updateUserFromToken() {
  username.value = ''
  try {
    const raw = localStorage.getItem('iec104_token')
    if (!raw) return
    const parts = raw.split('.')
    if (parts.length !== 3) return
    const payload = JSON.parse(atob(parts[1]))
    if (payload.username) username.value = payload.username
  } catch { clearToken() }
}

function handleUserCommand(cmd: string) {
  if (cmd === 'logout') { clearToken(); username.value = ''; router.push('/login') }
}

watch(() => route.path, updateUserFromToken)
updateUserFromToken()

let statusTimer: ReturnType<typeof setInterval> | null = null

async function refreshStatus() {
  try {
    const s = await getStatus()
    status.value = s
    if (s?.version) version.value = s.version
    animateCount(s.running, animatedRunning)
    animateCount(s.configured, animatedConfigured)
  } catch {}
}

onMounted(async () => {
  await refreshStatus()
  statusTimer = setInterval(refreshStatus, 15000)
  window.addEventListener('toggle-sidebar', () => { sidebarCollapsed.value = !sidebarCollapsed.value })
})

onUnmounted(() => {
  if (statusTimer) clearInterval(statusTimer)
  window.removeEventListener('toggle-sidebar', () => {})
})
</script>

<style>
:root {
  --header-height: 48px;
}
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: var(--bg-primary); color: var(--text-primary); }
*:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.el-button:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }

.slide-fade-enter-active {
  transition: opacity 0.28s ease, transform 0.32s cubic-bezier(0.22, 1, 0.36, 1);
}
.slide-fade-leave-active {
  transition: opacity 0.18s ease, transform 0.22s ease;
}
.slide-fade-enter-from {
  opacity: 0;
  transform: translateX(20px);
}
.slide-fade-leave-to {
  opacity: 0;
  transform: translateX(-10px);
}

.app-header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background: var(--header-bg);
  border-bottom: 1px solid var(--border-color);
  box-sizing: border-box;
  position: relative;
  z-index: 100;
}

.header-glow-line {
  position: absolute;
  bottom: -1px;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg,
    transparent 0%,
    rgba(59,130,246,0.4) 20%,
    rgba(139,92,246,0.6) 50%,
    rgba(59,130,246,0.4) 80%,
    transparent 100%);
  background-size: 200% 100%;
  animation: glow-slide 4s linear infinite;
}

@keyframes glow-slide {
  0%   { background-position: 100% 0; }
  100% { background-position: -100% 0; }
}

.app-header h2 { margin: 0; font-size: 16px; font-weight: 600; color: var(--text-primary); display: flex; align-items: center; gap: 10px; }

.app-header h2::before {
  content: '';
  display: inline-block;
  width: 8px;
  height: 8px;
  background: var(--accent);
  border-radius: 50%;
  box-shadow: var(--accent-glow);
}

.version-badge {
  font-size: 12px;
  font-weight: 400;
  color: var(--text-muted);
  padding: 1px 6px;
  background: rgba(30,41,59,0.5);
  border-radius: 4px;
}

.header-status-tag { font-size: 12px; }

.count-up {
  display: inline-block;
  font-variant-numeric: tabular-nums;
  min-width: 1ch;
  text-align: center;
  transition: color 0.3s;
}

/* Sidebar PillNav */
.app-sidebar {
  width: var(--sidebar-width);
  border-right: 1px solid var(--border-color);
  background: var(--sidebar-bg);
  transition: width 0.3s cubic-bezier(0.22, 1, 0.36, 1);
  overflow: hidden;
}

.app-sidebar.collapsed { width: var(--sidebar-collapsed); }

.sidebar-nav { display: flex; flex-direction: column; padding: 8px; gap: 2px; }

.nav-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  cursor: pointer;
  color: var(--text-secondary);
  transition: color 0.2s, background 0.2s, padding 0.3s cubic-bezier(0.22, 1, 0.36, 1);
  white-space: nowrap;
  overflow: hidden;
  user-select: none;
}

.nav-item:hover { color: var(--text-primary); background: var(--sidebar-hover); }

.nav-item.active { color: var(--accent); background: var(--sidebar-active); }

.nav-pill {
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%) scaleY(0);
  width: 3px;
  height: 20px;
  border-radius: 0 3px 3px 0;
  background: var(--accent);
  transition: transform 0.25s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.nav-item.active .nav-pill { transform: translateY(-50%) scaleY(1); }

.nav-label { font-size: 14px; transition: opacity 0.2s; }

.collapsed .nav-label { display: none; }
.collapsed .nav-item { justify-content: center; padding: 10px; gap: 0; }
.collapsed .nav-item .nav-pill { display: none; }

.app-main {
  background: var(--bg-primary);
  padding: 16px;
  overflow-y: auto;
}
</style>
