# IEC104 客户端模式设计

> 日期: 2026-06-21 | 版本: v3.2.0 | 状态: 设计中

## 1. 概述

GridSim 目前仅支持 IEC104 **服务端（从站/slave）** 模式，即模拟 RTU 等待 SCADA 主站连接。本设计新增 IEC104 **客户端（主站/master）** 模式，使 GridSim 能作为主站连接真实的 IEC104 从站设备，实现：

- 接收远端从站的遥测（YC/AI）、遥信（YX/DI）、遥脉（YM/PI）数据
- 向远端从站发送遥控（DO）、遥调（AO）命令
- 统一的 Web 管理界面管理客户端实例
- 复用现有的点表配置、自动变化引擎、实时数据库

### 用户确认的需求

| 需求 | 选择 | 说明 |
|------|------|------|
| 控制命令 | ✅ 完整读写 | 支持 DO 遥控 + AO 遥调 |
| 实例管理 | ✅ 一对一 | 一个客户端实例 = 一个远端设备（IP:Port + 站址） |
| 前端 UI | ✅ 一次到位 | 后端 API + Web UI 本次全部实现 |

---

## 2. 架构方案

采用 **方案 A：Client Wrapper + Protocol 复用**。

### 核心思路

`Protocol` 接口语义天然兼容客户端模式，零改动复用：

```
Protocol 接口方法    服务端语义             客户端语义
──────────────────────────────────────────────────────
ClientConnected()    客户端是否已连接        是否已连接到远端
ClientAddr()         已连接客户端地址        远端服务器地址
Publish(point)       变化上送给客户端        发送遥控/遥调命令给远端
SetStore(store)      设置本地点表            设置本地点表
Stats()/Uptime()     运行统计               运行统计
```

新增 `pkg/iec104/client.go` 实现 `Protocol` 接口，内部使用 go-iecp5 的 `cs104.Client`（已在 `go.mod` 中）。

### 架构图

```
┌──────────────────────────────────────────────────────────────┐
│                      Manager (internal/manager/)              │
│                                                              │
│  Instance {                                                  │
│    Config: InstanceConfig,                                   │
│    Protocol: Protocol,  ← IEC104ClientWrapper 或 IEC104Wrapper│
│    Store: *library.Store,  ← 本地点表（与服务器模式共享）      │
│    AutoEngine: *detail.Engine, ← 自动变化引擎（复用）         │
│    Logger: *InstanceLogger                                   │
│  }                                                           │
└──────────────────────────────────────────────────────────────┘
         │                             ▲
         ▼                             │
┌─────────────────┐          ┌──────────────────┐
│  IEC104Client    │          │  Web UI + HTTP   │
│  (cs104.Client)  │          │  API (cmd/main)  │
│                  │          │                  │
│  ← 接收 M_ME/M_SP │          │  PUT /command →  │
│  → 发送 C_SC/C_SE │          │  DO/AO 操作      │
└────────┬─────────┘          └──────────────────┘
         │ TCP/IP
         ▼
┌─────────────────┐
│  远端 IEC104     │
│  从站设备        │
└─────────────────┘
```

---

## 3. 详细设计

### 3.1 数据模型

**`internal/model/instance.go`** — 新增客户端配置：

```go
// IEC104ClientConfig 客户端实例配置
type IEC104ClientConfig struct {
    RemoteAddr     string `json:"remote_addr"`      // 远端 IP/域名
    RemotePort     int    `json:"remote_port"`       // 远端端口，默认 2404
    CommonAddr     int    `json:"common_addr"`       // 公共地址，默认 1
    ReconnectDelay int    `json:"reconnect_delay"`   // 重连间隔(秒)，默认 5
    ConnectTimeout int    `json:"connect_timeout"`   // 连接超时(秒)，默认 10
}

// InstanceConfig 扩展
type InstanceConfig struct {
    // ... 现有字段 ...
    IEC104ClientConfig *IEC104ClientConfig `json:"iec104_client_config,omitempty"`
}
```

`InstanceState` 无需改动 — `ClientConnected` 在客户端模式下表示"已连接到远端"。

**`internal/model/enum.go`** — 新增协议类型（如果有枚举定义）：

```go
// 扩展 InstanceType
const (
    TypeServer   = "iec104"
    TypeClient   = "iec104_client"
    TypeModbus   = "modbus_tcp"
    TypeMicrogrid = "microgrid"
)
```

