import { createRouter, createWebHashHistory } from 'vue-router'
import ConfigPage from '@/views/ConfigPage.vue'
import MonitorPage from '@/views/MonitorPage.vue'

const routes = [
  { path: '/', redirect: '/config' },
  { path: '/config', name: 'config', component: ConfigPage, meta: { title: '配置管理' } },
  { path: '/monitor', name: 'monitor', component: MonitorPage, meta: { title: '运行监控' } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
