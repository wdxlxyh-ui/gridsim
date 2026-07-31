<template>
  <div class="enos-credentials">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>EnOS 凭据管理</span>
          <el-button type="primary" @click="showCreateDialog">
            <el-icon><Plus /></el-icon>
            新建凭据
          </el-button>
        </div>
      </template>

      <el-table :data="credentials" v-loading="loading">
        <el-table-column prop="name" label="配置名称" min-width="120" />
        <el-table-column prop="apigw_address" label="API 网关" min-width="200" />
        <el-table-column prop="org_id" label="Organization ID" min-width="150" />
        <el-table-column label="密钥状态" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.access_key_present && row.secret_key_present" type="success">
              已配置
            </el-tag>
            <el-tag v-else type="danger">未配置</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button size="small" @click="testCredential(row)">测试</el-button>
            <el-button size="small" @click="editCredential(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="deleteCredential(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑凭据' : '新建凭据'"
      width="600px"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="120px">
        <el-form-item label="凭据ID" prop="id" v-if="!isEditing">
          <el-input v-model="form.id" placeholder="唯一标识符，如: enos-prod-cn5" />
        </el-form-item>
        <el-form-item label="配置名称" prop="name">
          <el-input v-model="form.name" placeholder="如: 生产环境 CN5" />
        </el-form-item>
        <el-form-item label="API 网关" prop="apigw_address">
          <el-input v-model="form.apigw_address" placeholder="https://ag-cn5.example.com" />
        </el-form-item>
        <el-form-item label="Organization ID" prop="org_id">
          <el-input v-model="form.org_id" placeholder="org-example-12345" />
        </el-form-item>
        <el-form-item label="Access Key" prop="access_key">
          <el-input v-model="form.access_key" placeholder="输入新的Access Key（留空则不更新）" />
        </el-form-item>
        <el-form-item label="Secret Key" prop="secret_key">
          <el-input 
            v-model="form.secret_key" 
            type="password" 
            placeholder="输入新的Secret Key（留空则不更新）"
            show-password 
          />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitForm" :loading="submitting">确定</el-button>
      </template>
    </el-dialog>

    <!-- 测试结果对话框 -->
    <el-dialog v-model="testDialogVisible" title="连接测试结果" width="500px">
      <div v-if="testResult">
        <el-result
          :icon="testResult.success ? 'success' : 'error'"
          :title="testResult.success ? '连接成功' : '连接失败'"
          :sub-title="testResult.message"
        >
          <template #extra>
            <el-descriptions :column="1" size="small">
              <el-descriptions-item label="API 网关">{{ testResult.api_gateway }}</el-descriptions-item>
              <el-descriptions-item label="Organization ID">{{ testResult.org_id }}</el-descriptions-item>
              <el-descriptions-item label="测试时间">{{ formatTime(testResult.tested_at) }}</el-descriptions-item>
            </el-descriptions>
          </template>
        </el-result>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox, FormInstance } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { 
  listEnOSCredentials, 
  createEnOSCredential, 
  updateEnOSCredential, 
  deleteEnOSCredential,
  testEnOSCredential,
  type EnOSCredential,
  type EnOSTestResult
} from '@/api'

const loading = ref(false)
const credentials = ref<EnOSCredential[]>([])
const dialogVisible = ref(false)
const testDialogVisible = ref(false)
const isEditing = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const testResult = ref<EnOSTestResult>()

const form = ref({
  id: '',
  name: '',
  apigw_address: '',
  org_id: '',
  access_key: '',
  secret_key: ''
})

const rules = {
  id: [{ required: true, message: '请输入凭据ID', trigger: 'blur' }],
  name: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
  apigw_address: [
    { required: true, message: '请输入API网关地址', trigger: 'blur' },
    { pattern: /^https:\/\//, message: 'API网关必须使用HTTPS', trigger: 'blur' }
  ],
  org_id: [{ required: true, message: '请输入Organization ID', trigger: 'blur' }],
  access_key: [{ required: true, message: '请输入Access Key', trigger: 'blur' }],
  secret_key: [{ required: true, message: '请输入Secret Key', trigger: 'blur' }]
}

onMounted(() => {
  loadCredentials()
})

async function loadCredentials() {
  loading.value = true
  try {
    credentials.value = await listEnOSCredentials()
  } catch (error) {
    console.error('加载凭据失败:', error)
    ElMessage.error('加载凭据失败')
  } finally {
    loading.value = false
  }
}

function showCreateDialog() {
  isEditing.value = false
  form.value = {
    id: '',
    name: '',
    apigw_address: '',
    org_id: '',
    access_key: '',
    secret_key: ''
  }
  dialogVisible.value = true
}

function editCredential(credential: EnOSCredential) {
  isEditing.value = true
  form.value = {
    id: credential.id,
    name: credential.name,
    apigw_address: credential.apigw_address,
    org_id: credential.org_id,
    access_key: '',
    secret_key: ''
  }
  dialogVisible.value = true
}

async function submitForm() {
  if (!formRef.value) return
  
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    if (isEditing.value) {
      // 编辑模式，只更新非空字段
      const updates: Partial<EnOSCredential> = {}
      if (form.value.name) updates.name = form.value.name
      if (form.value.apigw_address) updates.apigw_address = form.value.apigw_address
      if (form.value.org_id) updates.org_id = form.value.org_id
      if (form.value.access_key) updates.access_key = form.value.access_key
      if (form.value.secret_key) updates.secret_key = form.value.secret_key

      await updateEnOSCredential(form.value.id, updates)
      ElMessage.success('凭据更新成功')
    } else {
      await createEnOSCredential(form.value)
      ElMessage.success('凭据创建成功')
    }
    
    dialogVisible.value = false
    loadCredentials()
  } catch (error: any) {
    console.error('保存凭据失败:', error)
    ElMessage.error(error.response?.data?.error || '保存凭据失败')
  } finally {
    submitting.value = false
  }
}

async function testCredential(credential: EnOSCredential) {
  try {
    testResult.value = await testEnOSCredential(credential.id)
    testDialogVisible.value = true
    
    if (testResult.value.success) {
      ElMessage.success('连接测试成功')
    } else {
      ElMessage.error('连接测试失败')
    }
  } catch (error: any) {
    console.error('测试连接失败:', error)
    ElMessage.error(error.response?.data?.error || '测试连接失败')
  }
}

async function deleteCredential(credential: EnOSCredential) {
  try {
    await ElMessageBox.confirm(
      `确定要删除凭据"${credential.name}"吗？此操作不可恢复。`,
      '确认删除',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    
    await deleteEnOSCredential(credential.id)
    ElMessage.success('凭据删除成功')
    loadCredentials()
  } catch (error: any) {
    if (error !== 'cancel') {
      console.error('删除凭据失败:', error)
      ElMessage.error(error.response?.data?.error || '删除凭据失败')
    }
  }
}

function formatTime(time: string): string {
  return new Date(time).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.enos-credentials {
  padding: 20px;
}
</style>