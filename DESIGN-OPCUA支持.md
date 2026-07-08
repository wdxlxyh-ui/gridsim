# GridSim OPC UA Server 模拟器集成方案

> 版本：草案 v0.1 ｜ 适用工程：`gridsim_dev` ｜ Go 1.25 / Vue 3
> 目标：在现有多规约架构上新增 `opcua_server` 规约，模拟一个 OPC UA Server，复用现有点表、Store、自动变化引擎、HTTP API 与前端，做到与 IEC104 / Modbus 同级的"一等公民"。

---

## 1. 需求与定位

当前 GridSim 已支持 IEC104（服务端/客户端）、Modbus TCP、微电网四类规约，统一抽象为 `protocol.Protocol` 接口并由工厂创建（见 `pkg/protocol/factory.go`）。本方案新增 **OPC UA Server 模拟器**：

- 对外暴露一个标准 OPC UA Server（`opc.tcp://host:4840`），供测试主站 / SCADA / 边缘网关（如 EGC）作为 OPC UA Client 连接。
- 点表中的每个测点（AI/DI/PI/DO/AO）映射为 OPC UA 地址空间中的一个 Variable 节点。
- 支持 Client 的 Read / Write / Browse / Subscription（数据变化订阅）。
- 写操作（遥控/遥调）回写到 `library.Store`，与自动变化引擎、HTTP API 双向打通。

非目标（本期不做）：OPC UA Client（主站采集）模式、安全策略加密（先只做 None + 匿名）、历史数据 HistoryRead、Method 调用。这些列入后续迭代。

---

## 2. 开源库选型

### 2.1 候选对比

