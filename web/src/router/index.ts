import { createRouter, createWebHashHistory } from 'vue-router'
import { getToken } from '../api'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/login', name: 'login', component: () => import('@/views/LoginPage.vue'), meta: { title: '登录', public: true } },
  { path: '/dashboard', name: 'dashboard', component: () => import('@/views/DashboardPage.vue'), meta: { title: '仪表盘' } },
  { path: '/config', name: 'config', component: () => import('@/views/ConfigPage.vue'), meta: { title: '配置管理' } },
  { path: '/monitor', name: 'monitor', component: () => import('@/views/MonitorPage.vue'), meta: { title: '运行监控' } },
  { path: '/trend', name: 'trend', component: () => import('@/views/TrendPage.vue'), meta: { title: '实时趋势' } },
  { path: '/proxy', name: 'proxy', component: () => import('@/views/ProxyPage.vue'), meta: { title: '接口测试' } },
  { path: '/detail/:id', name: 'detail', component: () => import('@/views/DetailPage.vue'), meta: { title: '实例详情' } },
  { path: '/microgrid/:id', name: 'microgrid', component: () => import('@/views/MicrogridEditor.vue'), meta: { title: '微电网编辑' } },
  { path: '/py-microgrid/:id', name: 'py-microgrid', component: () => import('@/views/PyMicrogridEditor.vue'), meta: { title: 'Python微电网' } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

// Navigation guard: redirect to login if not authenticated
router.beforeEach((to, _from, next) => {
  const token = getToken()
  if (to.meta.public || token) {
    next()
  } else {
    next('/login')
  }
})

export default router
