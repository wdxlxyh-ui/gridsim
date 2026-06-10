# EGC 场景自动化测试 — 接口测试扩展设计

> **版本**: v1.0  
> **日期**: 2026-06-10  
> **范围**: 在现有 ProxyPage 接口测试工具上新增场景自动化执行能力

---

## 1. 目标

在 GridSim 的接口测试工具（ProxyPage）上扩展「场景自动化执行」功能，使测试人员能够：

1. 预置 EnOS 登录/策略管理/TSDB 查询等接口
2. 将接口组织为有序测试场景（步骤列表）
3. 一键自动执行场景，支持变量传递和断言
4. 通过 MCP 工具让 AI Agent 也能触发场景执行

核心价值：**EGC 策略配置 = 执行用例的前提，TSDB 查询 = 用例结果的判断，都在一个工具里闭环。**

---

## 2. 数据模型

### 2.1 新增结构

在 `proxy_store.go` 中新增：

```go
// TestScenario 测试场景
type TestScenario struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    CreatedAt   int64  `json:"created_at"`
    Steps       []Step `json:"steps"`
}

// Step 场景中的一个执行步骤
type Step struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Method      string            `json:"method"`           // GET/POST/PUT/DELETE
    URL         string            `json:"url"`              // 支持 {{variable}}
    Headers     map[string]string `json:"headers,omitempty"`
    Body        string            `json:"body,omitempty"`   // 支持 {{variable}}
    WaitMs      int               `json:"wait_ms"`          // 执行后等待毫秒
    PreScript   string            `json:"pre_script,omitempty"`  // 前置脚本
    PostScript  string            `json:"post_script,omitempty"` // 后置断言脚本
    ExtractVars map[string]string `json:"extract_vars,omitempty"` // 从响应提取变量
    Skip        bool              `json:"skip,omitempty"`   // 跳过此步骤
}
```

### 2.2 ProxyStore 扩展

在现有 `ProxyStore` 中新增字段：

```go
type ProxyStore struct {
    // ... existing fields ...
    Scenarios []*TestScenario `json:"scenarios"`
}
```

### 2.3 执行结果结构

```go
// ScenarioResult 场景执行结果
type ScenarioResult struct {
    ScenarioID   string      `json:"scenario_id"`
    ScenarioName string      `json:"scenario_name"`
    StartTime    int64       `json:"start_time"`
    EndTime      int64       `json:"end_time"`
    TotalSteps   int         `json:"total_steps"`
    Passed       int         `json:"passed"`
    Failed       int         `json:"failed"`
    Skipped      int         `json:"skipped"`
    Steps        []StepResult `json:"steps"`
}

// StepResult 单步执行结果
type StepResult struct {
    StepID     string          `json:"step_id"`
    StepName   string          `json:"step_name"`
    Status     string          `json:"status"` // passed / failed / skipped / error
    Response   *ProxyResponse  `json:"response,omitempty"`
    Assertions []AssertionResult `json:"assertions,omitempty"`
    Error      string          `json:"error,omitempty"`
    DurationMs int64           `json:"duration_ms"`
}

type AssertionResult struct {
    Name   string `json:"name"`
    Passed bool   `json:"passed"`
    Actual string `json:"actual"`
    Expected string `json:"expected,omitempty"`
}
```

---

## 3. API 设计

### 3.1 场景 CRUD

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/v1/proxy/scenarios` | 获取所有场景 |
| `POST` | `/api/v1/proxy/scenarios` | 创建场景 |
| `PUT` | `/api/v1/proxy/scenarios/{id}` | 更新场景 |
| `DELETE` | `/api/v1/proxy/scenarios/{id}` | 删除场景 |

### 3.2 场景执行

| 方法 | 端点 | 说明 |
|------|------|------|
| `POST` | `/api/v1/proxy/scenarios/{id}/execute` | 执行指定场景 |
| `POST` | `/api/v1/proxy/scenarios/execute` | 临时执行（不保存，body 直接传 steps） |

执行请求支持覆盖变量：
```json
{
  "overrides": {
    "strategy_id": "xxx",
    "expected_power": 500
  }
}
```

### 3.3 执行历史

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/v1/proxy/scenarios/{id}/history` | 获取场景执行历史 |

---

## 4. 执行引擎

### 4.1 执行流程

```
for each step in scenario.steps:
    1. 检查 step.skip → 跳过
    2. 运行 PreScript（可修改变量）
    3. 替换 URL/Body/Headers 中的 {{variable}}
    4. 执行 HTTP 请求
    5. 提取 ExtractVars → 写入环境变量
    6. 运行 PostScript（断言）
    7. 记录 StepResult
    8. 等待 step.wait_ms
    9. 如果断言失败且 step 不可继续 → 停止场景
```