### 3.2 客户端包装器

**新建 `pkg/iec104/client.go`**:

```go
package iec104

// Client IEC104 客户端，连接远端从站
type Client struct {
    config  model.IEC104ClientConfig
    store   *library.Store
    client  *cs104.Client
    // 状态
    mu         sync.RWMutex
    connected  bool
    startTime  time.Time
    // 统计
    interrogCnt atomic.Int64
    controlCnt  atomic.Int64
    spontCnt    atomic.Int64
}

func NewClient(cfg model.IEC104ClientConfig) *Client { ... }

// Protocol 接口实现

func (c *Client) Name() string              { return "iec104_client" }
func (c *Client) Start() error              { /* 连接远端，发送 STARTDT，发起总召 */ }
func (c *Client) Stop()                     { /* 断开连接 */ }
func (c *Client) ClientConnected() bool     { return c.connected }
func (c *Client) ClientAddr() string        { return fmt.Sprintf("%s:%d", c.config.RemoteAddr, c.config.RemotePort) }
func (c *Client) Stats() (int64, int64, int64) { return c.interrogCnt.Load(), c.controlCnt.Load(), c.spontCnt.Load() }
func (c *Client) Uptime() int64             { return int64(time.Since(c.startTime).Seconds()) }
func (c *Client) Publish(point *config.Point) { /* DO/AO → 发送命令到远端 */ }
func (c *Client) SetStore(store *library.Store) { c.store = store }
```

#### 3.2.1 连接流程 (`Start()`)

1. 解析远端地址: `tcp://{remote_addr}:{remote_port}`
2. 创建 `cs104.ClientOption`:
   - `SetAutoReconnect(true)`
   - `SetReconnectInterval(config.ReconnectDelay * time.Second)`
   - `AddRemoteServer(addr)`
3. 创建 `ClientHandler` 实现 `cs104.ClientHandlerInterface`
4. 创建 `cs104.NewClient(handler, option)`
5. 设置连接回调 `SetOnConnectHandler`:
   - 连接建立 → 发送 `SendStartDt()`
   - STARTDT 确认 → 发送总召唤 `InterrogationCmd()`
6. 设置断开回调 `SetConnectionLostHandler`:
   - 标记 `connected = false`
7. 调用 `client.Start()` → 异步连接

#### 3.2.2 数据接收 (`ASDUHandler`)

当远端从站发送数据时，`ClientHandler.ASDUHandler` 被调用：

| ASDU 类型 | 处理方式 |
|-----------|---------|
| M_ME_NC_1 (遥测 AI) | `store.SetValue(ioa, float64(value))` |
| M_SP_NA_1 (遥信 DI) | `store.SetBoolValue(ioa, value)` |
| M_IT_NA_1 (遥脉 PI) | `store.SetIntValue(ioa, value)` |
| C_SC_NA_1 ACT_CON/TERM | 记录控制执行结果 |
| C_SE_NC_1 ACT_CON/TERM | 记录控制执行结果 |
| 总召唤响应 | 增加 interrogCnt |
| 其他 | 记录日志 |

数据写入 Store 后，自动变化引擎托管的点会被通知到（通过 `Publish()` 回调），但客户端模式下不应将引擎变化发回远端。

#### 3.2.3 命令发送 (`Publish()`)

仅 DO/AO 点会触发命令发送：

```go
func (c *Client) Publish(point *config.Point) {
    if !c.connected || c.client == nil {
        return
    }
    
    coa := asdu.CauseOfTransmission{Cause: asdu.Activation}
    ca := asdu.CommonAddr(c.config.CommonAddr)
    
    switch point.PointType {
    case config.TypeDO:
        cmd := asdu.SingleCommand{
            Ioa:   asdu.InfoObjAddr(point.IOA),
            Value: point.BoolValue,
        }
        asdu.SingleCommand(c.client, false, coa, ca, cmd)
        c.controlCnt.Add(1)
        
    case config.TypeAO:
        cmd := asdu.SetpointCommandFloat{
            Ioa:   asdu.InfoObjAddr(point.IOA),
            Value: float32(point.Value),
        }
        asdu.SetpointCommandFloat(c.client, false, coa, ca, cmd)
        c.controlCnt.Add(1)
    }
}
```

### 3.3 工厂扩展

**`pkg/protocol/factory.go`**:

