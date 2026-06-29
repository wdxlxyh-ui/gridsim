<template>
  <el-card v-if="visible" class="client-config-card">
    <template #header>
      <span>客户端配置</span>
    </template>
    <el-form :model="config" label-width="120px" size="small">
      <el-form-item label="远端地址" required>
        <el-input
          v-model="config.remote_addr"
          placeholder="如 192.168.1.100"
        />
      </el-form-item>
      <el-form-item label="远端端口" required>
        <el-input-number v-model="config.remote_port" :min="1" :max="65535" />
      </el-form-item>
      <el-form-item label="公共地址">
        <el-input-number v-model="config.common_addr" :min="1" :max="255" />
      </el-form-item>
      <el-form-item label="重连间隔(秒)">
        <el-input-number v-model="config.reconnect_delay" :min="1" :max="300" />
      </el-form-item>
      <el-form-item label="连接超时(秒)">
        <el-input-number v-model="config.connect_timeout" :min="1" :max="60" />
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
import type { IEC104ClientConfig } from '../../api'

defineProps<{
  config: IEC104ClientConfig
  visible: boolean
}>()
</script>

<style scoped>
.client-config-card {
  margin-bottom: 16px;
}
</style>