### 4.2 变量作用域

- **环境变量**（全局）: `{{base_url}}`、`{{token}}` 等
- **场景变量**（本次执行）: ExtractVars 提取的变量
- **覆盖变量**（执行时传入）: 优先级最高

优先级: overrides > scenario vars > environment vars

### 4.3 断言脚本 (Post-Script)

使用现有 pm sandbox，扩展以下断言 API：

```javascript
// 基本断言
pm.assert(condition, message)
pm.assertEqual(actual, expected, message)
pm.assertRange(value, min, max, message)

// 快捷访问
pm.response.status    // HTTP 状态码
pm.response.body      // 解析后的 JSON body
pm.response.headers   // 响应头
pm.vars.key           // 当前变量

// 提取变量
pm.extractVar("token", "$.data.accessToken")
pm.extractVar("status", "$.data.status")
```

### 4.4 前置脚本 (Pre-Script)

与现有 Pre-Script 一致，用于：
- 动态生成时间戳
- 计算签名
- 条件判断是否跳过后续步骤

---

## 5. 前端 UI

### 5.1 ProxyPage 左侧面板扩展

现有面板是「集合」列表，新增一个 Tab 切换：

```
┌─────────────────────────┐
│ [集合]  [场景]           │  ← Tab 切换
├─────────────────────────┤
│ 📁 EGC 策略配置          │  ← 场景文件夹
│   ▶ 并网点限功率         │
│   ▶ 储能目标跟踪         │
│   ▶ 光伏最大功率         │
│ 📁 EGC 结果验证          │
│   ▶ TSDB 查询验证        │
│   ▶ 策略状态检查         │
│ 📁 完整闭环              │
│   ▶ 登录→配置→发布→验证  │
│                          │
│ [+ 场景] [+ 文件夹]      │
└─────────────────────────┘
```

### 5.2 场景编辑器

右侧编辑区切换为场景编辑模式：

- 步骤列表（可拖拽排序）
- 每步显示：方法标签 + 名称 + URL 摘要 + 等待时间
- 点击步骤展开编辑（URL/Body/Headers/Scripts）
- 底部「执行」按钮

### 5.3 执行结果面板

执行后底部展开结果面板：

```
┌─────────────────────────────────────────┐
│ 执行结果: 并网点限功率  ✅ 4/5  ⏭ 1     │
├────┬──────┬────────┬────────┬───────────┤
│ #  │ 步骤  │ 状态   │ 耗时   │ 断言      │
├────┼──────┼────────┼────────┼───────────┤
│ 1  │ 登录  │ ✅ 200 │ 320ms  │ token提取  │
│ 2  │ 修改  │ ✅ 200 │ 150ms  │ status=ok │
│ 3  │ 发布  │ ✅ 200 │ 180ms  │ status=ok │
│ 4  │ 写入  │ ⏭ 跳过 │ -      │ -         │
│ 5  │ TSDB  │ ✅ 200 │ 450ms  │ 值在范围内 │
└────┴──────┴────────┴────────┴───────────┘
```

---

## 6. MCP 工具

新增 MCP 工具让 AI Agent 也能触发场景执行：

| 工具名 | 说明 |
|--------|------|
| `proxy_list_scenarios` | 列出所有测试场景 |
| `proxy_execute_scenario` | 执行指定场景，返回完整结果 |
| `proxy_get_scenario_history` | 获取场景执行历史 |

Agent 使用示例：
```
1. proxy_execute_scenario(name="并网点限功率", overrides={expected_power: 500})
2. 收到完整结果（包含每步的 response + assertions）
3. Agent 自行分析 TSDB 查询结果，判断 EGC 控制是否符合预期
4. 生成测试报告
```

---

## 7. 预置 EGC 测试场景模板

作为内置资源提供 JSON 模板（类似 `gridsim-builtin.postman_collection.json`）：

### 场景 1: EnOS 登录

```json
{
  "name": "EnOS 登录",
  "steps": [
    {
      "name": "获取 Access Token",
      "method": "POST",
      "url": "{{enos_api_url}}/login",
      "headers": { "Content-Type": "application/json" },
      "body": "{\"username\": \"{{enos_user}}\", \"password\": \"{{enos_password}}\"}",
      "post_script": "pm.assert(response.status === 200); pm.extractVar('token', '$.data.accessToken');",
      "extract_vars": { "token": "$.data.accessToken" }
    }
  ]
}
```

