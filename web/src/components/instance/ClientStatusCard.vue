<template>
  <el-card shadow="never" class="client-status-card">
    <template #header>
      <div class="csc-header">
        <div class="csc-header-left">
          <el-tag type="info" size="small" class="csc-proto-tag">IEC104 客户端</el-tag>
          <span class="csc-title">连接状态</span>
        </div>
        <div class="csc-header-right">
          <el-tag
            :type="connectionStatus === 'connected' ? 'success' : 'danger'"
            size="small"
            class="csc-status-badge"
          >
            <span class="csc-status-dot" :class="connectionStatus" />
            {{ connectionStatus === 'connected' ? '已连接' : '未连接' }}
          </el-tag>
          <el-tooltip content="刷新连接状态" placement="top">
            <el-button size="small" text circle :loading="refreshing" @click="$emit('refresh')">
              <el-icon><Refresh /></el-icon>
            </el-button>
          </el-tooltip>
        </div>
      </div>
    </template>

    <div class="csc-body">
      <!-- Remote Address -->
      <div class="csc-addr-row">
        <div class="csc-addr-label">远端地址</div>
        <div class="csc-addr-value">
          <span v-if="remoteAddr" class="csc-addr-text">{{ remoteAddr }}:{{ remotePort }}</span>
          <span v-else class="csc-addr-placeholder">未配置</span>
          <el-tag v-if="connectionStatus === 'connected'" size="small" type="success" effect="dark" class="csc-addr-tag">
            <el-icon style="vertical-align: -2px; margin-right: 2px"><Link /></el-icon>
            已建立连接
          </el-tag>
          <el-tag v-else size="small" type="danger" effect="dark" class="csc-addr-tag">
            <el-icon style="vertical-align: -2px; margin-right: 2px"><Link /></el-icon>
            未连接
          </el-tag>
        </div>
      </div>

      <!-- Connection indicator bar -->
      <div class="csc-indicator-bar">
        <div class="csc-indicator-track">
          <div
            class="csc-indicator-fill"
            :class="connectionStatus"
            :style="{ width: connectionStatus === 'connected' ? '100%' : '0%' }"
          />
        </div>
      </div>

      <!-- Stats Grid -->
      <div class="csc-stats-grid">
        <div class="csc-stat-item">
          <div class="csc-stat-icon interrogations">
            <el-icon :size="16"><Search /></el-icon>
          </div>
          <div class="csc-stat-body">
            <span class="csc-stat-value">{{ stats?.interrogations ?? 0 }}</span>
            <span class="csc-stat-label">总召唤</span>
          </div>
        </div>
        <div class="csc-stat-item">
          <div class="csc-stat-icon controls">
            <el-icon :size="16"><SwitchButton /></el-icon>
          </div>
          <div class="csc-stat-body">
            <span class="csc-stat-value">{{ stats?.controls ?? 0 }}</span>
            <span class="csc-stat-label">遥控次数</span>
          </div>
        </div>
        <div class="csc-stat-item">
          <div class="csc-stat-icon spontaneous">
            <el-icon :size="16"><DataAnalysis /></el-icon>
          </div>
          <div class="csc-stat-body">
            <span class="csc-stat-value">{{ stats?.spontaneous ?? 0 }}</span>
            <span class="csc-stat-label">变化上送</span>
          </div>
        </div>
        <div class="csc-stat-item">
          <div class="csc-stat-icon uptime">
            <el-icon :size="16"><Clock /></el-icon>
          </div>
          <div class="csc-stat-body">
            <span class="csc-stat-value">{{ fmtDuration(stats?.uptime_seconds ?? 0) }}</span>
            <span class="csc-stat-label">运行时长</span>
          </div>
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { Refresh, Link, Search, SwitchButton, DataAnalysis, Clock } from '@element-plus/icons-vue'

defineProps<{
  connectionStatus: 'connected' | 'disconnected'
  remoteAddr?: string
  remotePort?: number
  stats: { interrogations: number; controls: number; spontaneous: number; uptime_seconds: number } | null
  refreshing?: boolean
}>()

defineEmits<{
  refresh: []
}>()

function fmtDuration(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return `${h}h${m}m`
  if (m > 0) return `${m}m${sec}s`
  return `${sec}s`
}
</script>

<style scoped>
.client-status-card {
  margin-bottom: 16px;
}

.csc-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.csc-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.csc-proto-tag {
  font-size: 11px;
}

.csc-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.csc-header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.csc-status-badge {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  padding: 0 10px;
  height: 24px;
}

.csc-status-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.csc-status-dot.connected {
  background: var(--color-success);
  box-shadow: 0 0 6px rgba(34, 197, 94, 0.6);
  animation: csc-pulse 2s ease-in-out infinite;
}

.csc-status-dot.disconnected {
  background: var(--color-danger);
  box-shadow: 0 0 6px rgba(239, 68, 68, 0.4);
}

@keyframes csc-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.4); }
}

/* ─── Remote Address Row ─── */
.csc-addr-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-color);
  margin-bottom: 8px;
}

.csc-addr-label {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
  flex-shrink: 0;
}

.csc-addr-value {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
}

.csc-addr-text {
  font-family: var(--font-mono, 'JetBrains Mono', 'SF Mono', monospace);
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: 0.3px;
}

.csc-addr-placeholder {
  font-size: 13px;
  color: var(--text-muted);
  font-style: italic;
}

.csc-addr-tag {
  font-size: 10px !important;
  padding: 0 6px !important;
  height: 20px !important;
  line-height: 20px !important;
}

/* ─── Connection Indicator Bar ─── */
.csc-indicator-bar {
  margin-bottom: 12px;
}

.csc-indicator-track {
  height: 3px;
  background: var(--border-color);
  border-radius: 2px;
  overflow: hidden;
}

.csc-indicator-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.5s cubic-bezier(0.22, 1, 0.36, 1);
}

.csc-indicator-fill.connected {
  background: linear-gradient(90deg, var(--color-success), #34d399);
}

.csc-indicator-fill.disconnected {
  background: var(--color-danger);
}

/* ─── Stats Grid ─── */
.csc-stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}

.csc-stat-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  background: var(--bg-input);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
}

.csc-stat-icon {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  flex-shrink: 0;
}

.csc-stat-icon.interrogations {
  background: rgba(59, 130, 246, 0.12);
  color: var(--accent);
}

.csc-stat-icon.controls {
  background: rgba(239, 68, 68, 0.12);
  color: var(--color-danger);
}

.csc-stat-icon.spontaneous {
  background: rgba(139, 92, 246, 0.12);
  color: #8b5cf6;
}

.csc-stat-icon.uptime {
  background: rgba(34, 197, 94, 0.12);
  color: var(--color-success);
}

.csc-stat-body {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.csc-stat-value {
  font-size: 15px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--text-primary);
  line-height: 1.2;
}

.csc-stat-label {
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
}

/* ─── Responsive ─── */
@media (max-width: 768px) {
  .csc-stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .csc-addr-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
}
</style>
