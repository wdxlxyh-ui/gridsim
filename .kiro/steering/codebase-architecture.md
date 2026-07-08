# GridSim 代码架构说明

本文档描述 GridSim 项目的完整代码架构，供 AI 助手在后续会话中快速理解项目结构。

## 项目概述

**GridSim** 是一个多协议电网仿真平台，核心功能是模拟 IEC104 从站设备，用于 SCADA 系统开发和集成测试。

- **当前版本**: v3.1.0
- **技术栈**: Go 1.21+ (后端) + Vue 3 + TypeScript + Element Plus + ECharts (前端)
- **主要协议**: IEC 104、Modbus TCP

## 核心架构

### 双运行模式

1. **传统模式 (Legacy Mode)**: 单进程 = 单端口 + 单客户端，纯 CLI
2. **服务模式 (Serve Mode)**: 多实例生命周期管理 + Vue 3 Web 界面

```
cmd/gridsim/main.go
├── runLegacyMode()  // 传统模式：单实例
└── runServerMode()  // 服务模式：多实例管理
```

## 目录结构详解

### 后端 (Go)

```
gridsim_dev/
├── cmd/
│   ├── gridsim/
│   │   ├── main.go              # 主入口，路由注册，HTTP 处理器
│   │   └── resources/           # 内置资源（Postman collection 等）
│   └── gridsim-mcp/
│       └── main.go              # MCP Server 入口 (stdio)
│
├── internal/                    # 内部模块（不对外暴露）
│   ├── manager/
│   │   └── manager.go           # 多实例生命周期管理（最多1000个）
│   │                           # - 创建/启动/停止/删除实例
│   │                           # - 端口冲突检测
│   │                           # - 微电网实例特殊处理
│   │
│   ├── detail/                  # v2.1 详情页模块
│   │   ├── engine.go           # 自动变化调度引擎
│   │                           # - 管理 changeTask goroutine
│   │                           # - 支持 11 种策略
│   │   ├── strategy.go         # 策略计算逻辑
│   │                           # - increment/random/csv/max/min/soc/energy/ao_follow/api_update/manual/custom
│   │   ├── handler.go          # 详情页 HTTP API
│   │   ├── store.go            # 自动变化配置持久化
│   │   └── provider.go         # StoreProvider 接口
│   │
│   ├── microgrid/              # v3.0 微电网模块
│   │   ├── engine.go           # 微电网仿真引擎
│   │   ├── handler.go          # 微电网 HTTP API
│   │   ├── model.go            # 设备模型（PV/Battery/Load/Charger）
│   │   ├── pointmap.go         # IOA 地址分配
│   │   ├── history.go          # 历史数据
│   │   └── formula_test.go     # 自定义公式测试
│   │
│   ├── model/                  # 数据模型
│   │   ├── instance.go         # InstanceConfig, InstanceState
│   │   ├── detail.go           # AutoChangeConfig, 策略枚举
│   │   └── user.go             # UserConfig
│   │
│   ├── storage/
│   │   └── store.go            # JSON 配置持久化
│   │
│   └── mcp/
│       ├── server.go           # MCP Server 实现
│       └── client.go           # MCP Client
│
├── pkg/                        # 公共包（可对外暴露）
│   ├── api/
│   │   ├── handler.go          # HTTP API 处理器（实例级）
│   │   ├── proxy_handler.go    # 接口测试代理处理器
│   │   └── proxy_store.go      # 代理配置存储
│   │
│   ├── config/
│   │   ├── loader.go           # Excel 点表加载器
│   │   ├── model.go            # Point 数据模型
│   │   └── writer.go           # Excel 点表写入器
│   │
│   ├── iec104/
│   │   └── server.go           # IEC 104 服务端实现
│   │
│   ├── protocol/               # 多规约协议支持
│   │   ├── protocol.go         # Protocol 接口定义
│   │   ├── factory.go          # 协议工厂
│   │   ├── iec104_wrapper.go   # IEC104 包装
│   │   └── modbus/
│   │       ├── tcp_server.go   # Modbus TCP 服务端
│   │       └── converter.go    # 数据类型转换
│   │
│   ├── library/
│   │   └── store.go            # 并发安全内存点表
│   │
│   ├── middleware/
│   │   ├── auth.go             # JWT 认证
│   │   ├── idempotency.go      # 幂等性中间件
│   │   └── recovery.go         # 恢复中间件
│   │
│   ├── errors/
│   │   └── errors.go           # 结构化错误响应
│   │
│   ├── events/
│   │   └── bus.go              # SSE 事件总线
│   │
│   ├── openapi/
│   │   └── spec.go             # OpenAPI 3.0 规范生成
│   │
│   ├── recording/
│   │   └── recorder.go         # 场景录制器
│   │
│   └── firewall/
│       └── firewall.go         # iptables 防火墙管理
│
├── web/                        # Vue 3 前端
│   └── src/
│       ├── main.ts             # 入口
│       ├── App.vue             # 根组件
│       │
│       ├── views/              # 页面组件
│       │   ├── DashboardPage.vue   # v3.1 仪表盘
│       │   ├── ConfigPage.vue      # 实例配置管理
│       │   ├── MonitorPage.vue     # 运行监控
│       │   ├── DetailPage.vue      # v2.1 实例详情页
│       │   ├── TrendPage.vue       # v2.2 趋势对比页
│       │   ├── ProxyPage.vue       # 接口测试
│       │   ├── MicrogridEditor.vue # v3.0 微电网编辑器
│       │   └── LoginPage.vue       # 登录页
│       │
│       ├── components/         # 通用组件
│       │   ├── OnboardingGuide.vue  # v3.1 操作引导
│       │   ├── CommandPalette.vue   # v3.1 命令面板
│       │   ├── SkeletonScreen.vue   # v3.1 骨架屏
│       │   ├── PointTableEditor.vue # 点表编辑器
│       │   ├── microgrid/           # 微电网子组件（10+）
│       │   └── proxy/               # 代理测试组件
│       │
│       ├── api/
│       │   └── index.ts        # Axios API 客户端
│       │
│       ├── composables/
│       │   ├── useApi.ts       # API 封装
│       │   └── useApiFeedback.ts # API 反馈封装
│       │
│       ├── router/
│       │   └── index.ts        # Vue Router 配置
│       │
│       └── styles/
│           └── theme.css       # 主题变量
│
├── config/                     # 运行时配置
│   ├── instances.json          # 实例配置存储
│   ├── users.json              # 用户配置
│   ├── proxy-store.json        # 代理配置
│   └── csv/                    # CSV 回放文件
│
├── scripts/                    # 启停脚本
│   ├── start.sh / start.bat
│   ├── stop.sh / stop.bat
│   └── restart.sh / restart.bat
│
└── samples/                    # 示例点表
    ├── point.xlsx
    ├── 固定验证-储能.xlsx
    └── ModbusTCP-ESS.xlsx
```