### 场景 2: EGC 策略配置

```json
{
  "name": "EGC 策略配置",
  "steps": [
    {
      "name": "登录（提取 token）",
      "method": "POST",
      "url": "{{enos_api_url}}/login",
      "...": "...",
      "extract_vars": { "token": "$.data.accessToken" }
    },
    {
      "name": "修改控制策略",
      "method": "PUT",
      "url": "{{enos_api_url}}/strategy/config",
      "headers": { "Authorization": "Bearer {{token}}" },
      "body": "{{strategy_payload}}",
      "post_script": "pm.assert(response.status === 200);"
    },
    {
      "name": "发布策略",
      "method": "POST",
      "url": "{{enos_api_url}}/strategy/publish",
      "headers": { "Authorization": "Bearer {{token}}" },
      "wait_ms": 5000,
      "post_script": "pm.assert(response.status === 200);"
    }
  ]
}
```

### 场景 3: TSDB 结果验证

```json
{
  "name": "TSDB 结果验证",
  "steps": [
    {
      "name": "登录",
      "...": "..."
    },
    {
      "name": "查询 TSDB 数据",
      "method": "POST",
      "url": "{{enos_tsdb_url}}/query",
      "headers": { "Authorization": "Bearer {{token}}" },
      "body": "{\"assetId\": \"{{asset_id}}\", \"pointIds\": [\"{{ao_point}}\"], \"timeRange\": \"{{time_range}}\"}",
      "post_script": "pm.assert(response.status === 200); let values = response.body.data; pm.assert(values.length > 0, '有数据返回'); pm.assertRange(values[0].value, {{min_value}}, {{max_value}}, 'AO值在预期范围内');"
    },
    {
      "name": "查看策略状态",
      "method": "GET",
      "url": "{{enos_api_url}}/strategy/status",
      "headers": { "Authorization": "Bearer {{token}}" },
      "post_script": "pm.assert(response.status === 200); pm.assertEqual(response.body.status, 'running', '策略运行中');"
    }
  ]
}
```

---

## 8. 文件变更清单

| 文件 | 变更 |
|------|------|
| `pkg/api/proxy_store.go` | 新增 TestScenario/Step 结构，ProxyStore 增加 Scenarios 字段和 CRUD 方法 |
| `pkg/api/proxy_handler.go` | 新增场景 CRUD + 执行 API handler |
| `cmd/gridsim/main.go` | 注册新路由 |
| `internal/mcp/server.go` | 新增 3 个 MCP 工具 |
| `web/src/views/ProxyPage.vue` | 左侧面板新增「场景」Tab，场景编辑器，执行结果面板 |
| `web/src/api/index.ts` | 新增场景相关 API 调用 |
| `cmd/gridsim/resources/` | 内置 EGC 场景模板 JSON |

---

## 9. 用户确认的设计决策

基于需求讨论，确认以下决策：

1. **双触发模式**: 前端手动触发 + MCP 工具触发，两者共用同一套执行引擎
2. **存档与复用**: 场景执行后自动存档（含变量快照、执行结果），回归测试时 AI Agent 可通过 MCP 直接调用已存档场景
3. **接口创建**: 提供创建接口/文件夹/变量的 API（后端已有），AI Agent 通过 MCP 工具自行创建接口，无需预设模板
4. **断言模式**: 混合模式（脚本断言 + AI 自由判断）

## 10. 实施优先级

### Phase 1: 核心场景执行（本次实现）

1. ~~Bug fix: 新建请求只创建到第一个文件夹~~ ✅ 已修复
2. ~~拖拽排序: 请求可拖拽到其他文件夹~~ ✅ 已实现
3. 后端数据模型 + 存储（TestScenario/Step）
4. 场景 CRUD API
5. 场景执行引擎（变量替换 + 请求执行 + 脚本断言 + 变量提取）
6. 执行存档（自动保存每次执行结果）
7. 前端场景 Tab + 步骤编辑器 + 执行按钮 + 结果面板
8. MCP 工具（场景 CRUD + 执行 + 查询历史）

### Phase 2: MCP + 历史（后续）

1. MCP 工具（proxy_list_scenarios / proxy_execute_scenario）
2. 执行历史存储与查询
3. 场景导入/导出（Postman 格式兼容）

### Phase 3: 高级能力（后续）

1. 场景编排（场景 A 的输出 → 场景 B 的输入）
2. 定时执行
3. 报告生成与推送

---

> **EGC 场景自动化测试 — 接口测试扩展设计 · v1.0**