| 库 | 语言 | License | Server 支持 | 维护状态 | 评价 |
|----|------|---------|------------|---------|------|
| **[gopcua/opcua](https://github.com/gopcua/opcua)** | Go | MIT | 是（Read/Write/Browse/Subscription/MonitoredItems）| 活跃，Northvolt/evosoft 赞助，生产使用 | **首选**，纯 Go、零 CGO，与现有工程同语言同构 |
| [ASNeG/OpcUaStack](https://github.com/ASNeG/OpcUaStack) | C++ | Apache-2.0 | 完整 | 活跃 | 功能全但需 CGO/独立进程，跨平台编译与打包复杂 |
| open62541 | C | MPL-2.0 | 完整 | 活跃 | 同上，C 库，集成成本高 |
| node-opcua | Node.js | MIT | 完整 | 活跃 | 需 Node 运行时，与 Go 单二进制部署模型冲突 |

> 内容已按授权要求改写，详情以各项目官方仓库为准。

### 2.2 结论：选用 gopcua/opcua

理由：

1. **纯 Go 实现**，无 CGO，可直接 `go get`，沿用现有 `Makefile` + UPX 单二进制打包，跨平台（Linux/Windows）零额外依赖。
2. **MIT 协议**，与商业/内部测试工具兼容。
3. 内置 `server` 包，已实现 OPC UA Binary over TCP 的安全通道、会话、Read/Write/Browse/订阅，覆盖模拟器所需的全部核心服务。
4. 提供 **`server.MapNamespace`**：以 `map[string]any` 为后端的命名空间，自带变化通知与外部写入通知通道，**几乎与我们的 `Store` 一一对应**，集成量最小。

### 2.3 gopcua Server 关键 API（已核对官方 `examples/server/map_server`）

```go
import (
    "github.com/gopcua/opcua/server"
    "github.com/gopcua/opcua/ua"
    "github.com/gopcua/opcua/id"
)

// 1. 创建 server
s := server.New(
    server.EndPoint("0.0.0.0", 4840),
    server.EnableSecurity("None", ua.MessageSecurityModeNone),
    server.EnableAuthMode(ua.UserTokenTypeAnonymous),
    server.SetLogger(logger),
)

// 2. 创建 map 命名空间（后端是 map[string]any）
ns := server.NewMapNamespace(s, "GridSim")
ns.Data["AI.4001"] = 123.4          // 写值
ns.ChangeNotification("AI.4001")    // 触发订阅推送（需先手动加锁）
ns.SetValue("DO.5001", true)        // 封装版：自动加锁 + 通知

// 3. 监听 Client 写入（遥控/遥调回写）
go func() {
    for key := range ns.ExternalNotification {
        v := ns.GetValue(key)       // Client 写进来的新值
        // → 回写 library.Store
    }
}()

// 4. 挂到根对象节点，使其可被 Browse
rootNS, _ := s.Namespace(0)
rootNS.Objects().AddRef(ns.Objects(), id.HasComponent, true)

// 5. 启停
s.Start(context.Background())
defer s.Close()
```

`MapNamespace` 提供的能力正好满足模拟器：`Data`/`Mu`/`ChangeNotification`/`SetValue`/`GetValue`/`ExternalNotification`/`Objects()`/`ID()`。

---

## 3. 整体架构

沿用多规约的"协议抽象层 + Store 复用"原则，OPC UA Server 作为 `Protocol` 接口的又一实现：

```
+------------------------------------------------------------------+
|                       Manager (多实例管理)                         |
|  +------------------+  +------------------+  +-----------------+  |
|  | Instance IEC104  |  | Instance Modbus  |  | Instance OPCUA  |  |
|  | +------------+   |  | +------------+   |  | +-----------+   |  |
|  | |IEC104 Srv  |   |  | |Modbus Srv  |   |  | |OPCUA Srv  |   |  |
|  | +-----+------+   |  | +-----+------+   |  | +-----+-----+   |  |
|  |       |          |  |       |          |  |       |         |  |
|  |  +----v----+     |  |  +----v----+     |  |  +----v----+    |  |
|  |  |  Store  |     |  |  |  Store  |     |  |  |  Store  |    |  |
|  |  +---------+     |  |  +---------+     |  |  +---------+    |  |
|  +------------------+  +------------------+  +-----------------+  |
|              共享：自动变化引擎 / HTTP API / 点表加载              |
+------------------------------------------------------------------+
|                    HTTP API + Web UI (Vue 3)                      |
+------------------------------------------------------------------+
```

数据流：

- **读 / 订阅**：自动变化引擎或 HTTP API 改写 `Store` → OPCUA Wrapper 同步到 `MapNamespace` 并 `ChangeNotification` → Client 收到订阅推送。
- **写（遥控/遥调）**：Client 写节点 → `MapNamespace.ExternalNotification` → Wrapper 回写 `Store.SetValue/SetBoolValue` → AO 跟随、HTTP API 可见。

---

## 4. 后端设计

### 4.1 目录结构

```
pkg/protocol/
  protocol.go          （不变）Protocol 接口
  factory.go           （改）新增 case "opcua_server"
  opcua/
    server.go          OPCUAServer：实现 Protocol 接口，封装 gopcua server
    nodemap.go         Point ↔ OPC UA NodeID/数据类型 映射规则
    sync.go            Store→Namespace 同步 + ExternalNotification 回写
```

### 4.2 Protocol 接口实现

OPC UA Server 需实现现有接口（见 `pkg/protocol/protocol.go`）：

```go
type Protocol interface {
    Name() string
    Start() error
    Stop()
    ClientConnected() bool
    ClientAddr() string
    Stats() (interrog, control, spont int64)
    Uptime() int64
    Publish(point *config.Point)
    SetStore(store *library.Store)
}
```

骨架（`pkg/protocol/opcua/server.go`）：

```go
package opcua

import (
    "context"
    "fmt"
    "sync"
    "sync/atomic"
    "time"

    "github.com/gopcua/opcua/id"
    "github.com/gopcua/opcua/server"
    "github.com/gopcua/opcua/ua"

    "gridsim/pkg/config"
    "gridsim/pkg/library"
)

type OPCUAServer struct {
    port      int
    nsURI     string
    store     *library.Store

    srv  *server.Server
    ns   *server.MapNamespace
    ctx  context.Context
    stop context.CancelFunc

    startTime   time.Time
    interrogCnt atomic.Int64 // 读次数（可选统计）
    controlCnt  atomic.Int64 // 写次数
    spontCnt    atomic.Int64 // 变化推送次数

    connMu    sync.RWMutex
    connected bool
    clientAddr string
}

func NewServer(port int, nsURI string) *OPCUAServer {
    if port == 0 { port = 4840 }
    if nsURI == "" { nsURI = "GridSim" }
    return &OPCUAServer{port: port, nsURI: nsURI}
}

func (o *OPCUAServer) Name() string { return "opcua_server" }
func (o *OPCUAServer) SetStore(s *library.Store) { o.store = s }

func (o *OPCUAServer) Start() error {
    o.startTime = time.Now()
    o.ctx, o.stop = context.WithCancel(context.Background())

    o.srv = server.New(
        server.EndPoint("0.0.0.0", o.port),
        server.EndPoint("localhost", o.port),
        server.EnableSecurity("None", ua.MessageSecurityModeNone),
        server.EnableAuthMode(ua.UserTokenTypeAnonymous),
    )

    o.ns = server.NewMapNamespace(o.srv, o.nsURI)

    // 初始化：把 Store 所有点写入命名空间
    for _, p := range o.store.GetAll() {
        o.ns.Data[NodeKey(p)] = pointToVariant(p)
    }
    rootNS, _ := o.srv.Namespace(0)
    rootNS.Objects().AddRef(o.ns.Objects(), id.HasComponent, true)

    if err := o.srv.Start(o.ctx); err != nil {
        return fmt.Errorf("start opcua server: %w", err)
    }

    go o.watchExternalWrites() // Client 写回写 Store
    return nil
}

func (o *OPCUAServer) Stop() {
    if o.stop != nil { o.stop() }
    if o.srv != nil { o.srv.Close() }
    o.connMu.Lock(); o.connected = false; o.connMu.Unlock()
}
```

### 4.3 点表 ↔ 节点映射（`nodemap.go`）

为避免不同测点类型复用同一 IOA 造成键冲突，**节点 Key 用 `类型.IOA` 复合键**（与 Store 内 `byType` 多类型共存的设计一致）：

| Point 字段 | OPC UA 映射 | 说明 |
|-----------|-------------|------|
| `PointType` + `IOA` | NodeID（BrowseName）`AI.4001` | 复合键，浏览路径清晰 |
| `Name` / `Alias` | DisplayName / Description | 人类可读 |
| `AI` / `AO` | `Double` | `Point.Value` |
| `PI` | `Int32` | `Point.IntValue` |
| `DI` / `DO` | `Boolean` | `Point.BoolValue` |

```go
func NodeKey(p *config.Point) string {
    return fmt.Sprintf("%s.%d", p.PointType, p.IOA)
}

func pointToVariant(p *config.Point) any {
    switch p.PointType {
    case config.TypeAI, config.TypeAO:
        return p.Value
    case config.TypePI:
        return p.IntValue
    case config.TypeDI, config.TypeDO:
        return p.BoolValue
    }
    return p.Value
}
```

> 可选增强：第二期支持 OPC UA 标准的层级 Browse（按 Group 分文件夹），用 `NodeNamespace` 替代 `MapNamespace` 构造 Object/Variable 树。本期先用扁平 `MapNamespace` 快速落地。

### 4.4 Store → Namespace 同步（`sync.go`）

复用现有机制：自动变化引擎和 HTTP API 写 `Store` 后，会调用 `Protocol.Publish(point)`（见 `manager.go` 引擎装配）。在 `Publish` 中把单点同步到命名空间并触发订阅推送：

```go
func (o *OPCUAServer) Publish(p *config.Point) {
    if o.ns == nil { return }
    o.ns.SetValue(NodeKey(p), pointToVariant(p)) // 内部加锁 + ChangeNotification
    o.spontCnt.Add(1)
}
```

> `SetValue` 已封装加锁与变化通知，Client 订阅即可实时收到。无需额外轮询。

### 4.5 Client 写回写 Store（遥控/遥调）

监听 `MapNamespace.ExternalNotification`，将 Client 写入的值落回 `Store`，从而触发 AO 跟随、被 HTTP API 读到：

```go
func (o *OPCUAServer) watchExternalWrites() {
    for {
        select {
        case <-o.ctx.Done():
            return
        case key := <-o.ns.ExternalNotification:
            pt, ok := parseNodeKey(o.store, key) // 反查 IOA + 类型
            if !ok { continue }
            v := o.ns.GetValue(key)
            switch pt.PointType {
            case config.TypeAO, config.TypeAI:
                if f, ok := toFloat64(v); ok { o.store.SetValue(pt.IOA, f) }
            case config.TypePI:
                if i, ok := toInt32(v); ok { o.store.SetIntValue(pt.IOA, i) }
            case config.TypeDO, config.TypeDI:
                if b, ok := v.(bool); ok { o.store.SetBoolValue(pt.IOA, b) }
            }
            o.controlCnt.Add(1)
        }
    }
}
```

> 与 Modbus 的 `SetAOFollowHandler` 一致：写 AO 后由自动变化引擎处理 AO→AI 跟随逻辑。OPCUAServer 也实现 `SetAOFollowHandler(func(uint32))` 接口，在写 AO 时回调引擎（见 `manager.StartInstance` 中的可选断言）。

### 4.6 连接状态统计

gopcua server 暂未直接暴露"当前连接客户端"回调，状态可用两种方式：

- **简化**：`ClientConnected()` 在 server 成功 `Start` 后返回 `true`（表示"服务可用"），`ClientAddr()` 返回监听端点。
- **增强（第二期）**：包装 `server.SetLogger` 或会话钩子统计活动会话数（OPC UA 服务端支持多 Client，与 IEC104/Modbus 的单连接语义不同，前端文案需相应调整为"会话数"）。

`Stats()` 返回 `(读, 写, 变化推送)` 三计数，`Uptime()` 用 `startTime`。

### 4.7 工厂注册（`factory.go`）

```go
case "opcua_server":
    port := cfg.IEC104Port
    nsURI := "GridSim"
    if cfg.OPCUAConfig != nil {
        if cfg.OPCUAConfig.Port > 0 { port = cfg.OPCUAConfig.Port }
        if cfg.OPCUAConfig.NamespaceURI != "" { nsURI = cfg.OPCUAConfig.NamespaceURI }
    }
    return opcua.NewServer(port, nsURI), nil
```

并在 `SupportedProtocols()` 增加 `"opcua_server"`。

### 4.8 配置模型扩展（`internal/model/instance.go`）

```go
type InstanceConfig struct {
    // ... 原有字段
    OPCUAConfig *OPCUAInstanceConfig `json:"opcua_config,omitempty"`
}

type OPCUAInstanceConfig struct {
    Port         int    `json:"port,omitempty"`          // 默认 4840
    NamespaceURI string `json:"namespace_uri,omitempty"` // 默认 "GridSim"
    SecurityNone bool   `json:"security_none,omitempty"` // 本期固定 true
    AllowAnonymous bool `json:"allow_anonymous,omitempty"` // 本期固定 true
}
```

向后兼容：`opcua_config` 为可选指针，旧配置不受影响；`Protocol` 为空仍默认 `iec104`。

### 4.9 点表加载（`pkg/config/loader.go`）

OPC UA 不需要功能码/寄存器地址，**沿用 IEC104 的标准列（A-G）即可**，无需新增专属列。`LoadFromXLSX(path, "opcua_server")` 走与 IEC104 相同的零值回退分支（Modbus 校验仅在 `protocol == "modbus_tcp"` 时触发，不影响本规约）。

> 可选：未来若要自定义 NodeID 命名（而非 `类型.IOA`），可读取 `Alias` 列作为 BrowseName。

### 4.10 Manager 装配

`manager.StartInstance` 现有流程（加载点表 → 建 Store → 工厂建协议 → `SetStore` → `Start` → 装配自动变化引擎 → 可选 HTTP API）**对 OPC UA 完全适用，无需特殊分支**。仅需注意：

- 端口占用检查：OPC UA 用 `IEC104Port`（或 `OPCUAConfig.Port`）字段承载监听端口，复用现有 `net.Listen` 预检逻辑。
- 防火墙：复用 `firewall.EnsurePort`。

---

## 5. 前端设计

### 5.1 类型定义（`web/src/api/index.ts`）

```ts
export interface OPCUAConfig {
  port?: number              // 默认 4840
  namespace_uri?: string     // 默认 "GridSim"
}

export interface InstanceConfig {
  // ... 原有字段
  opcua_config?: OPCUAConfig
}
// InstanceState 同步增加 opcua_config?: OPCUAConfig
```

### 5.2 规约选择（`web/src/views/ConfigPage.vue`）

协议下拉来自 `getProtocols()`（`/api/v1/protocols`），后端 `SupportedProtocols()` 增加 `opcua_server` 后会自动出现在列表。需要：

1. 协议下拉新增 `opcua_server` 的中文标签映射：`OPC UA 服务端`。
2. 当选中 `opcua_server` 时，表单条件渲染 OPC UA 专属字段：
   - **端口**（默认 4840）
   - **命名空间 URI**（默认 `GridSim`）
   - 端点预览：`opc.tcp://<host>:<port>`（只读，便于复制给测试人员）
3. 提交时组装 `opcua_config`，与现有 `modbus_config` / `iec104_client_config` 的处理方式一致。

协议标签映射示例（与现有 Modbus/微电网标签放在一起）：

```ts
const PROTOCOL_LABELS: Record<string, string> = {
  iec104: 'IEC 104 服务端',
  iec104_client: 'IEC 104 客户端',
  modbus_tcp: 'Modbus TCP',
  microgrid: '微电网',
  opcua_server: 'OPC UA 服务端',
}
```

### 5.3 监控 / 详情页（`MonitorPage.vue` / `DetailPage.vue`）

- 点表与实时值展示**完全复用**现有点位快照接口（`/instances/{id}/points`），OPC UA 实例的 Store 与其他规约同构，无需改动表格主体。
- 详情页头部展示 OPC UA 端点 URL、命名空间、会话/读写统计。
- 列展示：OPC UA 无功能码/寄存器列，按协议隐藏 Modbus 专属列（前端已按 `function_code` 是否存在条件渲染，OPC UA 实例这些字段为 0/空，自动隐藏）。

### 5.4 仪表盘（`DashboardPage.vue`）

`by_protocol` 聚合会自动包含 `opcua_server` 计数（后端 `GetDashboardData` 按 `Config.Protocol` 分组），前端补一个协议图标/颜色即可。

---

## 6. 依赖与构建

```bash
# 后端
go get github.com/gopcua/opcua@latest
go mod tidy
```

- `go.mod` 增加 `github.com/gopcua/opcua`。纯 Go，无新增 CGO/系统库，**不影响现有 Makefile + UPX 跨平台打包**。
- 前端无新增依赖。

> 安全提示：本期 OPC UA Server 仅启用 **Security=None + 匿名** 鉴权，属于无认证暴露的网络服务，仅用于内网测试环境。生产/对外暴露场景需在第二期补充证书与用户名密码鉴权（gopcua 已预留 `EnableSecurity` / `EnableAuthMode` / `PrivateKey` / `Certificate` 选项）。前端创建页应给出该提示。

---

## 7. 开发阶段与工时

| 阶段 | 内容 | 交付物 | 工时 |
|------|------|--------|------|
| P0 | 引入 gopcua，PoC：起一个空 server + 1 个节点，本地用 UAExpert/Prosys Client 连通 | 可连通 demo | 0.5d |
| P1 | `pkg/protocol/opcua/server.go` 实现 Protocol 接口（Start/Stop/Stats/Uptime）+ Store 初始化映射 | 读通路 | 1.5d |
| P2 | `nodemap.go` 映射规则 + `Publish` 同步 + 订阅推送 | 实时订阅 | 1d |
| P3 | `watchExternalWrites` 写回写 Store + AO 跟随接口 | 写通路 | 1d |
| P4 | 配置模型 / 工厂 / SupportedProtocols / 端口校验 | 后端打通 | 0.5d |
| P5 | 前端规约选择 + OPCUA 表单 + 类型定义 + 详情页头部 | 前端打通 | 1d |
| P6 | 联调（与真实 OPC UA Client）+ 单测 + 文档 + 打包验证 | 可发布 | 1.5d |
| **合计** | | | **约 7 人天** |

---

## 8. 测试计划

### 8.1 单元测试（Go）

- `nodemap_test.go`：各 PointType → Variant 类型映射、NodeKey/parseNodeKey 往返一致。
- `server_test.go`：起 server → gopcua Client 自连读取节点值；写节点后断言 Store 被更新；Stop 后端口释放。

### 8.2 集成测试

- 用第三方 OPC UA Client 验证互操作：UAExpert / Prosys OPC UA Simulation Client / Python `opcua-asyncio`。
- 场景：Browse 出全部节点 → 订阅 AI 点 → 自动变化引擎驱动 AI 变化 → Client 收到推送；Client 写 AO/DO → HTTP API 读到新值。

### 8.3 回归

- 现有 IEC104/Modbus/微电网实例创建、启动、点位读写不受影响（`Protocol` 为空仍默认 iec104）。
- 多实例并存：同机同时运行 IEC104(2404) + Modbus(502) + OPCUA(4840)。

---

## 9. 向后兼容性

| 场景 | 行为 |
|------|------|
| 旧 JSON 配置（无 `opcua_config`、`protocol` 为空） | 默认 iec104，无影响 |
| 旧 Excel 点表 | OPC UA 走标准列加载，无需新增列 |
| 旧版前端 | 不发送 `opcua_server`，后端不受影响 |
| 现有 HTTP API / 自动变化引擎 / 详情页 | Store 同构，直接复用 |

---

## 10. 风险与权衡

1. **gopcua server 仍标注 "APIs will change"**：锁定具体版本号（go.mod 固定 tag），升级前回归。
2. **连接语义差异**：OPC UA 天然多 Client，"单连接"状态字段语义需在前端调整为"会话数/服务可用"，否则统计含义易误解。
3. **安全**：本期 None+匿名，仅限内网测试；对外需补加密鉴权（已预留接口，列入第二期）。
4. **MapNamespace 扁平结构**：Browse 出来是扁平节点列表，若测试方要求标准 OPC UA 设备模型层级树，需第二期改用 `NodeNamespace` 自建 Object/Variable 树。
5. **数据类型精度**：AI/AO 统一用 `Double`，若 Client 期望 `Float`，需在映射层按 `ValueType`（FLOAT/DOUBLE/INT）细化 Variant 类型。

---

## 11. 后续迭代（Roadmap）

- OPC UA Client（主站采集）模式，与 `iec104_client` 对齐。
- 安全策略（Basic256Sha256）+ 用户名密码 / 证书鉴权。
- 标准设备模型层级 Browse（NodeNamespace + 按 Group 分文件夹）。
- HistoryRead（历史数据，可对接现有 TrendPage 的历史能力）。
- Method 节点（模拟设备命令）。

---

## 附：关键文件改动清单

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `go.mod` / `go.sum` | 改 | 增加 `github.com/gopcua/opcua` |
| `pkg/protocol/opcua/server.go` | 新增 | OPCUAServer 实现 Protocol |
| `pkg/protocol/opcua/nodemap.go` | 新增 | Point↔Node 映射 |
| `pkg/protocol/opcua/sync.go` | 新增 | 同步与回写（可合并进 server.go） |
| `pkg/protocol/factory.go` | 改 | 新增 `opcua_server` case + SupportedProtocols |
| `internal/model/instance.go` | 改 | 新增 `OPCUAInstanceConfig` |
| `cmd/gridsim/main.go` | 改 | 端口取值兼容 opcua_config（参考 modbus 分支） |
| `web/src/api/index.ts` | 改 | `OPCUAConfig` 类型 + InstanceConfig 字段 |
| `web/src/views/ConfigPage.vue` | 改 | 规约标签 + OPCUA 表单 |
| `web/src/views/DetailPage.vue` | 改 | 头部展示 OPC UA 端点信息 |
| `pkg/protocol/opcua/*_test.go` | 新增 | 单元测试 |
