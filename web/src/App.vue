<template>
  <el-container style="height: 100vh">
    <el-header style="display: flex; align-items: center; background: #409eff; color: white; padding: 0 20px">
      <h2 style="margin: 0; font-size: 18px; flex: 1">IEC104 模拟器管理系统 v2.1</h2>
      <el-tag v-if="status" type="" style="margin-right: 12px" effect="dark">
        运行: {{ status.running }} / 总: {{ status.configured }}
      </el-tag>
    </el-header>
    <el-container style="height: calc(100vh - 60px)">
      <el-aside width="160px" style="background: #f5f7fa; border-right: 1px solid #e4e7ed">
        <el-menu :router="true" :default-active="currentRoute" style="border-right: none">
          <el-menu-item index="/config">
            <el-icon><Setting /></el-icon>
            <span>配置管理</span>
          </el-menu-item>
          <el-menu-item index="/monitor">
            <el-icon><Monitor /></el-icon>
            <span>运行监控</span>
          </el-menu-item>
        </el-menu>
      </el-aside>
      <el-main style="padding: 20px; background: #f0f2f5">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { Setting, Monitor } from '@element-plus/icons-vue'
import { getStatus, type GlobalStatus } from './api'

const route = useRoute()
const currentRoute = computed(() => route.path)
const status = ref<GlobalStatus | null>(null)

onMounted(async () => {
  try {
    status.value = await getStatus()
  } catch {}
})
</script>

<style>
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
.el-menu-item { font-size: 14px; }
</style>
