# API 测试工具 设计文档

> 日期: 2026-06-02 | 版本: v2.0

## 目标

为 IEC104 模拟器管理系统新增「接口测试」功能，类似 Postman 的 API 测试器，支持 Collection 树形管理、环境变量系统、请求代理。

## 设计原则

- 后端代理模式：请求由服务器发起，避免 CORS 问题，支持内网接口
- 界面风格与现有深色主题一致
- 支持 Collection 树形文件夹管理（新建/复制/删除）
- 支持环境变量系统（多环境切换）
- 请求历史自动保存

---

## 1. 后端设计

### 1.1 API 端点总览

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/proxy` | 代理 HTTP 请求 |
| GET | `/api/v1/proxy/collections` | 获取所有 Collection |
| POST | `/api/v1/proxy/collections` | 创建 Collection/文件夹 |
| PUT | `/api/v1/proxy/collections/{id}` | 更新 Collection |
| DELETE | `/api/v1/proxy/collections/{id}` | 删除 Collection |
| POST | `/api/v1/proxy/requests` | 保存请求 |
| PUT | `/api/v1/proxy/requests/{id}` | 更新请求 |
| DELETE | `/api/v1/proxy/requests/{id}` | 删除请求 |
| POST | `/api/v1/proxy/requests/{id}/copy` | 复制请求 |
| GET | `/api/v1/proxy/environments` | 获取所有环境变量 |
| POST | `/api/v1/proxy/environments` | 创建环境 |
| PUT | `/api/v1/proxy/environments/{id}` | 更新环境 |
| DELETE | `/api/v1/proxy/environments/{id}` | 删除环境 |

### 1.2 代理请求

**`POST /api/v1/proxy`**

请求体：
```json
{
  "method": "POST",
  "url": "http://192.168.1.100:8080/api/data",
  "headers": {"Content-Type": "application/json"},
  "body": "{\"key\": \"value\"}",
  "timeout": 30
}
```

响应体：
```json
{
  "status": 200,
  "status_text": "OK",
  "headers": {"Content-Type": "application/json"},
  "body": "{\"result\": \"success\"}",
  "time_ms": 123,
  "size": 456
}
```

### 1.3 Collection 数据模型

```json
{
  "id": "col_001",
  "name": "实例管理",
  "parent_id": null,
  "type": "folder",
  "children": [
    {
      "id": "req_001",
      "name": "获取实例列表",
      "type": "request",
      "method": "GET",
      "url": "{{base_url}}/api/v1/instances",
      "headers": {},
      "body": null,
      "environment_id": "env_001"
    }
  ]
}
```

### 1.4 环境变量数据模型

```json
{
  "id": "env_001",
  "name": "本地开发",
  "variables": {
    "base_url": "http://localhost:8989",
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "instance_id": "a1b2c3d4e5f6"
  }
}
```

### 1.5 实现要点

- 文件: `pkg/api/proxy_handler.go`
- 使用 `net/http.Client` + `http.Transport`
- 支持 HTTP/HTTPS 代理
- 超时可配（默认 30s）
- Collection 和环境变量存储在 `config/proxy-store.json`
- 错误处理: DNS 解析失败、连接拒绝、TLS 错误、超时

---

## 2. 前端设计

### 2.1 菜单位置

左侧导航栏新增第4项「接口测试」，图标使用 🔌

### 2.2 页面布局（三栏）

```
┌──────────────────────────────────────────────────────────────┐
│  接口测试                                                     │
├──────────────┬───────────────────────────────────────────────┤
│ Collection   │  [GET ▼] [URL __________________] [发送]     │
│ 树形列表     ├───────────────────────────────────────────────┤
│              │  Headers  │  Body  │  Pre-Script  │  Auth     │
│ 📁 实例管理  ├───────────────────────────────────────────────┤
│   📄 获取列表│  请求配置面板                                  │
│   📄 创建实例│                                               │
│ 📁 测点操作  ├───────────────────────────────────────────────┤
│   📄 批量写入│  响应结果面板                                  │
│   📄 策略配置│  Status: 200  │  Time: 123ms  │  Size: 456B   │
│              │  ┌─────────────────────────────────────────┐ │
│ [+ 新建文件夹│  │  { "result": "success" }               │ │
│ [+ 新建请求  │  └─────────────────────────────────────────┘ │
├──────────────┴───────────────────────────────────────────────┤
│  环境: [本地开发 ▼]  │  请求历史                              │
└──────────────────────────────────────────────────────────────┘
```

### 2.3 Collection 树形管理

**左侧面板功能：**
- 文件夹：可展开/折叠，右键菜单（重命名/删除）
- 请求：点击加载到编辑区，右键菜单（复制/删除/重命名）
- 拖拽排序：支持拖拽移动请求到不同文件夹
- 搜索框：按名称搜索 Collection

**操作按钮：**
- `[+ 新建文件夹]`：创建新 Collection 文件夹
- `[+ 新建请求]`：在根目录创建空白请求

### 2.4 环境变量系统

**底部工具栏：**
- 环境选择下拉框：切换当前使用的环境
- `[管理环境]`：打开环境管理弹窗

**环境管理弹窗：**
```
┌─────────────────────────────────────────┐
│  环境变量管理                            │
├─────────────────────────────────────────┤
│  环境列表:                               │
│  [本地开发] [测试环境] [生产环境] [+新建] │
├─────────────────────────────────────────┤
│  当前环境: 本地开发                       │
│  ┌──────────────┬─────────────────────┐ │
│  │ 变量名        │ 值                  │ │
│  │ base_url      │ http://localhost:8989│ │
│  │ token         │ eyJhbG...           │ │
│  └──────────────┴─────────────────────┘ │
│  [+ 添加变量]                            │
│                       [保存] [取消]      │
└─────────────────────────────────────────┘
```

### 2.5 变量替换规则

1. 选择环境后，发送时自动替换 URL/Headers/Body 中的 `{{variable}}`
2. 优先级：请求级变量 > 环境变量
3. 未定义的变量保留原样不报错

### 2.6 交互流程

1. 创建环境 → 配置变量（base_url, token 等）
2. 创建 Collection 文件夹 → 添加请求
3. 编辑请求（Method/URL/Headers/Body）
4. 选择环境 → 点击「发送」
5. 后端替换变量 → 代理请求 → 返回结果
6. 响应区展示 Status/Time/Body

### 2.7 请求历史

- 最近 50 条请求自动保存到 localStorage
- 右侧抽屉展示，点击可回显
- 显示 Method/URL/Status/Time

---

## 3. 文件结构

```
web/src/views/ProxyPage.vue              ← 主页面
web/src/components/ProxyTree.vue         ← Collection 树形组件
web/src/components/EnvManager.vue        ← 环境变量管理弹窗
web/src/api/index.ts                     ← 新增 API 方法
pkg/api/proxy_handler.go                 ← 代理处理器
pkg/api/proxy_store.go                   ← Collection/环境存储
web/src/router/index.ts                  ← 新增路由 /proxy
web/src/App.vue                          ← 侧边栏新增菜单项
```

---

## 4. 交互设计稿

可交互 HTML 设计稿: `docs/superpowers/specs/2026-06-02-api-tester-design-v2.html`

浏览器打开即可体验完整交互流程（模拟数据）。

---

## 5. 验收标准

### 基础功能
- [ ] 左侧菜单显示「接口测试」入口
- [ ] 支持选择 HTTP Method（GET/POST/PUT/DELETE/PATCH）
- [ ] URL 输入框支持 `{{variable}}` 变量占位符
- [ ] Headers 编辑器支持添加/删除/启用/禁用
- [ ] Body 支持 JSON/Text/None 切换
- [ ] 点击发送调用后端代理 API
- [ ] 响应区显示 Status/Time/Size + JSON 高亮

### Collection 管理
- [ ] 左侧树形展示 Collection 文件夹和请求
- [ ] 支持新建文件夹/请求
- [ ] 支持复制/删除/重命名请求
- [ ] 支持拖拽移动请求到不同文件夹
- [ ] 支持搜索 Collection

### 环境变量
- [ ] 支持创建/编辑/删除环境
- [ ] 支持切换当前环境
- [ ] 发送时自动替换 `{{variable}}`
- [ ] 支持多环境切换

### 数据持久化
- [ ] Collection 数据保存到后端
- [ ] 环境变量保存到后端
- [ ] 请求历史保存到 localStorage
- [ ] 深色主题一致
