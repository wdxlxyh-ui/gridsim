<template>
  <div class="request-bar">
    <el-select v-model="method" class="method-select" :class="method.toLowerCase()">
      <el-option v-for="m in ['GET','POST','PUT','DELETE','PATCH']" :key="m" :value="m" :label="m" />
    </el-select>
    <el-input v-model="url" placeholder="输入 URL，支持 {{variable}}" class="url-input" />
    <el-button type="warning" @click="$emit('send')" :loading="sending">▶ 发送</el-button>
    <el-button @click="$emit('save')">💾 保存</el-button>
    <el-dropdown trigger="click">
      <el-button size="small">📤 导出 <el-icon><ArrowDown /></el-icon></el-button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item @click="$emit('export-gridsim')">导出 GridSim 格式</el-dropdown-item>
          <el-dropdown-item @click="$emit('export-postman')">导出 Postman 格式</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
    <el-dropdown trigger="click">
      <el-button size="small">📥 导入 <el-icon><ArrowDown /></el-icon></el-button>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item @click="$emit('import', 'gridsim')">导入 GridSim 格式</el-dropdown-item>
          <el-dropdown-item @click="$emit('import', 'postman')">导入 Postman 格式</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ArrowDown } from '@element-plus/icons-vue'

const props = defineProps<{
  method: string
  url: string
  sending: boolean
}>()

const emit = defineEmits<{
  'update:method': [v: string]
  'update:url': [v: string]
  send: []
  save: []
  'export-gridsim': []
  'export-postman': []
  'import': [format: string]
}>()

const method = computed({ get: () => props.method, set: (v) => emit('update:method', v) })
const url = computed({ get: () => props.url, set: (v) => emit('update:url', v) })
</script>

<style scoped>
.request-bar { display: flex; gap: 8px; padding: 12px 16px; background: #111827; border-bottom: 1px solid #1e293b; }
.method-select :deep(.el-input__wrapper) { background: #0d1117 !important; width: auto; min-width: 58px; padding: 0 4px; }
.method-select :deep(.el-input__inner) { text-align: center; font-weight: 600; font-size: 12px; width: 100%; }
</style>