## 核心数据流

### 1. 实例管理流程

```
用户请求 → main.go (HTTP Handler)
         → manager.Manager
         → 创建 InstanceConfig
         → storage.ConfigStore 持久化
         
启动实例 → manager.StartInstance()
        → config.LoadFromXLSX() 加载点表
        → library.NewStore() 创建内存点表
        → protocol.New() 创建协议实例
        → detail.NewEngine() 创建自动变化引擎
        → 注册到 manager.instances map
```

### 2. 测点数据流

```
IEC104 客户端请求
    → pkg/iec104/server.go 接收
    → pkg/library/store.go 查询点值
    → 返回遥测/遥信/遥脉数据

HTTP API 置数请求
    → internal/detail/handler.go
    → detail.Engine 检查策略冲突
    → library.Store.SetValue()
    → protocol.Publish() 触发变化上送
```

### 3. 自动变化引擎

```
Engine.startTaskLocked(cfg)
    → 创建 context + ticker
    → 启动 goroutine 循环
    → strategyRunner.runOnce() 计算新值
    → Store.SetValue() 更新点值
    → publisher.Publish() 触发上送
```

## 关键数据模型

### InstanceConfig (internal/model/instance.go)

```go
type InstanceConfig struct {
    ID              string            `json:"id"`
    Name            string            `json:"name"`
    IEC104Port      int               `json:"iec104_port"`
    XLSXFile        string            `json:"xlsx_file"`
    Protocol        string            `json:"protocol"`      // "iec104", "modbus", "microgrid"
    HttpEnabled     bool              `json:"http_enabled"`
    HttpPort        int               `json:"http_port"`
    ModbusConfig    *ModbusConfig     `json:"modbus_config,omitempty"`
    MicrogridConfig *MicrogridConfig  `json:"microgrid_config,omitempty"`
}
```

