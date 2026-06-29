<template>
  <el-card v-if="visible" class="command-panel-card">
    <template #header>
      <div class="cp-header">
        <div class="cp-header-left">
          <span>控制命令</span>
          <el-tag type="danger" size="small">DO / AO</el-tag>
          <span class="cp-point-count">{{ commandablePoints.length }} 个可控测点</span>
        </div>
        <div class="cp-header-right">
          <el-button
            v-if="commandLog.length > 0"
            size="small"
            text
            @click="showLog = !showLog"
          >
            <el-icon style="margin-right: 4px"><List /></el-icon>
            命令日志 ({{ commandLog.length }})
            <el-icon :style="{ transform: showLog ? 'rotate(180deg)' : '', transition: 'transform .2s', marginLeft: '4px' }">
              <ArrowDown />
            </el-icon>
          </el-button>
        </div>
      </div>
    </template>

    <!-- Empty state -->
    <el-alert
      v-if="commandablePoints.length === 0"
      title="当前实例没有可控制的 DO/AO 测点"
      type="info"
      show-icon
      :closable="false"
    />

    <template v-else>
      <!-- Command Table -->
      <el-table :data="commandablePoints" size="small" max-height="400" class="cp-table">
        <el-table-column prop="ioa" label="IOA" width="72" />
        <el-table-column prop="name" label="名称" min-width="130" />
        <el-table-column prop="point_type" label="类型" width="64">
          <template #default="{ row }">
            <el-tag :type="row.point_type === 'DO' ? 'danger' : 'warning'" size="small" effect="plain" class="cp-type-tag">
              {{ row.point_type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="当前值" width="80">
          <template #default="{ row }">
            <span class="cp-current-value" :class="{ 'text-success': row.point_type === 'DO' && row.bool_value }">
              {{ row.point_type === 'DO' ? (row.bool_value ? 'ON' : 'OFF') : row.value?.toFixed(2) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="260">
          <template #default="{ row }">
            <template v-if="row.point_type === 'DO'">
              <div class="cp-action-row">
                <el-switch
                  :model-value="row.bool_value"
                  active-text="ON"
                  inactive-text="OFF"
                  active-color="#ef4444"
                  @change="(val: boolean) => confirmDOSend(row, val)"
                />
                <el-button
                  v-if="row.bool_value !== undefined"
                  size="small"
                  text
                  class="cp-inverse-btn"
                  :type="row.bool_value ? 'default' : 'danger'"
                  @click="confirmDOSend(row, !row.bool_value)"
                >
                  {{ row.bool_value ? '→ OFF' : '→ ON' }}
                </el-button>
              </div>
            </template>
            <template v-else-if="row.point_type === 'AO'">
              <div class="cp-action-row">
                <el-input-number
                  v-model="aoValues[row.ioa]"
                  :step="0.1"
                  size="small"
                  style="width: 130px"
                  :min="undefined"
                  :max="undefined"
                  :controls="false"
                  placeholder="输入数值"
                />
                <el-button
                  size="small"
                  type="primary"
                  :loading="sendingIoas.has(row.ioa)"
                  :disabled="sendingIoas.has(row.ioa)"
                  @click="sendAOCommand(row)"
                >
                  {{ sendingIoas.has(row.ioa) ? '发送中' : '发送' }}
                </el-button>
              </div>
            </template>
          </template>
        </el-table-column>
      </el-table>

      <!-- Recent Command Log -->
      <transition name="slide-fade">
        <div v-if="showLog && commandLog.length > 0" class="cp-log-section">
          <div class="cp-log-header">
            <span class="cp-log-title">最近命令</span>
            <el-button size="small" text type="danger" @click="clearLog">清空</el-button>
          </div>
          <div class="cp-log-list">
            <div v-for="(entry, idx) in commandLog.slice().reverse()" :key="idx" class="cp-log-entry" :class="entry.status">
              <span class="cp-log-time">{{ entry.time }}</span>
              <el-tag
                :type="entry.pointType === 'DO' ? 'danger' : 'warning'"
                size="small"
                effect="plain"
                class="cp-log-type"
              >{{ entry.pointType }}</el-tag>
              <span class="cp-log-point">{{ entry.name }} (IOA: {{ entry.ioa }})</span>
              <span class="cp-log-arrow">→</span>
              <span class="cp-log-value">{{ entry.value }}</span>
              <el-tag
                :type="entry.status === 'success' ? 'success' : 'danger'"
                size="small"
                class="cp-log-status"
              >
                {{ entry.status === 'success' ? '成功' : '失败' }}
              </el-tag>
            </div>
          </div>
        </div>
      </transition>
    </template>
  </el-card>

  <!-- DO Confirmation Dialog -->
  <el-dialog
    v-model="confirmVisible"
    title="确认发送控制命令"
    width="420px"
    :close-on-click-modal="false"
    append-to-body
  >
    <div class="confirm-body">
      <el-alert type="warning" :closable="false" show-icon>
        <template #title>
          即将发送 DO 遥控命令
        </template>
      </el-alert>
      <div class="confirm-detail" v-if="confirmTarget">
        <div class="confirm-row">
          <span class="confirm-label">测点</span>
          <span class="confirm-value">{{ confirmTarget.name }}</span>
        </div>
        <div class="confirm-row">
          <span class="confirm-label">IOA</span>
          <span class="confirm-value mono">{{ confirmTarget.ioa }}</span>
        </div>
        <div class="confirm-row">
          <span class="confirm-label">目标值</span>
          <el-tag
            :type="confirmValue ? 'danger' : 'info'"
            size="small"
            effect="dark"
          >{{ confirmValue ? 'ON' : 'OFF' }}</el-tag>
        </div>
        <div class="confirm-row">
          <span class="confirm-label">当前值</span>
          <el-tag
            :type="confirmTarget.bool_value ? 'danger' : 'info'"
            size="small"
            effect="plain"
          >{{ confirmTarget.bool_value ? 'ON' : 'OFF' }}</el-tag>
        </div>
      </div>
      <div class="confirm-hint">发送后测点值将立即更新，请确认操作正确。</div>
    </div>
    <template #footer>
      <el-button @click="confirmVisible = false">取消</el-button>
      <el-button type="danger" :loading="sendingConfirm" @click="executeDOSend">
        {{ sendingConfirm ? '发送中' : '确认发送' }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowDown, List } from '@element-plus/icons-vue'
import { sendCommand } from '../../api'
import type { PointSnapshot } from '../../api'

const props = defineProps<{
  points: PointSnapshot[]
  instanceId: string
  visible: boolean
}>()

const emit = defineEmits<{
  commandSent: [ioa: number]
}>()

// ── Computed ──
const commandablePoints = computed(() =>
  props.points.filter((p) => p.point_type === 'DO' || p.point_type === 'AO')
)

// ── AO Input Values (local state, not coupled to point values) ──
const aoValues = reactive<Record<number, number>>({})

// Initialize AO values from current point values
commandablePoints.value.forEach((p) => {
  if (p.point_type === 'AO' && aoValues[p.ioa] === undefined) {
    aoValues[p.ioa] = p.value
  }
})

// ── Loading state per IOA ──
const sendingIoas = reactive<Set<number>>(new Set())

// ── DO Confirmation Dialog ──
const confirmVisible = ref(false)
const confirmTarget = ref<PointSnapshot | null>(null)
const confirmValue = ref(false)
const sendingConfirm = ref(false)

// ── Command Log ──
const showLog = ref(false)
interface CommandLogEntry {
  time: string
  ioa: number
  name: string
  pointType: string
  value: string
  status: 'success' | 'error'
}
const commandLog = ref<CommandLogEntry[]>([])

function addLogEntry(entry: CommandLogEntry) {
  commandLog.value.push(entry)
  // Keep last 20 entries
  if (commandLog.value.length > 20) {
    commandLog.value = commandLog.value.slice(-20)
  }
}

function clearLog() {
  commandLog.value = []
}

function fmtTime(): string {
  const d = new Date()
  return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

// ── DO Command Flow ──
function confirmDOSend(point: PointSnapshot, newVal: boolean) {
  confirmTarget.value = point
  confirmValue.value = newVal
  confirmVisible.value = true
}

async function executeDOSend() {
  if (!confirmTarget.value) return
  const point = confirmTarget.value
  const val = confirmValue.value

  sendingConfirm.value = true
  try {
    await sendCommand(props.instanceId, { ioa: point.ioa, bool_value: val })
    ElMessage.success(`DO 命令已发送 — ${point.name} → ${val ? 'ON' : 'OFF'}`)
    addLogEntry({
      time: fmtTime(),
      ioa: point.ioa,
      name: point.name,
      pointType: 'DO',
      value: val ? 'ON' : 'OFF',
      status: 'success',
    })
    confirmVisible.value = false
    emit('commandSent', point.ioa)
  } catch (e: any) {
    ElMessage.error('DO 命令发送失败: ' + (e?.response?.data?.error || e.message))
    addLogEntry({
      time: fmtTime(),
      ioa: point.ioa,
      name: point.name,
      pointType: 'DO',
      value: val ? 'ON' : 'OFF',
      status: 'error',
    })
  } finally {
    sendingConfirm.value = false
  }
}

// ── AO Command Flow ──
async function sendAOCommand(point: PointSnapshot) {
  const val = aoValues[point.ioa]
  if (val === undefined || val === null) {
    ElMessage.warning('请先输入要发送的数值')
    return
  }

  sendingIoas.add(point.ioa)
  try {
    await sendCommand(props.instanceId, { ioa: point.ioa, value: val })
    ElMessage.success(`AO 命令已发送 — ${point.name} → ${val.toFixed(2)}`)
    addLogEntry({
      time: fmtTime(),
      ioa: point.ioa,
      name: point.name,
      pointType: 'AO',
      value: val.toFixed(2),
      status: 'success',
    })
    emit('commandSent', point.ioa)
  } catch (e: any) {
    ElMessage.error('AO 命令发送失败: ' + (e?.response?.data?.error || e.message))
    addLogEntry({
      time: fmtTime(),
      ioa: point.ioa,
      name: point.name,
      pointType: 'AO',
      value: val.toFixed(2),
      status: 'error',
    })
  } finally {
    sendingIoas.delete(point.ioa)
  }
}
</script>

<style scoped>
.command-panel-card {
  margin-bottom: 16px;
}

.cp-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.cp-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cp-point-count {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 400;
}

.cp-header-right {
  display: flex;
  align-items: center;
  gap: 4px;
}

/* ─── Table ─── */
.cp-table {
  --cp-cell-padding: 6px 8px;
}

.cp-table :deep(.el-table__cell) {
  padding: var(--cp-cell-padding);
}

.cp-type-tag {
  font-size: 10px !important;
  padding: 0 4px !important;
}

.cp-current-value {
  font-weight: 600;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.cp-current-value.text-success {
  color: var(--color-success);
}

/* ─── Action Row ─── */
.cp-action-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.cp-inverse-btn {
  font-size: 11px !important;
  padding: 0 6px !important;
}

/* ─── Command Log ─── */
.cp-log-section {
  margin-top: 12px;
  border-top: 1px solid var(--border-color);
  padding-top: 10px;
}

.cp-log-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.cp-log-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.cp-log-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
  max-height: 200px;
  overflow-y: auto;
}

.cp-log-entry {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  background: var(--bg-input);
  border: 1px solid var(--border-color);
}

.cp-log-entry.success {
  border-left: 3px solid var(--color-success);
}

.cp-log-entry.error {
  border-left: 3px solid var(--color-danger);
}

.cp-log-time {
  font-family: var(--font-mono, 'JetBrains Mono', monospace);
  font-size: 11px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.cp-log-type {
  font-size: 9px !important;
  padding: 0 4px !important;
  height: 18px !important;
  line-height: 18px !important;
  flex-shrink: 0;
}

.cp-log-point {
  color: var(--text-primary);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cp-log-arrow {
  color: var(--text-muted);
  flex-shrink: 0;
}

.cp-log-value {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--text-primary);
  flex-shrink: 0;
}

.cp-log-status {
  font-size: 10px !important;
  padding: 0 6px !important;
  height: 18px !important;
  line-height: 18px !important;
  margin-left: auto;
  flex-shrink: 0;
}

/* ─── Confirmation Dialog ─── */
.confirm-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.confirm-detail {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  background: var(--bg-input);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
}

.confirm-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.confirm-label {
  font-size: 13px;
  color: var(--text-muted);
  width: 60px;
  flex-shrink: 0;
}

.confirm-value {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.confirm-value.mono {
  font-family: var(--font-mono, 'JetBrains Mono', monospace);
}

.confirm-hint {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

/* ─── Transition ─── */
.slide-fade-enter-active {
  transition: all 0.25s cubic-bezier(0.22, 1, 0.36, 1);
}

.slide-fade-leave-active {
  transition: all 0.2s ease;
}

.slide-fade-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}

.slide-fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
