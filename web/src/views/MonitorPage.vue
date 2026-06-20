<template>
  <div>
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span style="font-size: 16px; font-weight: 600">运行监控</span>
        <div>
          <span style="color: #666; font-size: 13px; margin-right: 12px">
            上次刷新: {{ lastRefresh }}
          </span>
          <el-button @click="fetchData" :icon="Refresh" :loading="loading">刷新</el-button>
        </div>
      </div>
    </el-card>

    <div v-if="instances.length === 0 && !loading" style="text-align: center; padding: 60px; color: #999">
      暂无实例，请先在"配置管理"页面添加
    </div>

    <el-row :gutter="16" v-loading="loading">
      <el-col v-for="(inst, idx) in instances" :key="inst.id" :xs="24" :sm="12" :md="8" :lg="6"
        class="stagger-col" :style="{ marginBottom: '16px', animationDelay: idx * 0.06 + 's' }">
        <el-card shadow="never" class="monitor-card" @mousemove="onCardHover" @mouseleave="onCardLeave">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
            <span style="font-weight: 600; font-size: 15px">{{ inst.name }}</span>
            <el-tag v-if="inst.status === 'running'" type="success" size="small">运行中</el-tag>
            <el-tag v-else-if="inst.status === 'error'" type="danger" size="small">错误</el-tag>
            <el-tag v-else type="info" size="small">已停止</el-tag>
          </div>

          <template v-if="inst.stats">
            <el-descriptions :column="1" size="small" border style="margin-bottom: 12px">
              <el-descriptions-item label="IEC104端口">
                <el-tag size="small">{{ inst.iec104_port }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="HTTP端口" v-if="inst.http_enabled">
                <el-tag size="small" type="warning">{{ inst.http_port }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="客户端">
                <el-tag :type="inst.stats.client_connected ? 'success' : 'danger'" size="small">
                  {{ inst.stats.client_connected ? '已连接' : '未连接' }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="测点数">{{ inst.stats.total_points }}</el-descriptions-item>
              <el-descriptions-item label="运行时间">{{ fmtUptime(inst.stats.uptime_seconds) }}</el-descriptions-item>
              <el-descriptions-item label="总召次数">{{ inst.stats.interrogations }}</el-descriptions-item>
              <el-descriptions-item label="变化上送">{{ inst.stats.spontaneous }}</el-descriptions-item>
            </el-descriptions>
          </template>
          <template v-else>
            <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 12px">
              <template #title>实例未运行</template>
            </el-alert>
          </template>

          <div style="display: flex; gap: 8px">
            <el-button
              v-if="inst.status === 'running'"
              type="primary"
              size="small"
              @click="openInstance(inst)"
            >
              {{ inst.protocol === 'microgrid' ? '微电网' : '详情' }}
            </el-button>
            <el-button
              v-if="inst.status === 'running'"
              type="warning"
              size="small"
              style="flex: 1"
              :loading="actionLoading === inst.id"
              @click="handleRestart(inst.id)"
            >
              重启
            </el-button>
            <el-button
              v-else
              type="success"
              size="small"
              style="flex: 1"
              :loading="actionLoading === inst.id"
              @click="handleStart(inst.id)"
            >
              启动
            </el-button>
            <el-button
              v-if="inst.status === 'running'"
              size="small"
              :loading="actionLoading === inst.id"
              @click="handleStop(inst.id)"
            >
              停止
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>


  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listInstances,
  startInstance,
  stopInstance,
  restartInstance,
  type InstanceState,
} from '../api'
const loading = ref(false)
const instances = ref<InstanceState[]>([])
const lastRefresh = ref('')
const actionLoading = ref('')
const router = useRouter()
let timer: ReturnType<typeof setInterval> | null = null

function openInstance(inst: InstanceState) {
  if (inst.protocol === 'microgrid') {
    router.push('/microgrid/' + inst.id)
  } else {
    router.push('/detail/' + inst.id)
  }
}

async function fetchData() {
  loading.value = true
  try {
    instances.value = await listInstances()
    lastRefresh.value = new Date().toLocaleTimeString()
  } catch (e: any) {
    // Silent fail on auto-refresh
  } finally {
    loading.value = false
  }
}

async function handleStart(id: string) {
  actionLoading.value = id
  try {
    await startInstance(id)
    ElMessage.success('已启动')
    await fetchData()
  } catch (e: any) {
    ElMessage.error('启动失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    actionLoading.value = ''
  }
}

async function handleStop(id: string) {
  actionLoading.value = id
  try {
    await stopInstance(id)
    ElMessage.success('已停止')
    await fetchData()
  } catch (e: any) {
    ElMessage.error('停止失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    actionLoading.value = ''
  }
}

async function handleRestart(id: string) {
  try {
    await ElMessageBox.confirm('确定重启此实例？实例重启期间绑定的 IEC104 客户端将断开。', '确认重启')
  } catch {
    return
  }
  actionLoading.value = id
  try {
    await restartInstance(id)
    ElMessage.success('已重启')
    await fetchData()
  } catch (e: any) {
    ElMessage.error('重启失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    actionLoading.value = ''
  }
}

function fmtUptime(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return `${h}h${m}m${sec}s`
  if (m > 0) return `${m}m${sec}s`
  return `${sec}s`
}

// P9: SpotlightCard — mouse-tracking glow
function onCardHover(e: MouseEvent) {
  const el = e.currentTarget as HTMLElement
  const rect = el.getBoundingClientRect()
  el.style.setProperty('--mx', `${e.clientX - rect.left}px`)
  el.style.setProperty('--my', `${e.clientY - rect.top}px`)
}

function onCardLeave(e: MouseEvent) {
  const el = e.currentTarget as HTMLElement
  el.style.setProperty('--mx', '-200px')
  el.style.setProperty('--my', '-200px')
}

onMounted(() => {
  fetchData()
  timer = setInterval(fetchData, 5000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
/* P8: AnimatedList stagger — card grid */
@keyframes card-stagger {
  from { opacity: 0; transform: translateY(12px) scale(0.97); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

:deep(.stagger-col) {
  animation: card-stagger 0.4s cubic-bezier(0.22, 1, 0.36, 1) both;
}

/* P9: SpotlightCard — hover glow */
:deep(.monitor-card) {
  position: relative;
  overflow: hidden;
  --mx: -200px;
  --my: -200px;
  transition: border-color 0.3s, box-shadow 0.3s;
}

:deep(.monitor-card::before) {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(circle 200px at var(--mx) var(--my), rgba(245, 158, 11, 0.08), transparent 70%);
  pointer-events: none;
  z-index: 0;
  opacity: 0;
  transition: opacity 0.3s;
}

:deep(.monitor-card:hover::before) {
  opacity: 1;
}

:deep(.monitor-card:hover) {
  border-color: rgba(245, 158, 11, 0.25) !important;
  box-shadow: 0 0 20px rgba(245, 158, 11, 0.06);
}

:deep(.monitor-card .el-card__body) {
  position: relative;
  z-index: 1;
}
</style>