### Point (pkg/config/model.go)

```go
type Point struct {
    IOA             uint32     `json:"ioa"`              // 信息体地址
    Name            string     `json:"name"`             // 测点名称
    ValueType       ValueType  `json:"value_type"`       // FLOAT/DOUBLE/INT/BIT
    PointType       PointType  `json:"point_type"`       // AI/DI/PI/DO/AO
    Efficient       float64    `json:"efficient"`        // 系数
    BaseValue       float64    `json:"base_value"`       // 初始值
    Alias           string     `json:"alias"`            // 别名
    Value           float64    `json:"value"`            // AI/AO 当前值
    BoolValue       bool       `json:"bool_value"`       // DI/DO 当前值
    IntValue        int32      `json:"int_value"`        // PI 当前值
    FunctionCode    uint8      `json:"function_code"`    // Modbus 功能码
    RegisterAddress uint16     `json:"register_address"` // Modbus 寄存器地址
    QDS             uint8      `json:"qds"`              // 品质描述
    UpdatedAt       time.Time  `json:"updated_at"`       // 更新时间
}
```

### AutoChangeConfig (internal/model/detail.go)

```go
type AutoChangeConfig struct {
    PointIOA uint32        `json:"point_ioa"`
    Strategy StrategyType  `json:"strategy"`
    Enabled  bool          `json:"enabled"`
    Params   StrategyParams `json:"params"`
}

type StrategyType string
const (
    StrategyIncrement  StrategyType = "increment"   // 递增
    StrategyRandom     StrategyType = "random"      // 随机
    StrategyCSV        StrategyType = "csv"         // CSV 回放
    StrategyMax        StrategyType = "max"         // MAX
    StrategyMin        StrategyType = "min"         // MIN
    StrategySOC        StrategyType = "soc"         // SOC 计算
    StrategyEnergy     StrategyType = "energy"      // 电量统计
    StrategyAOFollow   StrategyType = "ao_follow"   // AO 关联
    StrategyAPIUpdate  StrategyType = "api_update"  // 接口更新
    StrategyManual     StrategyType = "manual"      // 手动
    StrategyCustom     StrategyType = "custom"      // 自定义公式
)
```

## HTTP API 概览

### 实例管理 API

| 端点 | 说明 |
|------|------|
| `GET /api/v1/instances` | 列出所有实例 |
| `POST /api/v1/instances` | 创建实例 |
| `GET /api/v1/instances/{id}` | 获取实例详情 |
| `PUT /api/v1/instances/{id}` | 更新实例 |
| `DELETE /api/v1/instances/{id}` | 删除实例 |
| `POST /api/v1/instances/{id}/start` | 启动实例 |
| `POST /api/v1/instances/{id}/stop` | 停止实例 |
| `GET /api/v1/status` | 全局状态 |
| `GET /api/v1/state` | 统一状态快照 |

### 详情页 API

| 端点 | 说明 |
|------|------|
| `GET /api/v1/instances/{id}/points` | 获取所有测点 |
| `GET /api/v1/instances/{id}/points/batch?ioas=X,Y,Z` | 批量读取 |
| `PUT /api/v1/instances/{id}/points/{ioa}` | 置数 |
| `PUT /api/v1/instances/{id}/points/auto-change/{ioa}` | 配置自动变化 |
| `GET /api/v1/instances/{id}/points/export` | 导出 CSV |

### 微电网 API

