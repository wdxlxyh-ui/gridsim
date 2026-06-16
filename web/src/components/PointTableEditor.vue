<template>
  <el-dialog v-model="dialogVisible" title="编辑点表" width="920px" @open="loadPoints" :close-on-click-modal="false">
    <div v-loading="loading" style="min-height: 200px">
      <el-alert v-if="duplicateIOAs.length > 0" type="error" :closable="false"
        :title="'IOA 重复: ' + duplicateIOAs.join(', ')" style="margin-bottom: 12px" />

      <el-table :data="editPoints" border stripe style="width: 100%" max-height="500"
        @selection-change="onSelectionChange">
        <el-table-column type="selection" width="40" />
        <el-table-column label="IOA" width="100">
          <template #default="{ row }">
            <el-input-number v-model="row.ioa" :min="0" :max="16777215" :controls="false" size="small" style="width: 100%" />
          </template>
        </el-table-column>
        <el-table-column label="名称" min-width="140">
          <template #default="{ row }">
            <el-input v-model="row.name" size="small" />
          </template>
        </el-table-column>
        <el-table-column label="数据类型" width="120">
          <template #default="{ row }">
            <el-select v-model="row.value_type" size="small" style="width: 100%">
              <el-option label="FLOAT" value="FLOAT" />
              <el-option label="DOUBLE" value="DOUBLE" />
              <el-option label="INT" value="INT" />
              <el-option label="BIT" value="BIT" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="测点类型" width="110">
          <template #default="{ row }">
            <el-select v-model="row.point_type" size="small" style="width: 100%">
              <el-option label="AI (遥测)" value="AI" />
              <el-option label="DI (遥信)" value="DI" />
              <el-option label="PI (遥脉)" value="PI" />
              <el-option label="DO (遥控)" value="DO" />
              <el-option label="AO (遥调)" value="AO" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="系数" width="90">
          <template #default="{ row }">
            <el-input-number v-model="row.efficient" :controls="false" size="small" style="width: 100%" />
          </template>
        </el-table-column>
        <el-table-column label="基值" width="90">
          <template #default="{ row }">
            <el-input-number v-model="row.base_value" :controls="false" size="small" style="width: 100%" />
          </template>
        </el-table-column>
        <el-table-column label="别名" width="100">
          <template #default="{ row }">
            <el-input v-model="row.alias" size="small" />
          </template>
        </el-table-column>
        <el-table-column v-if="isModbus" label="寄存器地址" width="110">
          <template #default="{ row }">
            <el-input-number v-model="row.register_address" :controls="false" size="small" style="width: 100%" />
          </template>
        </el-table-column>
        <el-table-column v-if="isModbus" label="功能码" width="90">
          <template #default="{ row }">
            <el-select v-model="row.function_code" size="small" style="width: 100%">
              <el-option :label="1" :value="1" />
              <el-option :label="3" :value="3" />
              <el-option :label="4" :value="4" />
              <el-option :label="16" :value="16" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="60" fixed="right">
          <template #default="{ $index }">
            <el-button type="danger" size="small" :icon="Delete" circle @click="removeRow($index)" />
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top: 12px; font-size: 12px; color: #999">
        共 {{ editPoints.length }} 个测点
      </div>
    </div>
    <template #footer>
      <el-button @click="dialogVisible = false" :disabled="saving">取消</el-button>
      <el-button type="warning" @click="addRow" :disabled="saving">新增行</el-button>
      <el-button type="danger" @click="deleteSelected" :disabled="saving || selectedRows.length === 0">
        删除选中 ({{ selectedRows.length }})
      </el-button>
      <el-button type="primary" @click="save" :loading="saving" :disabled="duplicateIOAs.length > 0">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getPointTable, savePointTable } from '../api'

const props = defineProps<{
  visible: boolean
  instanceId: string
  protocol?: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'saved'): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const loading = ref(false)
const saving = ref(false)
const editPoints = ref<any[]>([])
const selectedRows = ref<any[]>([])

const isModbus = computed(() => props.protocol === 'modbus_tcp')

const duplicateIOAs = computed(() => {
  const seen = new Map<string, string[]>()
  for (const p of editPoints.value) {
    const key = `${p.point_type}:${p.ioa}`
    if (!seen.has(key)) seen.set(key, [])
    seen.get(key)!.push(p.name)
  }
  const dups: string[] = []
  for (const [key, names] of seen) {
    if (names.length > 1) dups.push(`${key} (${names.join(', ')})`)
  }
  return dups
})

async function loadPoints() {
  loading.value = true
  try {
    const data = await getPointTable(props.instanceId)
    editPoints.value = data.points.map(p => ({
      ...p,
      register_address: p.register_address || 0,
      function_code: p.function_code || 0,
    }))
  } catch (e: any) {
    ElMessage.error('加载点表失败: ' + (e?.response?.data?.error?.message || e?.response?.data?.error || e.message))
    dialogVisible.value = false
  } finally {
    loading.value = false
  }
}

function addRow() {
  editPoints.value.push({
    ioa: 0,
    name: '',
    value_type: 'FLOAT',
    point_type: 'AI',
    efficient: 1.0,
    base_value: 0,
    alias: '',
    register_address: 0,
    function_code: 0,
  })
}

function removeRow(index: number) {
  editPoints.value.splice(index, 1)
}

function onSelectionChange(rows: any[]) {
  selectedRows.value = rows
}

function deleteSelected() {
  const ids = new Set(selectedRows.value)
  editPoints.value = editPoints.value.filter(p => !ids.has(p))
  selectedRows.value = []
}

async function save() {
  saving.value = true
  try {
    await savePointTable(props.instanceId, editPoints.value)
    ElMessage.success('点表已保存')
    emit('saved')
    dialogVisible.value = false
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e?.response?.data?.error?.message || e?.response?.data?.error || e.message))
  } finally {
    saving.value = false
  }
}
</script>
