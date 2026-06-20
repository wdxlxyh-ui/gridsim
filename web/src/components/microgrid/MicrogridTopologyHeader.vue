<template>
  <el-card shadow="never" class="header-card">
    <div class="header-row">
      <div class="header-left">
        <el-button @click="$emit('go-back')" text>← 返回</el-button>
        <span style="font-size: 16px; font-weight: 600; margin-left: 8px">{{ instanceName || '微电网仿真系统' }}</span>
        <el-tag :type="running ? 'success' : 'info'" style="margin-left: 12px">
          {{ running ? '运行中' : '已停止' }}
        </el-tag>
      </div>
      <div class="header-right">
        <el-button v-if="!running" type="success" :loading="actionLoading" @click="$emit('start')">启动</el-button>
        <el-button v-else type="warning" :loading="actionLoading" @click="$emit('stop')">停止</el-button>
        <el-button :disabled="running || !topologyChanged" type="primary" @click="$emit('save-topology')">保存拓扑</el-button>
        <el-button @click="$emit('export-topology')">导出拓扑</el-button>
        <el-button @click="$emit('import-topology')" :disabled="running">导入拓扑</el-button>
        <el-button @click="$emit('export-xlsx')">导出点表</el-button>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
defineProps<{
  instanceName: string
  running: boolean
  actionLoading: boolean
  topologyChanged: boolean
}>()
defineEmits<{
  'go-back': []
  start: []
  stop: []
  'save-topology': []
  'export-topology': []
  'import-topology': []
  'export-xlsx': []
}>()
</script>

<style scoped>
.header-card { margin-bottom: 12px; }
.header-row { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
.header-left { display: flex; align-items: center; }
</style>
