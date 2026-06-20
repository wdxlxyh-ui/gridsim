<template>
  <!-- Initial loading skeleton -->
  <SkeletonScreen v-if="!data && loading" type="dashboard" />
  <!-- Error / empty state -->
  <div v-else-if="!data && !loading" class="empty-state">
    <el-empty description="无法加载仪表盘数据">
      <el-button type="primary" @click="fetchDashboard">重新加载</el-button>
    </el-empty>
  </div>
  <!-- Dashboard content -->
  <div v-else class="dashboard">
    <!-- Page Header -->
    <div class="page-header">
      <div>
        <h1 class="page-title">仪表盘</h1>
        <p class="page-subtitle">实例运行状态总览</p>
      </div>
      <div class="header-actions">
        <el-button size="small" text @click="fetchDashboard" :loading="loading" style="color: var(--text-muted)">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
        <el-tag v-if="lastRefresh" size="small" effect="plain" type="info">
          更新于 {{ lastRefresh }}
        </el-tag>
      </div>
    </div>

    <!-- Summary Cards Row -->
    <div class="stats-grid">
      <div class="stat-card" :class="{ active: filterStatus === '' }" @click="filterStatus = ''">
        <div class="stat-icon-wrap icon-total">
          <el-icon :size="22"><Grid /></el-icon>
        </div>
        <div class="stat-body">
          <div class="stat-value accent-gradient">{{ animatedTotal }}</div>
          <div class="stat-label">总实例</div>
        </div>
        <div class="stat-trend">
          <span class="trend-dot running-bg"></span>
          <span class="trend-text">{{ data?.running_instances ?? 0 }} 运行中</span>
        </div>
        <div class="stat-glow stat-glow-total"></div>
      </div>
      <div class="stat-card" :class="{ active: filterStatus === 'running' }" @click="filterStatus = 'running'">
        <div class="stat-icon-wrap icon-running">
          <el-icon :size="22"><CaretRight /></el-icon>
        </div>
        <div class="stat-body">
          <div class="stat-value running-text">{{ animatedRunning }}</div>
          <div class="stat-label">运行中</div>
        </div>
        <div class="stat-trend">
          <span class="trend-up">↑</span>
          <span class="trend-text">{{ data?.total_instances ? Math.round(data.running_instances / data.total_instances * 100) : 0 }}%</span>
        </div>
        <div class="stat-glow stat-glow-running"></div>
      </div>
      <div class="stat-card" :class="{ active: filterStatus === 'stopped' }" @click="filterStatus = 'stopped'">
        <div class="stat-icon-wrap icon-stopped">
          <el-icon :size="22"><VideoPause /></el-icon>
        </div>
        <div class="stat-body">
          <div class="stat-value stopped-text">{{ animatedStopped }}</div>
          <div class="stat-label">已停止</div>
        </div>
        <div class="stat-trend">
          <span class="trend-down">↓</span>
          <span class="trend-text">{{ data?.total_instances ? Math.round(data.stopped_instances / data.total_instances * 100) : 0 }}%</span>
        </div>
        <div class="stat-glow stat-glow-stopped"></div>
      </div>
      <div class="stat-card" @click="filterStatus = ''">
        <div class="stat-icon-wrap icon-clients">
          <el-icon :size="22"><Connection /></el-icon>
        </div>
        <div class="stat-body">
          <div class="stat-value clients-text">{{ animatedClients }}</div>
          <div class="stat-label">在线客户端</div>
        </div>
        <div class="stat-trend">
          <span class="trend-up">↑</span>
          <span class="trend-text">{{ data?.running_instances ? Math.round((data.clients_connected || 0) / data.running_instances * 100) : 0 }}% 在线率</span>
        </div>
        <div class="stat-glow stat-glow-clients"></div>
      </div>
    </div>

    <!-- Protocol Distribution & Instance List -->
    <div class="content-grid">
      <!-- Protocol Breakdown -->
      <el-card shadow="never" class="proto-card">
        <template #header>
          <div class="card-header-with-action">
            <span class="card-title">
              <el-icon size="16" style="margin-right: 6px; vertical-align: -2px"><DataBoard /></el-icon>
              规约分布
            </span>
          </div>
        </template>
        <div v-if="data?.by_protocol && Object.keys(data.by_protocol).length > 0" class="proto-list">
          <div v-for="(count, proto) in data.by_protocol" :key="proto" class="proto-row">
            <div class="proto-info">
              <div class="proto-dot" :style="{ background: protoColor(proto) }"></div>
              <span class="proto-name">{{ protoLabel(proto) }}</span>
            </div>
            <div class="proto-meta">
              <span class="proto-count">{{ count }}</span>
              <div class="proto-bar-track">
                <div class="proto-bar-fill" :style="{ width: (count / data.total_instances * 100) + '%', background: protoColor(proto) }"></div>
              </div>
            </div>
          </div>
        </div>
        <el-empty v-else description="暂无数据" :image-size="60" />
      </el-card>

      <!-- Instance Quick List -->
      <el-card shadow="never" class="instance-section-card">
        <template #header>
          <div class="card-header-with-action">
            <span class="card-title">
              <el-icon size="16" style="margin-right: 6px; vertical-align: -2px"><Monitor /></el-icon>
              实例概览
              <el-tag v-if="filterStatus" size="small" closable @close="filterStatus = ''" style="margin-left: 8px">
                {{ filterStatus === 'running' ? '运行中' : filterStatus === 'stopped' ? '已停止' : '异常' }}
              </el-tag>
            </span>
            <el-button size="small" text class="view-all-btn" @click="$router.push('/config')">
              管理全部 <el-icon><ArrowRight /></el-icon>
            </el-button>
          </div>
        </template>
        <div v-if="filteredInstances.length > 0" class="instance-grid">
          <div
            v-for="inst in filteredInstances" :key="inst.id"
            class="instance-card"
            @click="goToDetail(inst)"
          >
            <div class="instance-card-glow"></div>
            <div class="ic-header">
              <div class="ic-status-row">
                <span class="status-indicator" :class="inst.status">
                  <span class="status-pulse" :class="inst.status"></span>
                </span>
                <span class="ic-status-text" :class="inst.status">{{ statusLabel(inst.status) }}</span>
              </div>
              <div class="ic-protocol">
                <el-tag :type="protoTag(inst.protocol)" size="small" effect="plain">{{ protoLabel(inst.protocol) }}</el-tag>
              </div>
            </div>
            <div class="ic-name">{{ inst.name }}</div>
            <div class="ic-detail-row">
              <div class="ic-detail-item">
                <el-icon :size="12"><Connection /></el-icon>
                <span>{{ inst.client_connected ? '在线' : '离线' }}</span>
              </div>
              <div class="ic-detail-divider"></div>
              <div class="ic-detail-item">
                <el-icon :size="12"><DataLine /></el-icon>
                <span>{{ inst.total_points || 0 }} 测点</span>
              </div>
              <div class="ic-detail-divider"></div>
              <div class="ic-detail-item">
                <el-icon :size="12"><Clock /></el-icon>
                <span>{{ fmtUptime(inst.uptime_seconds || 0) }}</span>
              </div>
            </div>
            <div class="ic-port-info">
              <el-icon :size="11"><Coin /></el-icon>
              <span>端口 {{ inst.port }}</span>
            </div>
            <div v-if="inst.error" class="ic-error">
              <el-icon :size="12"><WarningFilled /></el-icon>
              <span>{{ inst.error }}</span>
            </div>
          </div>
        </div>
        <el-empty v-else description="暂无符合条件的实例" :image-size="60" />
      </el-card>
    </div>

    <!-- Quick Actions -->
    <el-card shadow="never" class="quick-actions-card">
      <template #header>
        <span class="card-title">
          <el-icon size="16" style="margin-right: 6px; vertical-align: -2px"><Lightning /></el-icon>
          快捷操作
        </span>
      </template>
      <div class="quick-actions">
        <div class="action-item" @click="$router.push('/config')">
          <div class="action-icon-wrap">
            <el-icon :size="24"><Plus /></el-icon>
          </div>
          <div class="action-info">
            <span class="action-label">新建实例</span>
            <span class="action-desc">添加仿真实例配置</span>
          </div>
        </div>
        <div class="action-item" @click="$router.push('/monitor')">
          <div class="action-icon-wrap icon-monitor">
            <el-icon :size="24"><Monitor /></el-icon>
          </div>
          <div class="action-info">
            <span class="action-label">运行监控</span>
            <span class="action-desc">全局实例状态看板</span>
          </div>
        </div>
        <div class="action-item" @click="$router.push('/trend')">
          <div class="action-icon-wrap icon-trend">
            <el-icon :size="24"><DataLine /></el-icon>
          </div>
          <div class="action-info">
            <span class="action-label">实时趋势</span>
            <span class="action-desc">测点数据趋势对比</span>
          </div>
        </div>
        <div class="action-item" @click="$router.push('/proxy')">
          <div class="action-icon-wrap icon-proxy">
            <el-icon :size="24"><Connection /></el-icon>
          </div>
          <div class="action-info">
            <span class="action-label">接口测试</span>
            <span class="action-desc">HTTP API 调试工具</span>
          </div>
        </div>
      </div>
    </el-card>
  </div>

  <!-- Floating Action Button: Onboarding Guide -->
  <div class="onboard-fab" @click="guideRef?.start()" title="操作引导">
    <span class="onboard-fab-icon">❓</span>
    <span class="onboard-fab-label">操作引导</span>
  </div>

  <!-- Onboarding Guide Overlay -->
  <OnboardingGuide ref="guideRef" />
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  Grid, CaretRight, VideoPause, DataLine, Connection,
  Plus, Monitor, ArrowRight, Clock, Coin, Lightning, DataBoard, Refresh,
} from '@element-plus/icons-vue'
import { getDashboard, type DashboardData, type DashboardBriefInstance } from '../api'
import SkeletonScreen from '../components/SkeletonScreen.vue'
import OnboardingGuide from '../components/OnboardingGuide.vue'

