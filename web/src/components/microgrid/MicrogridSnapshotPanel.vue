<template>
  <div class="snapshot-panel">
    <div class="snapshot-header">
      <span class="snapshot-title">场景快照</span>
      <el-button text size="small" @click="showInput = !showInput" :icon="showInput ? 'ArrowUp' : 'ArrowDown'" style="color:#94a3b8">
        {{ showInput ? '收起' : '展开' }}
      </el-button>
    </div>

    <div v-if="showInput" class="snapshot-input">
      <el-input v-model="newName" placeholder="快照名称（如: 保存前A）" size="small" @keyup.enter="saveSnapshot">
        <template #append>
          <el-button @click="saveSnapshot" :disabled="!newName.trim()">保存</el-button>
        </template>
      </el-input>
    </div>

    <div v-if="snapshots.length === 0 && !showInput" class="snapshot-empty">
      暂无快照，点击上方按钮保存
    </div>

    <div v-else class="snapshot-list">
      <div v-for="snap in snapshots" :key="snap.id" class="snapshot-item" :class="{ active: activeId === snap.id }">
        <div class="snapshot-info">
          <span class="snapshot-name">{{ snap.name }}</span>
          <span class="snapshot-time">{{ snap.timestamp }}</span>
        </div>
        <div class="snapshot-actions">
          <el-button size="small" text type="primary" @click="loadSnapshot(snap)" :disabled="!running">
            加载
          </el-button>
          <el-button size="small" text type="danger" @click="deleteSnapshot(snap.id)">
            删除
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessageBox, ElMessage } from 'element-plus'

interface SnapshotEntry {
  id: string
  name: string
  timestamp: string
  bus_name: string
  bus_voltage_kv: number
  grid_meter: { rated_capacity_kw: number; island_mode: boolean }
  devices: Array<{
    id: string
    type: string
    name: string
    switch: { closed: boolean }
    params: Record<string, unknown>
    ioa_base?: number
    control_mode?: string
    custom_points?: Array<{ name: string; type: string; alias?: string }>
  }>
}

const props = defineProps<{
  snapshots: SnapshotEntry[]
  activeId?: string
  running: boolean
}>()

const emit = defineEmits<{
  save: [payload: { name: string; bus_name: string; bus_voltage_kv: number; grid_meter: any; devices: any[] }]
  load: [payload: SnapshotEntry]
  delete: [id: string]
}>()

const showInput = ref(false)
const newName = ref('')

function formatDate(ts: number): string {
  const d = new Date(ts)
  return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function saveSnapshot() {
  const name = newName.value.trim()
  if (!name) return
  emit('save', {
    name,
    bus_name: '',
    bus_voltage_kv: 0,
    grid_meter: {},
    devices: [],
  })
  newName.value = ''
  showInput.value = false
  ElMessage.success('快照已保存')
}

function loadSnapshot(snap: SnapshotEntry) {
  emit('load', snap)
  ElMessage.success(`已加载快照 "${snap.name}"`)
}

async function deleteSnapshot(id: string) {
  try {
    await ElMessageBox.confirm('确定删除此快照？', '确认')
    emit('delete', id)
    ElMessage.success('快照已删除')
  } catch {}
}

const localSnapshots = ref<SnapshotEntry[]>([])
</script>

<style scoped>
.snapshot-panel {
  background: var(--bg-card, #141b2d);
  border: 1px solid var(--border-color, #2d3a4e);
  border-radius: 8px;
  overflow: hidden;
}

.snapshot-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-color, #2d3a4e);
  user-select: none;
}

.snapshot-title {
  font-weight: 600;
  font-size: 13px;
  color: var(--text-primary);
}

.snapshot-input {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-color, #2d3a4e);
}

.snapshot-empty {
  padding: 24px;
  text-align: center;
  color: var(--text-muted, #64748b);
  font-size: 12px;
}

.snapshot-list {
  max-height: 280px;
  overflow-y: auto;
  padding: 8px;
}

.snapshot-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: 6px;
  transition: background 0.15s;
}

.snapshot-item:hover {
  background: var(--sidebar-hover, rgba(255, 255, 255, 0.04));
}

.snapshot-item.active {
  background: var(--sidebar-active, rgba(59, 130, 246, 0.08));
  border-left: 2px solid var(--accent, #3b82f6);
}

.snapshot-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.snapshot-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.snapshot-time {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.snapshot-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
</style>