| 端点 | 说明 |
|------|------|
| `GET /api/v1/microgrid/{id}/topology` | 获取拓扑 |
| `PUT /api/v1/microgrid/{id}/topology` | 更新拓扑 |
| `GET /api/v1/microgrid/{id}/device` | 获取设备列表 |
| `POST /api/v1/microgrid/{id}/device` | 添加设备 |
| `GET /api/v1/microgrid/{id}/points` | 获取测点映射 |
| `GET /api/v1/microgrid/{id}/export-xlsx` | 导出点表 |

## 前端架构

### 路由配置 (web/src/router/index.ts)

```typescript
const routes = [
  { path: '/', component: DashboardPage },
  { path: '/config', component: ConfigPage },
  { path: '/monitor', component: MonitorPage },
  { path: '/detail/:id', component: DetailPage },
  { path: '/trend/:id', component: TrendPage },
  { path: '/proxy', component: ProxyPage },
  { path: '/microgrid/:id', component: MicrogridEditor },
  { path: '/login', component: LoginPage },
]
```

### API 客户端 (web/src/api/index.ts)

```typescript
// 实例管理
export const listInstances = () => axios.get('/api/v1/instances')
export const createInstance = (data) => axios.post('/api/v1/instances', data)
export const startInstance = (id) => axios.post(`/api/v1/instances/${id}/start`)

// 测点操作
export const getPoints = (id) => axios.get(`/api/v1/instances/${id}/points`)
export const setValue = (id, ioa, data) => axios.put(`/api/v1/instances/${id}/points/${ioa}`, data)
export const configAutoChange = (id, ioa, data) => axios.put(`/api/v1/instances/${id}/points/auto-change/${ioa}`, data)
```

## 构建与运行

### 本地开发

```bash
# 构建前端
cd web && npm install && npm run build && cd ..

# 构建后端
go build -o bin/gridsim ./cmd/gridsim/

# 启动服务模式
./bin/gridsim serve --http :8989 --config-dir ./config

# 浏览器访问 http://localhost:8989
```

### 交叉编译

```bash
make build-linux-amd64    # Linux amd64
make build-linux-arm64    # Linux arm64
make build-windows        # Windows amd64
make dist                 # 三平台发行包
```

## MCP 工具

GridSim 提供 37 个 MCP 工具，支持 AI 助手直接控制模拟器：

- 实例管理: `create_instance`, `start_instance`, `stop_instance`, `delete_instance`...
- 测点操作: `read_point`, `write_point`, `read_points_batch`...
- 自动变化: `config_auto_change`, `list_auto_change_configs`...
- 微电网: `microgrid_get_topology`, `microgrid_add_device`...
- 接口测试: `proxy_execute_request`, `proxy_create_collection`...

详见 `MCP.md` 和 `docs/MCP-工具文档.md`。

## 注意事项

1. **实例上限**: 最多支持 1000 个并发实例 (`manager.MaxInstances`)
2. **端口管理**: 自动通过 iptables 管理防火墙规则
3. **单客户端限制**: 每个 IEC104 实例只接受一个客户端连接
4. **策略冲突**: 置数操作会检查自动变化策略，`api_update` 和 `manual` 策略允许 API 写入
5. **CSV 回放**: 支持相对时间（ms/s）和绝对时间（hh:mm:ss）两种模式
6. **微电网拓扑**: 存储为 JSON 格式，支持 4 种设备类型

## 扩展开发指南

### 添加新的自动变化策略

1. 在 `internal/model/detail.go` 添加策略常量
2. 在 `internal/detail/strategy.go` 的 `runOnce()` 方法添加计算逻辑
3. 更新前端策略选择器

### 添加新的协议支持

1. 在 `pkg/protocol/protocol.go` 实现 `Protocol` 接口
2. 在 `pkg/protocol/factory.go` 注册新协议
3. 更新前端协议选择器

### 添加新的 API 端点

1. 在 `cmd/gridsim/main.go` 的 `registerRoutes()` 注册路由
2. 实现 HTTP 处理函数
3. 更新 `pkg/openapi/spec.go` 的 OpenAPI 规范
4. 更新前端 API 客户端