```go
func New(cfg model.InstanceConfig) (Protocol, error) {
    switch cfg.Protocol {
    case "modbus_tcp":
        // ... 现有逻辑 ...
    case "microgrid":
        return NewIEC104Wrapper(cfg.IEC104Port), nil
    case "iec104_client":
        if cfg.IEC104ClientConfig == nil {
            return nil, fmt.Errorf("iec104_client_config required for client mode")
        }
        return iec104.NewClient(*cfg.IEC104ClientConfig), nil
    case "", "iec104":
        return NewIEC104Wrapper(cfg.IEC104Port), nil
    default:
        return nil, fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
    }
}

func SupportedProtocols() []string {
    return []string{"iec104", "modbus_tcp", "microgrid", "iec104_client"}
}
```

### 3.4 Manager 适配

**`internal/manager/manager.go`** — `StartInstance()` 分支：

```go
func (m *Manager) StartInstance(id string) error {
    cfg, ok := m.store.Get(id)
    // ...
    
    // 客户端模式不走端口监听
    if cfg.Protocol != "iec104_client" {
        // 现有端口检查 + 监听验证逻辑
        // firewall.EnsurePort(...)
    }
    
    // XLSX 加载（客户端模式也需要点表定义）
    points, err := config.LoadFromXLSX(xlsxPath, cfg.Protocol)
    
    store := library.NewStore(points)
    
    proto, err := protocol.New(cfg)
    proto.SetStore(store)
    if err := proto.Start(); err != nil {
        return fmt.Errorf("start protocol: %w", err)
    }
    
    // 自动变化引擎（复用，客户端模式下引擎 DO/AO 会通过 Publish 发送到远端）
    acStore := detail.NewAutoChangeStore(m.cfgDir)
    engine := detail.NewEngine(cfg.ID, store, proto, acStore, m.cfgDir, m)
    // 注意：客户端模式不设置 AOFollowHandler（远端不需要跟随）
    if err := engine.LoadAndStart(); err != nil {
        slog.Warn("自动变化引擎加载失败", "id", id, "error", err)
    }
    
    // Logger + Instance 创建（复用）
    inst := &Instance{
        Config:     cfg,
        Protocol:   proto,
        Store:      store,
        AutoEngine: engine,
        Logger:     logger,
    }
    
    // 客户端模式不启动单独的 HTTP API（不走 cfg.HttpEnabled）
    // 所有操作通过主管理 API
    
    m.instances[id] = inst
    return nil
}
```

**`StopInstance()`** 调整：

```go
func (m *Manager) StopInstance(id string) error {
    // ...
    inst.Protocol.Stop()
    // ...
    if inst.Config.Protocol != "iec104_client" {
        firewall.RemovePort(inst.Config.IEC104Port)  // 客户端无端口
    }
    // ...
}
```

**`GetState()` / `ListStates()`** — `ClientConnected` 语义已兼容，无需改动。

### 3.5 HTTP API 扩展

**`cmd/gridsim/main.go`** — 新增控制命令端点：

```go
// 在 handleInstanceByID 的 parts[1] 分支中新增：
case "command":
    ws.handleClientCommand(w, r, id)
```

新端点：

```
PUT /api/v1/instances/{id}/command
Content-Type: application/json

{
    "ioa": 16385,         // 信息体地址
    "value": 235.5,       // AO 遥调值（float）
    "bool_value": true    // DO 遥控值（bool，与 value 二选一）
}
```

响应：

```json
{
    "success": true,
    "ioa": 16385,
    "value": 235.5
}
```

实现：