const guideRef = ref<InstanceType<typeof OnboardingGuide> | null>(null)

const router = useRouter()
const data = ref<DashboardData | null>(null)
const loading = ref(false)
const filterStatus = ref('')
const lastRefresh = ref('')

// Animated counters
const animatedTotal = ref(0)
const animatedRunning = ref(0)
const animatedStopped = ref(0)
const animatedClients = ref(0)

let refreshTimer: ReturnType<typeof setInterval> | null = null

const filteredInstances = computed(() => {
  if (!data.value) return []
  if (!filterStatus.value) return data.value.instances
  return data.value.instances.filter(i => i.status === filterStatus.value)
})

function animateTo(target: number, current: { value: number }) {
  const start = current.value
  const diff = target - start
  if (diff === 0) return
  const duration = 800
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

function protoColor(proto?: string): string {
  if (proto === 'modbus_tcp') return '#10b981'
  if (proto === 'microgrid') return '#f59e0b'
  return '#3b82f6'
}

function protoLabel(proto?: string): string {
  if (proto === 'modbus_tcp') return 'Modbus TCP'
  if (proto === 'microgrid') return '微电网'
  return 'IEC104'
}

function protoTag(proto?: string): 'success' | 'warning' | 'primary' {
  if (proto === 'modbus_tcp') return 'success'
  if (proto === 'microgrid') return 'warning'
  return 'primary'
}

function statusLabel(s: string): string {
  if (s === 'running') return '运行中'
  if (s === 'error') return '异常'
  return '已停止'
}

function fmtUptime(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h > 0 ? `${h}h${m}m` : `${m}m`
}

function goToDetail(inst: DashboardBriefInstance) {
  if (inst.protocol === 'microgrid') {
    router.push(`/microgrid/${inst.id}`)
  } else {
    router.push(`/detail/${inst.id}`)
  }
}

async function fetchDashboard() {
  loading.value = true
  try {
    data.value = await getDashboard()
    const d = data.value
    animateTo(d.total_instances, animatedTotal)
    animateTo(d.running_instances, animatedRunning)
    animateTo(d.stopped_instances, animatedStopped)
    animateTo(d.clients_connected, animatedClients)
    lastRefresh.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    // silent
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchDashboard()
  refreshTimer = setInterval(fetchDashboard, 30000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
.dashboard {
  --accent-rgb: 59, 130, 246;
  max-width: 1280px;
  margin: 0 auto;
  padding: 4px 0;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 400px;
}

/* ─── Page Header ─── */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 20px;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  background: linear-gradient(135deg, var(--text-primary), var(--text-secondary));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.page-subtitle {
  margin: 4px 0 0 0;
  font-size: 13px;
  color: var(--text-muted, #64748b);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* ─── Stats Grid ─── */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.stat-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
  border-radius: 12px;
  background: var(--el-bg-color-overlay, #1e293b);
  border: 1px solid var(--border-color, #2d3748);
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.22, 1, 0.36, 1);
  overflow: hidden;
}

.stat-card:hover {
  transform: translateY(-3px);
  border-color: rgba(var(--accent-rgb), 0.3);
  box-shadow: 0 8px 24px rgba(0,0,0,0.3);
}

.stat-card.active {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px rgba(var(--accent-rgb), 0.2), 0 4px 16px rgba(var(--accent-rgb), 0.1);
}

.stat-icon-wrap {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  flex-shrink: 0;
}

.icon-total { background: rgba(var(--accent-rgb), 0.15); color: var(--accent); }
.icon-running { background: rgba(16, 185, 129, 0.15); color: #10b981; }
.icon-stopped { background: rgba(148, 163, 184, 0.15); color: #94a3b8; }
.icon-clients { background: rgba(139, 92, 246, 0.15); color: #8b5cf6; }

.stat-body { flex: 1; }

.stat-value {
  font-size: 24px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  line-height: 1.1;
  letter-spacing: -0.02em;
}

.accent-gradient {
  background: linear-gradient(135deg, var(--accent), #60a5fa);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.running-text { color: #10b981; }
.stopped-text { color: #94a3b8; }
.clients-text { color: #8b5cf6; }

.stat-label {
  font-size: 12px;
  color: var(--text-muted, #64748b);
  margin-top: 2px;
  font-weight: 500;
}

.stat-trend {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
}

.trend-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.running-bg { background: #10b981; box-shadow: 0 0 6px rgba(16, 185, 129, 0.6); }

.trend-up { color: #10b981; font-weight: 700; font-size: 12px; }
.trend-down { color: #94a3b8; font-weight: 700; font-size: 12px; }
.trend-text { color: var(--text-muted, #64748b); }

.stat-glow {
  position: absolute;
  top: -50%;
  right: -50%;
  width: 100px;
  height: 100px;
  border-radius: 50%;
  opacity: 0.04;
  pointer-events: none;
  transition: opacity 0.3s;
}

.stat-card:hover .stat-glow { opacity: 0.08; }
.stat-glow-total { background: radial-gradient(circle, var(--accent), transparent); }
.stat-glow-running { background: radial-gradient(circle, #10b981, transparent); }
.stat-glow-stopped { background: radial-gradient(circle, #94a3b8, transparent); }
.stat-glow-clients { background: radial-gradient(circle, #8b5cf6, transparent); }

/* ─── Card Header ─── */
.card-header-with-action {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  display: flex;
  align-items: center;
}

.view-all-btn {
  color: var(--text-muted, #64748b);
  font-size: 12px;
  transition: color 0.2s;
}

.view-all-btn:hover {
  color: var(--accent);
}

/* ─── Content Grid ─── */
.content-grid {
  display: grid;
  grid-template-columns: 260px 1fr;
  gap: 16px;
  margin-bottom: 16px;
}

/* ─── Protocol Card ─── */
.proto-card {
  border: 1px solid var(--border-color, #2d3748);
  border-radius: 10px;
}

.proto-card :deep(.el-card__body) {
  padding: 12px 16px;
}

.proto-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.proto-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.proto-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.proto-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.proto-name {
  font-size: 13px;
  font-weight: 500;
  flex: 1;
}

.proto-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-left: 16px;
}

.proto-count {
  font-size: 15px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  min-width: 20px;
  text-align: right;
  color: var(--text-primary);
}

.proto-bar-track {
  flex: 1;
  height: 5px;
  background: var(--el-fill-color-lighter, #1e293b);
  border-radius: 3px;
  overflow: hidden;
}

.proto-bar-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.8s cubic-bezier(0.22, 1, 0.36, 1);
}

/* ─── Instance Section Card ─── */
.instance-section-card {
  border: 1px solid var(--border-color, #2d3748);
  border-radius: 10px;
}

.instance-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 10px;
}

.instance-card {
  position: relative;
  padding: 14px;
  border-radius: 10px;
  border: 1px solid var(--border-color, #2d3748);
  background: var(--el-bg-color, #1a202c);
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.22, 1, 0.36, 1);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.instance-card:hover {
  transform: translateY(-2px);
  border-color: rgba(var(--accent-rgb), 0.4);
  box-shadow: 0 6px 20px rgba(var(--accent-rgb), 0.12);
}

.instance-card-glow {
  position: absolute;
  top: -50%;
  right: -50%;
  width: 80px;
  height: 80px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(var(--accent-rgb), 0.06), transparent);
  pointer-events: none;
  transition: opacity 0.3s;
}

.instance-card:hover .instance-card-glow { opacity: 1; }

.ic-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.ic-status-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-indicator {
  position: relative;
  width: 8px;
  height: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.status-indicator.running::after,
.status-indicator.stopped::after,
.status-indicator.error::after {
  content: '';
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-indicator.running::after { background: #10b981; }
.status-indicator.stopped::after { background: #64748b; }
.status-indicator.error::after { background: #f56c6c; }

.status-pulse {
  position: absolute;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  animation: pulse-dot 2s ease-in-out infinite;
}

.status-pulse.running {
  background: rgba(16, 185, 129, 0.25);
}

.status-pulse.stopped {
  background: rgba(100, 116, 139, 0.2);
  animation: none;
}

.status-pulse.error {
  background: rgba(245, 108, 108, 0.25);
}

@keyframes pulse-dot {
  0%, 100% { transform: scale(1); opacity: 0.6; }
  50% { transform: scale(1.8); opacity: 0; }
}

.ic-status-text {
  font-size: 11px;
  font-weight: 500;
}

.ic-status-text.running { color: #10b981; }
.ic-status-text.stopped { color: #64748b; }
.ic-status-text.error { color: #f56c6c; }

.ic-protocol { flex-shrink: 0; }

.ic-name {
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-primary);
  margin: 2px 0;
}

.ic-detail-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.ic-detail-item {
  display: flex;
  align-items: center;
  gap: 3px;
}

.ic-detail-divider {
  width: 1px;
  height: 10px;
  background: var(--border-color, #2d3748);
}

.ic-port-info {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.ic-error {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: #f56c6c;
  background: rgba(245, 108, 108, 0.08);
  padding: 4px 6px;
  border-radius: 4px;
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ─── Quick Actions ─── */
.quick-actions-card {
  border: 1px solid var(--border-color, #2d3748);
  border-radius: 10px;
}

.quick-actions {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}

.action-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid var(--border-color, #2d3748);
  background: var(--el-bg-color, #1a202c);
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.22, 1, 0.36, 1);
}

.action-item:hover {
  transform: translateY(-2px);
  border-color: rgba(var(--accent-rgb), 0.3);
  box-shadow: 0 4px 16px rgba(0,0,0,0.25);
  background: rgba(var(--accent-rgb), 0.03);
}

.action-icon-wrap {
  width: 42px;
  height: 42px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  background: rgba(var(--accent-rgb), 0.12);
  color: var(--accent);
  flex-shrink: 0;
  transition: transform 0.2s;
}

.action-item:hover .action-icon-wrap {
  transform: scale(1.08);
}

.icon-monitor { background: rgba(var(--accent-rgb), 0.15); color: var(--accent); }
.icon-trend { background: rgba(139, 92, 246, 0.15); color: #8b5cf6; }
.icon-proxy { background: rgba(16, 185, 129, 0.15); color: #10b981; }

.action-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.action-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.action-desc {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

/* ─── Responsive ─── */
@media (max-width: 1200px) {
  .stats-grid { grid-template-columns: repeat(3, 1fr); }
}

@media (max-width: 900px) {
  .content-grid { grid-template-columns: 1fr; }
  .quick-actions { grid-template-columns: repeat(2, 1fr); }
}

@media (max-width: 640px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .quick-actions { grid-template-columns: 1fr; }
}

/* ─── Onboarding FAB ─── */
.onboard-fab {
  position: fixed;
  right: 28px;
  bottom: 28px;
  z-index: 7000;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 18px 0 14px;
  height: 44px;
  border-radius: 22px;
  background: linear-gradient(135deg, #1e40af, #3b82f6);
  box-shadow: 0 4px 20px rgba(59, 130, 246, 0.45);
  cursor: pointer;
  transition: transform 0.2s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.2s;
  user-select: none;
}

.onboard-fab:hover {
  transform: translateY(-3px) scale(1.04);
  box-shadow: 0 8px 28px rgba(59, 130, 246, 0.6);
}

.onboard-fab:active {
  transform: scale(0.97);
}

.onboard-fab-icon {
  font-size: 18px;
  line-height: 1;
}

.onboard-fab-label {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  white-space: nowrap;
}
</style>
