<template>
  <div>
    <el-card shadow="never" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px">
        <div style="display: flex; align-items: center; gap: 12px">
          <span style="font-size: 16px; font-weight: 600">实例配置</span>
          <span v-if="globalStatus" style="font-size: 13px; color: var(--el-text-color-secondary)">
            已配置 {{ globalStatus.configured }} / {{ globalStatus.max }} |
            运行中 <span style="color: #67c23a">{{ globalStatus.running }}</span> |
            已停止 <span style="color: #909399">{{ globalStatus.stopped }}</span>
          </span>
        </div>
        <div>
          <el-button @click="fetchData" :icon="Refresh" circle aria-label="刷新数据" />
          <el-button type="primary" @click="showAddDialog = true">添加实例</el-button>
        </div>
      </div>
    </el-card>

    <!-- Batch Action Bar -->
    <transition name="slide-fade">
      <el-card v-if="selectedIds.length > 0" shadow="never" style="margin-bottom: 12px; padding: 8px 12px">
        <div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap">
          <el-checkbox v-model="allSelected" :indeterminate="isIndeterminate" @change="toggleSelectAll" style="margin-right: 4px" />
          <span style="font-size: 13px; color: var(--text-secondary)">已选 {{ selectedIds.length }} / {{ filteredInstances.length }}</span>
          <div style="width: 1px; height: 20px; background: var(--border-color)" />
          <el-button size="small" type="success" :loading="batchLoading" @click="batchStart" :disabled="selectedIds.length === 0">批量启动</el-button>
          <el-button size="small" type="warning" :loading="batchLoading" @click="batchStop" :disabled="selectedIds.length === 0">批量停止</el-button>
          <el-button size="small" type="danger" :loading="batchLoading" @click="batchDelete" :disabled="selectedIds.length === 0">批量删除</el-button>
          <el-button size="small" text @click="clearSelection">取消选择</el-button>
        </div>
      </el-card>
    </transition>

    <el-card shadow="never" v-loading="loading">
      <el-empty v-if="instances.length === 0 && !loading" description="暂无实例，点击上方按钮添加" />
      <el-table
        ref="tableRef" v-else :data="filteredInstances" stripe style="width: 100%"
        @selection-change="onSelectionChange"
      >
        <el-table-column type="selection" width="40" />
        <el-table-column prop="id" label="ID" width="90" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column label="规约" width="120">
          <template #default="{ row }">
            <el-tag :type="protoTagType(row.protocol)">{{ protoLabel(row.protocol) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="端口" width="100">
          <template #default="{ row }">
            <el-tag>{{ displayPort(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="xlsx_file" label="点表文件" min-width="160" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.status === 'running'" type="success">运行中</el-tag>
            <el-tag v-else-if="row.status === 'error'" type="danger">错误</el-tag>
            <el-tag v-else type="info">已停止</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="统计" min-width="200">
          <template #default="{ row }">
            <span v-if="row.stats" style="font-size: 12px; color: var(--el-text-color-secondary)">
              <el-icon :color="row.stats.client_connected ? '#67c23a' : '#f56c6c'" style="vertical-align: middle; margin-right: 2px">
                <component :is="row.stats.client_connected ? SuccessFilled : CircleCloseFilled" />
              </el-icon>
              {{ row.stats.client_connected ? '在线' : '离线' }} |
              测点: {{ row.stats.total_points }} |
              运行: {{ fmtUptime(row.stats.uptime_seconds) }}
            </span>
            <span v-else style="color: var(--el-text-color-placeholder)">-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="400" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button v-if="row.status === 'running'" type="warning" size="small"
                :loading="actionLoading === row.id" @click="handleStop(row.id)">停止</el-button>
              <el-button v-else type="success" size="small"
                :loading="actionLoading === row.id" @click="handleStart(row.id)">启动</el-button>
              <el-button v-if="row.protocol === 'microgrid'" size="small" type="primary"
                @click="openMicrogrid(row.id)">微电网</el-button>
              <template v-else>
                <el-button size="small" @click="router.push('/detail/' + row.id)">详情</el-button>
                <el-button size="small" :disabled="row.status === 'running'" @click="openPointTableEditor(row.id, row.protocol)">编辑点表</el-button>
              </template>
              <el-button size="small" :disabled="row.status === 'running' || actionLoading === row.id" @click="handleEdit(row)">编辑</el-button>
              <el-button type="danger" size="small" :disabled="row.status === 'running' || actionLoading === row.id" @click="handleDelete(row.id)">删除</el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Add/Edit Wizard Dialog -->
    <el-dialog v-model="showAddDialog" :title="editing ? '编辑实例' : '新建实例'" width="620px" :close-on-click-modal="false">
      <!-- Step Indicator -->
      <el-steps :active="wizardStep" finish-status="success" simple style="margin-bottom: 20px" v-if="!editing">
        <el-step title="规约与名称" />
        <el-step title="网络配置" />
        <el-step title="点表与接口" />
        <el-step title="确认" />
      </el-steps>

      <el-form :model="form" label-width="120px" :rules="rules" ref="formRef" v-if="!editing">
        <!-- Step 1: Protocol & Name -->
        <div v-show="wizardStep === 0">
          <el-form-item label="规约类型" prop="protocol">
            <el-radio-group v-model="form.protocol">
              <el-radio-button value="iec104">IEC104</el-radio-button>
              <el-radio-button value="modbus_tcp">Modbus TCP</el-radio-button>
              <el-radio-button value="microgrid">微电网</el-radio-button>
            </el-radio-group>
            <div style="font-size:12px;color:#64748b;margin-top:6px;line-height:1.4">
              <template v-if="form.protocol === 'iec104'">电力行业标准规约，适用于变电站自动化系统联调测试</template>
              <template v-else-if="form.protocol === 'modbus_tcp'">工业自动化领域通用规约，适用于Modbus TCP设备仿真</template>
              <template v-else>微电网仿真场景，包含光伏、储能、负荷等设备的一体化仿真</template>
            </div>
          </el-form-item>
          <el-form-item label="实例名称" prop="name">
            <el-input v-model="form.name" placeholder="例如: 变电站A" maxlength="50" show-word-limit />
          </el-form-item>
        </div>

        <!-- Step 2: Network -->
        <div v-show="wizardStep === 1">
          <el-form-item :label="form.protocol === 'modbus_tcp' ? 'Modbus端口' : 'IEC104端口'" prop="iec104_port">
            <el-input-number v-model="form.iec104_port" :min="1" :max="65535" style="width: 100%" />
          </el-form-item>
          <el-form-item v-if="form.protocol === 'modbus_tcp'" label="从站地址">
            <el-input-number v-model="modbusSlaveId" :min="1" :max="247" style="width: 100%" />
          </el-form-item>
          <el-form-item v-if="form.protocol === 'modbus_tcp'" label="字节序">
            <el-select v-model="modbusByteOrder" style="width: 100%">
              <el-option label="ABCD (Big-Endian)" value="ABCD" />
              <el-option label="CDAB (Little-Endian)" value="CDAB" />
              <el-option label="BADC (Byte-Swapped)" value="BADC" />
              <el-option label="DCBA (Word-Swapped)" value="DCBA" />
            </el-select>
          </el-form-item>
        </div>

        <!-- Step 3: XLSX & HTTP -->
        <div v-show="wizardStep === 2">
          <el-form-item label="点表文件" prop="xlsx_file" v-if="form.protocol !== 'microgrid'">
            <el-select v-model="form.xlsx_file" placeholder="选择或上传文件" style="width: 100%" allow-create filterable @change="onFileSelected">
              <el-option v-for="f in availableFiles" :key="f.name" :label="f.name" :value="f.name" />
            </el-select>
            <div class="form-hint" style="font-size:12px;color:#64748b;margin-top:4px">{{ xlsxHint }}</div>
            <!-- File preview info -->
            <div v-if="selectedFileInfo" class="file-preview">
              <div class="fp-row"><span class="fp-label">大小</span><span>{{ selectedFileInfo.sizeStr }}</span></div>
              <div class="fp-row"><span class="fp-label">修改时间</span><span>{{ selectedFileInfo.modtime }}</span></div>
            </div>
          </el-form-item>
          <el-form-item v-if="form.protocol !== 'microgrid'" label="上传新文件">
            <el-upload :auto-upload="false" :show-file-list="false" accept=".xlsx" :on-change="handleFileChange">
              <el-button type="primary" :loading="uploading" size="small">{{ uploading ? '上传中...' : '选择 Excel 文件' }}</el-button>
            </el-upload>
          </el-form-item>
          <el-form-item v-if="form.protocol === 'microgrid'" label="微电网拓扑">
            <div style="color: var(--el-text-color-secondary); font-size: 13px;">
              微电网实例在创建后，需要进入微电网编辑器配置拓扑结构和设备参数。
            </div>
          </el-form-item>
          <el-form-item label="HTTP接口">
            <el-switch v-model="form.http_enabled" active-text="启用HTTP修改测点值" />
          </el-form-item>
          <el-form-item label="HTTP端口" v-if="form.http_enabled" prop="http_port">
            <el-input-number v-model="form.http_port" :min="1024" :max="65535" style="width: 100%" />
          </el-form-item>
        </div>

        <!-- Step 4: Confirm -->
        <div v-show="wizardStep === 3">
          <div class="wizard-review">
            <div class="review-section">
              <div class="review-label">规约类型</div>
              <div class="review-value"><el-tag :type="protoTagType(form.protocol)" size="small">{{ protoLabel(form.protocol) }}</el-tag></div>
            </div>
            <div class="review-section">
              <div class="review-label">实例名称</div>
              <div class="review-value">{{ form.name }}</div>
            </div>
            <div class="review-section">
              <div class="review-label">{{ form.protocol === 'modbus_tcp' ? 'Modbus端口' : 'IEC104端口' }}</div>
              <div class="review-value">{{ form.iec104_port }}</div>
            </div>
            <div class="review-section" v-if="form.protocol === 'modbus_tcp'">
              <div class="review-label">从站地址</div>
              <div class="review-value">{{ modbusSlaveId }}</div>
            </div>
            <div class="review-section" v-if="form.protocol === 'modbus_tcp'">
              <div class="review-label">字节序</div>
              <div class="review-value">{{ modbusByteOrder }}</div>
            </div>
            <div class="review-section" v-if="form.protocol !== 'microgrid'">
              <div class="review-label">点表文件</div>
              <div class="review-value">{{ form.xlsx_file || '未选择' }}</div>
            </div>
            <div class="review-section">
              <div class="review-label">HTTP接口</div>
              <div class="review-value">{{ form.http_enabled ? `已启用 (端口 ${form.http_port})` : '未启用' }}</div>
            </div>
          </div>
          <div class="wizard-start-option">
            <el-switch v-model="startAfterCreate" active-text="创建后立即启动" />
            <div class="start-hint" v-if="!startAfterCreate">实例创建后为停止状态，需手动启动</div>
            <div class="start-hint" v-else>实例创建后将自动启动 IEC104 服务</div>
          </div>
        </div>
      </el-form>

      <!-- Edit mode: show simple form (no steps) -->
      <el-form :model="form" label-width="120px" :rules="rules" ref="formEditRef" v-if="editing">
        <el-form-item label="规约类型" prop="protocol">
          <el-tag :type="protoTagType(form.protocol)" size="small">{{ protoLabel(form.protocol) }}</el-tag>
        </el-form-item>
        <el-form-item label="实例名称" prop="name">
          <el-input v-model="form.name" placeholder="例如: 变电站A" />
        </el-form-item>
        <el-form-item :label="form.protocol === 'modbus_tcp' ? 'Modbus端口' : 'IEC104端口'" prop="iec104_port">
          <el-input-number v-model="form.iec104_port" :min="1" :max="65535" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="form.protocol === 'modbus_tcp'" label="从站地址">
          <el-input-number v-model="modbusSlaveId" :min="1" :max="247" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="form.protocol === 'modbus_tcp'" label="字节序">
          <el-select v-model="modbusByteOrder" style="width: 100%">
            <el-option label="ABCD (Big-Endian)" value="ABCD" />
            <el-option label="CDAB (Little-Endian)" value="CDAB" />
            <el-option label="BADC (Byte-Swapped)" value="BADC" />
            <el-option label="DCBA (Word-Swapped)" value="DCBA" />
          </el-select>
        </el-form-item>
        <el-form-item label="HTTP接口">
          <el-switch v-model="form.http_enabled" active-text="启用HTTP修改测点值" />
        </el-form-item>
        <el-form-item label="HTTP端口" v-if="form.http_enabled" prop="http_port">
          <el-input-number v-model="form.http_port" :min="1024" :max="65535" style="width: 100%" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="wizardStep > 0 && !editing ? wizardStep-- : (showAddDialog = false)">
          {{ wizardStep > 0 && !editing ? '上一步' : '取消' }}
        </el-button>
        <span v-if="wizardStep < 3 && !editing">
          <el-button type="primary" @click="wizardNext">下一步</el-button>
        </span>
        <span v-else>
          <el-button type="primary" @click="handleSave" :loading="saving">{{ editing ? '保存' : '创建' }}</el-button>
        </span>
      </template>
    </el-dialog>
    <PointTableEditor
      v-model:visible="pointEditorVisible"
      :instance-id="pointEditorInstanceId"
      :protocol="pointEditorProtocol"
      @saved="fetchData"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Refresh, SuccessFilled, CircleCloseFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import PointTableEditor from '../components/PointTableEditor.vue'
import {
  listInstances,
  createInstance,
  updateInstance,
  deleteInstance,
  startInstance,
  stopInstance,
  uploadExcel,
  listFiles,
  getStatus,
  type InstanceConfig,
  type InstanceState,
  type GlobalStatus,
} from '../api'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const instances = ref<InstanceState[]>([])
const globalStatus = ref<GlobalStatus | null>(null)
const showAddDialog = ref(false)
const editing = ref(false)
const saving = ref(false)
const formRef = ref<FormInstance>()
const formEditRef = ref<FormInstance>()
const availableFiles = ref<{ name: string; size: number; modtime: string }[]>([])
const selectedFile = ref<File | null>(null)
const uploading = ref(false)
const pointEditorVisible = ref(false)
const pointEditorInstanceId = ref('')
const pointEditorProtocol = ref('')
const actionLoading = ref('')
const selectedIds = ref<string[]>([])
const batchLoading = ref(false)
const tableRef = ref<any>(null)
const wizardStep = ref(0)

const allSelected = computed({
  get: () => selectedIds.value.length === filteredInstances.value.length && filteredInstances.value.length > 0,
  set: (val: boolean) => {
    if (val) {
      selectedIds.value = filteredInstances.value.map(i => i.id)
    } else {
      selectedIds.value = []
    }
  },
})
const isIndeterminate = computed(() => {
  const len = selectedIds.value.length
  return len > 0 && len < filteredInstances.value.length
})

function toggleSelectAll(val: boolean) {
  allSelected.value = val
}

function clearSelection() {
  selectedIds.value = []
  tableRef.value?.clearSelection()
}
const startAfterCreate = ref(true)

const selectedFileInfo = computed(() => {
  const name = form.value.xlsx_file
  if (!name) return null
  const file = availableFiles.value.find(f => f.name === name)
  if (!file) return null
  const sizeStr = file.size > 1024 * 1024
    ? (file.size / 1024 / 1024).toFixed(1) + ' MB'
    : (file.size / 1024).toFixed(1) + ' KB'
  return { sizeStr, modtime: file.modtime }
})

function onFileSelected(val: string) {
  // Trigger computed update for file preview
  form.value.xlsx_file = val
}

const form = ref<InstanceConfig>({
  name: '',
  iec104_port: 2404,
  xlsx_file: '',
  http_enabled: false,
  http_port: 8081,
  protocol: 'iec104',
})

const modbusSlaveId = ref(1)
const modbusByteOrder = ref('ABCD')

const rules = {
  name: [{ required: true, message: '请输入实例名称', trigger: 'blur' }],
  iec104_port: [{ required: true, message: '请填写端口号', trigger: 'blur' }],
  xlsx_file: [
    {
      required: true,
      message: '请选择点表文件',
      trigger: 'change',
      validator: (_rule: any, value: string, callback: any) => {
        if (form.value.protocol === 'microgrid') {
          callback()
        } else if (!value) {
          callback(new Error('请选择点表文件'))
        } else {
          callback()
        }
      },
    },
  ],
}

const searchQuery = computed(() => (route.query.search as string) || '')

const filteredInstances = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return instances.value
  return instances.value.filter(i =>
    i.id.toLowerCase().includes(q) ||
    i.name.toLowerCase().includes(q) ||
    (i.xlsx_file || '').toLowerCase().includes(q) ||
    (i.protocol || '').toLowerCase().includes(q)
  )
})

const xlsxHint = computed(() => {
  if (form.value.protocol === 'microgrid') return ''
  if (form.value.protocol === 'modbus_tcp') {
    return 'Modbus 格式: 名称 | IOA | 类型 | 类型 | 系数 | 基值 | 别名 | 寄存器地址 | 功能码 | 数据类型 | (额外列自动忽略)'
  }
  return 'IEC104 格式: 名称 | IOA | 数据类型 | 测点类型 | 系数 | 基值 | 别名'
})

// Reset wizard state when dialog opens/closes
watch(showAddDialog, (v) => {
  if (!v) wizardStep.value = 0
})

async function wizardNext() {
  if (wizardStep.value === 0) {
    // Validate step 1 fields
    if (!form.value.name) { ElMessage.warning('请输入实例名称'); return }
    wizardStep.value++
  } else if (wizardStep.value === 1) {
    if (!form.value.iec104_port) { ElMessage.warning('请填写端口号'); return }
    wizardStep.value++
  } else if (wizardStep.value === 2) {
    wizardStep.value++
  }
}

function onSelectionChange(rows: InstanceState[]) {
  selectedIds.value = rows.map(r => r.id)
}

async function batchStart() {
  batchLoading.value = true
  let ok = 0
  for (const id of selectedIds.value) {
    try { await startInstance(id); ok++ } catch {}
  }
  ElMessage.success(`已启动 ${ok}/${selectedIds.value.length} 个实例`)
  selectedIds.value = []
  await fetchData()
  batchLoading.value = false
}

async function batchStop() {
  batchLoading.value = true
  let ok = 0
  for (const id of selectedIds.value) {
    try { await stopInstance(id); ok++ } catch {}
  }
  ElMessage.success(`已停止 ${ok}/${selectedIds.value.length} 个实例`)
  selectedIds.value = []
  await fetchData()
  batchLoading.value = false
}

async function batchDelete() {
  const count = selectedIds.value.length
  try {
    await ElMessageBox.confirm(`确定删除选中的 ${count} 个实例？`, '批量删除', { type: 'warning' })
  } catch { return }
  batchLoading.value = true
  let ok = 0
  for (const id of selectedIds.value) {
    try { await deleteInstance(id); ok++ } catch {}
  }
  ElMessage.success(`已删除 ${ok}/${count} 个实例`)
  selectedIds.value = []
  await fetchData()
  batchLoading.value = false
}

async function fetchData() {
  loading.value = true
  try {
    instances.value = await listInstances()
    globalStatus.value = await getStatus()
  } catch (e: any) {
    ElMessage.error('获取实例列表失败: ' + (e?.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

async function fetchFiles() {
  try {
    availableFiles.value = await listFiles()
  } catch {}
}

function handleEdit(row: InstanceState) {
  editing.value = true
  form.value = {
    id: row.id,
    name: row.name,
    iec104_port: row.iec104_port,
    xlsx_file: row.xlsx_file,
    http_enabled: row.http_enabled ?? false,
    http_port: row.http_port ?? 8081,
    protocol: row.protocol || 'iec104',
  }
  modbusSlaveId.value = 1
  modbusByteOrder.value = 'ABCD'
  showAddDialog.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    const data: InstanceConfig = { ...form.value }
    if (data.protocol === 'modbus_tcp') {
      data.modbus_config = {
        port: data.iec104_port,
        slave_id: modbusSlaveId.value,
        byte_order: modbusByteOrder.value,
      }
    }
    if (editing.value) {
      await updateInstance(data.id!, data)
      ElMessage.success('已更新')
      showAddDialog.value = false
      resetForm()
      await fetchData()
    } else {
      const { id, ...createData } = data
      await createInstance(createData)
      // Refresh list to get the new instance's ID
      await fetchData()
      const newInstance = instances.value.find(i => i.name === createData.name && i.iec104_port === createData.iec104_port)
      if (startAfterCreate.value && newInstance) {
        try {
          await startInstance(newInstance.id)
          ElMessage.success('已创建并启动')
        } catch {
          ElMessage.success('已创建，但自动启动失败')
        }
      } else {
        ElMessage.success('已创建')
      }
      showAddDialog.value = false
      resetForm()
    }
  } catch (e: any) {
    ElMessage.error((e?.response?.data?.error || e.message))
  } finally {
    saving.value = false
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

async function handleDelete(id: string) {
  try {
    await ElMessageBox.confirm('确定删除此实例？', '确认')
    await deleteInstance(id)
    ElMessage.success('已删除')
    await fetchData()
  } catch (e: any) {
    // Ignore cancel dialog, show other errors
    if (e !== 'cancel') {
      ElMessage.error('删除失败: ' + (e?.response?.data?.error || e.message))
    }
  }
}

function handleFileChange(file: any) {
  const raw = file.raw || file
  // Client-side size check
  const MAX_SIZE = 20 * 1024 * 1024 // 20MB
  if (raw.size > MAX_SIZE) {
    ElMessage.warning(`文件过大 (${(raw.size / 1024 / 1024).toFixed(1)}MB)，请上传小于 20MB 的文件`)
    return
  }
  // Check for overwrite
  const existingNames = availableFiles.value.map(f => f.name)
  const fileName = raw.name
  const willOverwrite = existingNames.some(n => n === fileName || n.endsWith('/' + fileName))
  if (willOverwrite) {
    ElMessage.info(`文件 "${fileName}" 已存在，将覆盖原文件`)
  }
  uploading.value = true
  uploadExcel(raw, true).then((filename) => {
    ElMessage.success(`上传成功: ${filename} (${(raw.size / 1024).toFixed(1)}KB)`)
    form.value.xlsx_file = filename
    fetchFiles()
  }).catch((e: any) => {
    ElMessage.error('上传失败: ' + (e?.response?.data?.error || e.message))
  }).finally(() => {
    uploading.value = false
  })
}

function openMicrogrid(id: string) {
  router.push(`/microgrid/${id}`)
}

function openPointTableEditor(id: string, protocol?: string) {
  pointEditorInstanceId.value = id
  pointEditorProtocol.value = protocol || 'iec104'
  pointEditorVisible.value = true
}

function resetForm() {
  editing.value = false
  form.value = { name: '', iec104_port: 2404, xlsx_file: '', http_enabled: false, http_port: 8081, protocol: 'iec104' }
  modbusSlaveId.value = 1
  modbusByteOrder.value = 'ABCD'
  selectedFile.value = null
}

function protoLabel(proto?: string): string {
  if (proto === 'modbus_tcp') return 'Modbus TCP'
  if (proto === 'microgrid') return '微电网'
  return 'IEC104'
}

function protoTagType(proto?: string): 'success' | 'primary' | 'info' | 'warning' {
  if (proto === 'modbus_tcp') return 'success'
  if (proto === 'microgrid') return 'warning'
  return 'primary'
}

function displayPort(row: InstanceState): string {
  if (row.protocol === 'microgrid') return String(row.iec104_port)
  if (row.protocol === 'modbus_tcp' && row.iec104_port) return String(row.iec104_port)
  return String(row.iec104_port)
}

function fmtUptime(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h > 0 ? `${h}h${m}m` : `${m}m`
}

onMounted(() => {
  fetchData()
  fetchFiles()
})
</script>

<style scoped>
/* P8: AnimatedList stagger — table rows */
@keyframes row-stagger {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}

:deep(.el-table__row) {
  animation: row-stagger 0.35s ease both;
}

:deep(.el-table__row:nth-child(1))  { animation-delay: 0.05s; }
:deep(.el-table__row:nth-child(2))  { animation-delay: 0.10s; }
:deep(.el-table__row:nth-child(3))  { animation-delay: 0.15s; }
:deep(.el-table__row:nth-child(4))  { animation-delay: 0.20s; }
:deep(.el-table__row:nth-child(5))  { animation-delay: 0.25s; }
:deep(.el-table__row:nth-child(6))  { animation-delay: 0.30s; }
:deep(.el-table__row:nth-child(7))  { animation-delay: 0.35s; }
:deep(.el-table__row:nth-child(8))  { animation-delay: 0.40s; }
:deep(.el-table__row:nth-child(9))  { animation-delay: 0.45s; }
:deep(.el-table__row:nth-child(10)) { animation-delay: 0.50s; }
:deep(.el-table__row:nth-child(11)) { animation-delay: 0.55s; }
:deep(.el-table__row:nth-child(12)) { animation-delay: 0.60s; }

/* Batch toolbar transition */
.slide-fade-enter-active { transition: all 0.3s cubic-bezier(0.22, 1, 0.36, 1); }
.slide-fade-leave-active { transition: all 0.2s ease; }
.slide-fade-enter-from { opacity: 0; transform: translateY(-8px); }
.slide-fade-leave-to { opacity: 0; transform: translateY(-8px); }

/* Wizard review section */
.wizard-review { display: flex; flex-direction: column; gap: 10px; }
.review-section { display: flex; padding: 8px 12px; border-radius: 6px; background: var(--el-fill-color-lighter, #1e293b); }
.review-label { width: 100px; font-size: 13px; color: var(--text-muted, #64748b); flex-shrink: 0; }
.review-value { font-size: 13px; font-weight: 500; color: var(--text-primary); }

/* File preview */
.file-preview {
  margin-top: 6px;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--bg-input, #0f1623);
  font-size: 12px;
  display: flex;
  gap: 16px;
}
.fp-row { display: flex; gap: 4px; color: var(--text-secondary); }
.fp-label { color: var(--text-muted); }

/* Start after create option */
.wizard-start-option {
  margin-top: 16px;
  padding: 12px;
  border-radius: 8px;
  border: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.start-hint { font-size: 12px; color: var(--text-muted); }
</style>
