# GridSim UI 升级方案 — 基于 ReactBits 动效分析

> 基于 [ReactBits](https://reactbits.dev) 110+ 组件库分析，筛选适合工业管理后台的动效组件，按优先级分阶段实施。

## 当前 UI 现状

| 维度 | 现状 |
|------|------|
| 技术栈 | Vue 3 + Element Plus + ECharts，零动画库 |
| 主题 | 暗色系 (`#0a0e17` / `#111827` / `#1e293b`)，强调色 `#f59e0b` |
| 布局 | Header + Sidebar(可折叠) + Main |
| 页面 | 登录、配置管理、运行监控、实时趋势、接口测试、微电网编辑器、实例详情 |
| 动效 | 仅 fade transition + sidebar 宽度过渡 |

---

## Phase 1：高 ROI、低风险

全部纯 CSS/JS 实现，零新增依赖。

| # | 改动 | 区域 | 实现方式 |
|---|------|------|---------|
| P1 | 登录页 Aurora 渐变背景 | LoginPage.vue | CSS keyframes 渐变动画 |
| P2 | 侧边栏 PillNav 滑动指示器 | App.vue | CSS transform，替换 el-menu active 样式 |
| P3 | 页面切换 slide+fade transition | App.vue router-view | Vue `<transition>` + CSS |
| P4 | Header 状态数字 CountUp | App.vue | CSS `@keyframes` 轻量 JS |
| P5 | Header 底部发光边线 | App.vue | CSS 伪元素 + animation |
| P6 | 登录标题 SplitText 入场 | LoginPage.vue | CSS stagger animation |
| P7 | 全局 Noise 纹理叠加 | App.vue | CSS SVG filter 伪元素 |

## Phase 2：中等 ROI（待 Phase 1 验收后实施）

| # | 改动 | 区域 |
|---|------|------|
| P8 | 列表/卡片 AnimatedList stagger | ConfigPage / MonitorPage |
| P9 | 卡片 SpotlightCard hover 光效 | ConfigPage / MonitorPage |
| P10 | 错误状态码 GlitchText | ProxyPage |
| P11 | 登录卡片 SpotlightCard | LoginPage |
| P12 | 侧边栏折叠缓动优化 | App.vue |

## Phase 3：锦上添花（可选）

| # | 改动 | 区域 |
|---|------|------|
| P13 | GooeyNav 侧边栏指示器 | App.vue |
| P14 | 点击 ClickSpark 粒子特效 | 全局按钮 |
| P15 | Background Particles/DotGrid | 登录页 |
| P16 | 请求树 StaggeredMenu | ProxyPage |

---

## 原则

1. **零新增依赖** — 所有效果纯 CSS + 少量 Vue composable
2. **工业科技风** — 选择 ElectricBorder、StarBorder、CountUp 等偏科技/工业风组件
3. **性能优先** — 管理后台数据密集，动效不影响 ECharts 和表格渲染
4. **背景动效仅限登录页** — 主界面保持克制