```go
func (ws *webServer) handleClientCommand(w http.ResponseWriter, r *http.Request, id string) {
    if r.Method != http.MethodPut {
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }
    
    inst := ws.mgr.GetInstance(id)  // 新增方法
    if inst == nil || inst.Protocol == nil {
        writeError(w, http.StatusNotFound, "instance not running")
        return
    }
    
    var body struct {
        IOA       uint32   `json:"ioa"`
        Value     *float64 `json:"value"`
        BoolValue *bool    `json:"bool_value"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON")
        return
    }
    
    pt, ok := inst.Store.Get(body.IOA)
    if !ok {
        writeError(w, http.StatusNotFound, "point not found")
        return
    }
    
    switch pt.PointType {
    case config.TypeDO:
        if body.BoolValue == nil {
            writeError(w, http.StatusBadRequest, "bool_value required for DO")
            return
        }
        inst.Store.SetBoolValue(body.IOA, *body.BoolValue)
        inst.Protocol.Publish(pt)  // → 发送到远端
    
    case config.TypeAO:
        if body.Value == nil {
            writeError(w, http.StatusBadRequest, "value required for AO")
            return
        }
        inst.Store.SetValue(body.IOA, *body.Value)
        inst.Protocol.Publish(pt)  // → 发送到远端
    
    default:
        writeError(w, http.StatusBadRequest, "only DO/AO supported for commands")
        return
    }
    
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "success": true, "ioa": body.IOA,
    })
}
```

**Manager 新增 `GetInstance()` 方法**：

```go
func (m *Manager) GetInstance(id string) *Instance {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.instances[id]
}
```

### 3.6 Dashboard 适配

`DashboardData` 中 `ClientsConnected` 对客户端模式应表示"已连接到远端的实例数"。现有 `GetDashboardData()` 逻辑已然兼容：

```go
if s.ClientConnected {
    data.ClientsConnected++
}
```

客户端模式下 `s.ClientConnected = proto.ClientConnected()` 返回的是"是否已连接到远端"，语义正确。

**样式区分** — 前端 / Dashboard 需要视觉区分服务端和客户端实例（见 3.7）。

### 3.7 Web 前端设计

#### 3.7.1 配置文件（i18n）

**`web/src/locales/zh-CN/instance.js`** 新增：

```js
export default {
  protocol: {
    iec104: 'IEC104 服务端',
    iec104_client: 'IEC104 客户端',
    modbus_tcp: 'Modbus TCP',
    microgrid: '微电网',
  },
  clientConfig: {
    title: '客户端配置',
    remoteAddr: '远端地址',
    remoteAddrPlaceholder: '如 192.168.1.100',
    remotePort: '远端端口',
    commonAddr: '公共地址',
    reconnectDelay: '重连间隔(秒)',
    connectTimeout: '连接超时(秒)',
    connected: '已连接',
    disconnected: '未连接',
    command: '控制命令',
    sendCommand: '发送',
    commandSent: '命令已发送',
  },
  // ... 现有字段 ...
}
```

**`web/src/locales/en/instance.js`** 对应英文。

#### 3.7.2 实例列表页 (`ConfigPage.vue`)

- 协议标签新增 `iec104_client` 紫色/橙色 badge
- 实例卡片/表格新增"连接状态"列：🟢 已连接 / 🔴 未连接（仅客户端模式）
- 操作列新增"发送命令"按钮（仅客户端模式运行中）

#### 3.7.3 实例创建/编辑表单

`protocol` 下拉选择新增 `"iec104_client"` 选项。

选择 `"iec104_client"` 时，展开客户端配置卡片：

```
┌─ 客户端配置 ─────────────────────────────┐
│  远端地址    [192.168.1.100           ]   │
│  远端端口    [2404                    ]   │
│  公共地址    [1                       ]   │
│  重连间隔    [5           ] 秒            │
│  连接超时    [10          ] 秒            │
│                                          │
│  点表文件    [选择 .xlsx ...]             │
│  （用于定义该客户端监视的测点列表）        │
└──────────────────────────────────────────┘
```

点表文件（XLSX）在客户端模式下定义客户端**期望监视的测点列表**。连接建立后，客户端发总召唤获取远端数据填入这些测点。用户可以为此点表配置自动变化策略。

#### 3.7.4 实例详情页 (`DetailPage.vue`)

- **协议 badge**：显示 "IEC104 客户端" + 连接状态指示灯
- **连接状态卡片**：显示已连接时长、远端地址
- **数据表格**：复用现有点表实时数据组件，显示从远端接收到的数据
- **控制命令面板**（新增）：
  - 表格中 DO 点行显示 ON/OFF 按钮，点击发送遥控
  - 表格中 AO 点行显示输入框 + 发送按钮，点击发送遥调
  - 发送后显示 "命令已发送" 绿色提示

#### 3.7.5 Dashboard 页

- 客户端实例归入 `by_protocol` 统计（自动）
- 实例概览卡片上显示协议 badge，客户端模式时 badge 显示 "客户端"
- 状态列显示"已连接"代替"客户端在线"

---

## 4. 影响范围与改动清单

### 4.1 后端改动

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `internal/model/instance.go` | ✅ 新增 | `IEC104ClientConfig` 结构体 |
| `pkg/iec104/client.go` | ✅ 新建 | 客户端包装器，~200 行 |
| `pkg/protocol/factory.go` | ✅ 修改 | 新增 `"iec104_client"` 分支，~5 行 |
| `internal/manager/manager.go` | ✅ 修改 | Start/Stop 分支适配，~30 行 |
| `cmd/gridsim/main.go` | ✅ 修改 | 新增 `/command` 端点 + handler，~50 行 |
| `pkg/api/handler.go` | ❌ 不改 | 实例级 API 仅传统模式使用 |

### 4.2 前端改动

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `web/src/locales/zh-CN/instance.js` | ✅ 修改 | 新增客户端模式翻译 |
| `web/src/locales/en/instance.js` | ✅ 修改 | 新增客户端模式翻译 |
| `web/src/views/ConfigPage.vue` | ✅ 修改 | 协议下拉 + 客户端配置表单 |
| `web/src/views/DetailPage.vue` | ✅ 修改 | 连接状态 + 控制命令面板 |
| `web/src/components/instance/ClientConfigForm.vue` | ✅ 新建 | 客户端配置表单组件 |
| `web/src/components/instance/CommandPanel.vue` | ✅ 新建 | 控制命令面板组件 |

### 4.3 零改动文件

| 文件 | 原因 |
|------|------|
| `pkg/protocol/protocol.go` | 接口语义兼容 |
| `pkg/protocol/iec104_wrapper.go` | 服务端包装器不变 |
| `pkg/iec104/server.go` | 服务端实现不变 |
| `internal/store/store.go` | 存储层复用 |
| `internal/detail/engine.go` | 自动变化引擎复用 |
| `internal/storage/` | 配置持久化不变 |
| `internal/microgrid/` | 微电网模块不变 |
| `web/src/views/DashboardPage.vue` | 统计兼容 |

---

## 5. 边界条件

### 5.1 连接管理

| 场景 | 行为 |
|------|------|
| 远端不可达 | 按 `reconnect_delay` 间隔自动重连，状态 = 未连接 |
| 连接中断后恢复 | 自动重连 → STARTDT → 总召 → 恢复数据同步 |
| 实例停止 | 断开连接，停止重连 |
| 实例删除 | 断开连接，清除状态 |

### 5.2 数据一致性

- **首次连接**：自动发送总召唤（QOI=20），同步全部数据
- **连接中断**：重连后再次总召，确保数据完整
- **变化上送**：远端从站的变化上送（COT=3）正常接收，实时更新 Store

### 5.3 命令执行

- **发送 DO**：构造 C_SC_NA_1 ASDU，使用 `Activation` 原因 → 远端返回 ACT_CON + ACT_TERM 表示执行成功
- **发送 AO**：构造 C_SE_NC_1 ASDU，使用 `Activation` 原因 → 远端返回 ACT_CON + ACT_TERM
- **远端无响应**：命令丢弃，日志警告（不阻塞）

### 5.4 自动变化引擎

- 客户端模式下，引擎仍可在本地 Store 上运行（模拟 AI/DI 变化）
- **引擎的 DO/AO 变化不会发到远端**（`Publish` 仅在 Store 被同一进程的本地修改触发时调用，引擎修改本地值不会触发 `Publish`）
- 只有用户通过 HTTP API 或 UI "发送命令"才会通过 `Publish()` 实际发送命令到远端

---

## 6. 实施建议

### Phase 1：后端核心（4h）
1. 新增 `IEC104ClientConfig` 模型
2. 实现 `pkg/iec104/client.go`（连接、接收、总召、Publish 发送）
3. 扩展 `factory.go` + `manager.go`
4. 新增 `/command` API 端点

### Phase 2：前端 UI（4h）
1. i18n 翻译
2. `ClientConfigForm.vue` 配置表单组件
3. `ConfigPage.vue` 协议选择 + 客户端表单集成
4. `CommandPanel.vue` 控制面板组件
5. `DetailPage.vue` 客户端状态展示

### Phase 3：验证（1h）
1. `make dist` 全平台构建
2. 手动测试：连接真实从站 / 连接 GridSim 服务端实例
3. 遥测遥信接收验证
4. 遥控遥调发送验证
5. 断线重连验证

总计约 **9h**。
