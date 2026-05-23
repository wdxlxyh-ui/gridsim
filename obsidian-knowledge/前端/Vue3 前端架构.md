---
title: Vue3 前端架构
tags:
  - frontend
  - vue3
  - ui
aliases:
  - Frontend Architecture
created: 2026-05-23
---

# Vue3 前端架构

## 技术栈

| 层 | 选型 |
|----|------|
| 框架 | Vue 3 (Composition API) |
| 语言 | TypeScript |
| UI 库 | Element Plus |
| 图表 | ECharts |
| 构建 | Vite |
| HTTP | Axios |

## 页面结构

| 页面 | 路由 | 说明 |
|------|------|------|
| ConfigPage | `/config` | 实例配置管理 + 容量显示 |
| MonitorPage | `/monitor` | 运行监控（自动刷新） |
| DetailPage | `/instance/:id` | 实例详情（测点/置数/策略/CSV 回放） |
| TrendPage | `/trend/:id` | ECharts 趋势对比 |
| MicrogridEditor | `/microgrid/:id` | 微电网设备管理 + SVG 拓扑图 |

## 状态管理模式

| 场景 | 模式 | 说明 |
|------|------|------|
| 表格数据 | `ref<T[]>([])` | 替换整个数组触发响应式更新 |
| 简单状态 | `reactive<Record>()` | setValues, autoStrategies |
| 组件引用 | `ref<HTMLElement>()` | 配合 nextTick |

## 置数值隔离模式

```
setValues[ioa] ← 存储用户手动输入
row.value     ← 后端实时数据

fetchPoints 初始化:
  if (p.ioa in setValues) return  // 保护用户输入

displayValue(): 只读后端数据
```

## 认证与授权

- JWT token 存储在 `localStorage`
- 登录页面路由 `/login`
- API 拦截器自动处理 401 重定向
- `users.json` 预置默认用户

---

相关笔记：[[前端/前端代码规范|前端代码规范]]、[[前端/TrendPage趋势页|TrendPage 趋势页]]、[[前端/DetailPage详情页|DetailPage 详情页]]
