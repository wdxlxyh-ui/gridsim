<template>
  <template v-if="isLoginPage">
    <router-view />
  </template>
  <template v-else>
    <el-container style="height: 100vh; flex-direction: column">
      <header class="app-header">
        <!-- Bottom glow line (P5) -->
        <div class="header-glow-line"></div>
        <div style="display: flex; align-items: center; gap: 12px">
          <el-button text @click="sidebarCollapsed = !sidebarCollapsed" style="font-size: 18px; padding: 4px; color: #94a3b8">
            <el-icon><Fold /></el-icon>
          </el-button>
          <h2>IEC104 模拟器管理系统 <span class="version-badge">v{{ version }}</span></h2>
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
  </template>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Setting, Monitor, DataLine, Fold, ArrowDown, Connection } from '@element-plus/icons-vue'
import { getStatus, clearToken, type GlobalStatus } from './api'

const route = useRoute()
const router = useRouter()
const sidebarCollapsed = ref(true)
const currentRoute = computed(() => {
  const path = route.path
  if (path.startsWith('/detail/')) return '/monitor'
  return path
})
const isLoginPage = computed(() => route.path === '/login')
const status = ref<GlobalStatus | null>(null)
const version = ref('2.5.2')
const username = ref('')

// Menu items for custom nav
const menuItems = [
  { path: '/config', label: '配置管理', icon: Setting },
  { path: '/monitor', label: '运行监控', icon: Monitor },
  { path: '/trend', label: '实时趋势', icon: DataLine },
  { path: '/proxy', label: '接口测试', icon: Connection },
]

// CountUp animation (P4)
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
  } catch {
    clearToken()
  }
}

function handleUserCommand(cmd: string) {
  if (cmd === 'logout') {
    clearToken()
    username.value = ''
    router.push('/login')
  }
}

watch(() => route.path, updateUserFromToken)
updateUserFromToken()

onMounted(async () => {
  try {
    status.value = await getStatus()
    if (status.value?.version) version.value = status.value.version
    // Trigger count animation
    animateCount(status.value.running, animatedRunning)
    animateCount(status.value.configured, animatedConfigured)
  } catch {}
})
</script>

<style>
:root {
  --header-height: 52px;
  --app-font-size-lg: 16px;
  --app-font-size-base: 14px;
  --app-font-size-sm: 13px;
  --app-font-size-xs: 12px;
  --app-color-text-primary: #303133;
  --app-color-text-regular: #606266;
  --app-color-text-secondary: #909399;
  --app-spacing-base: 16px;
  --app-spacing-sm: 8px;
}
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
*:focus-visible { outline: 2px solid #409eff; outline-offset: 2px; }
.el-button:focus-visible { outline: 2px solid #409eff; outline-offset: 1px; }

/* ─── Page transition: slide+fade (P3) ─── */
.slide-fade-enter-active {
  transition: opacity 0.25s ease, transform 0.3s cubic-bezier(0.22, 1, 0.36, 1);
}
.slide-fade-leave-active {
  transition: opacity 0.15s ease, transform 0.2s ease;
}
.slide-fade-enter-from {
  opacity: 0;
  transform: translateX(16px);
}
.slide-fade-leave-to {
  opacity: 0;
  transform: translateX(-8px);
}

/* ─── Header (P5 glow line) ─── */
.app-header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background: #0a0e17;
  border-bottom: 1px solid #1e293b;
  box-sizing: border-box;
  position: relative;
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
    transparent 100%
  );
  background-size: 200% 100%;
  animation: glow-slide 4s linear infinite;
}

@keyframes glow-slide {
  0%   { background-position: 100% 0; }
  100% { background-position: -100% 0; }
}

.app-header h2 {
  margin: 0;
  font-size: var(--app-font-size-lg);
  font-weight: 600;
  color: #e2e8f0;
}

.version-badge {
  font-size: 12px;
  font-weight: 400;
  color: #64748b;
  padding: 1px 6px;
  background: rgba(30,41,59,0.5);
  border-radius: 4px;
}

.header-status-tag {
  font-size: 12px;
}

.count-up {
  display: inline-block;
  font-variant-numeric: tabular-nums;
  min-width: 1ch;
  text-align: center;
  transition: color 0.3s;
}

/* ─── Sidebar with PillNav (P2) ─── */
.app-sidebar {
  width: 200px;
  border-right: 1px solid #1e293b;
  background: #111827;
  transition: width 0.3s cubic-bezier(0.22, 1, 0.36, 1);
  overflow: hidden;
}

.app-sidebar.collapsed {
  width: 64px;
}

.sidebar-nav {
  display: flex;
  flex-direction: column;
  padding: 8px;
  gap: 2px;
}

.nav-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  color: #94a3b8;
  transition: color 0.2s, background 0.2s, padding 0.3s cubic-bezier(0.22, 1, 0.36, 1);
  white-space: nowrap;
  overflow: hidden;
  user-select: none;
}

.nav-item:hover {
  color: #e2e8f0;
  background: rgba(30, 41, 59, 0.5);
}

.nav-item.active {
  color: #f59e0b;
  background: rgba(245,158,11,0.1);
}

/* Pill indicator */
.nav-pill {
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%) scaleY(0);
  width: 3px;
  height: 20px;
  border-radius: 0 3px 3px 0;
  background: #f59e0b;
  transition: transform 0.25s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.nav-item.active .nav-pill {
  transform: translateY(-50%) scaleY(1);
}

.nav-label {
  font-size: 14px;
  transition: opacity 0.2s;
}

.collapsed .nav-label {
  opacity: 0;
  pointer-events: none;
}

.collapsed .nav-item {
  justify-content: center;
  padding: 10px;
}

.collapsed .nav-item .nav-pill {
  display: none;
}

/* ─── Main content area ─── */
.app-main {
  background: #0f1520;
  padding: 16px;
  overflow-y: auto;
}

/* TrendPage dark card overrides */
.app-main .el-card {
  background: #111827 !important;
  border: 1px solid #1e293b !important;
  color: #e2e8f0;
}
.app-main .el-card__body {
  background: transparent;
}
.app-main .el-radio-button__inner {
  background: #1a1f2e !important;
  border-color: #1e293b !important;
  color: #94a3b8 !important;
}
.app-main .el-radio-button__original-radio:checked + .el-radio-button__inner {
  background: #f59e0b !important;
  border-color: #f59e0b !important;
  color: #000 !important;
  box-shadow: none !important;
}
.app-main .el-tag {
  --el-tag-bg-color: transparent;
}
.app-main .el-dialog {
  background: #111827 !important;
  border: 1px solid #1e293b !important;
}
.app-main .el-dialog__title {
  color: #e2e8f0 !important;
}
.app-main .el-dialog__body {
  background: #111827 !important;
}
.app-main .el-select-dropdown {
  background: #1a1f2e !important;
  border: 1px solid #1e293b !important;
}
.app-main .el-select-dropdown__item {
  color: #94a3b8 !important;
}
.app-main .el-select-dropdown__item.hover {
  background: #1e293b !important;
  color: #e2e8f0 !important;
}
.app-main .el-select-dropdown__item.selected {
  color: #f59e0b !important;
}
.app-main .el-input__wrapper {
  background: #0a0e17 !important;
  border-color: #1e293b !important;
  box-shadow: none !important;
}
.app-main .el-input__inner {
  color: #e2e8f0 !important;
}
</style>
