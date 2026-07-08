# GridSim 快速参考

## 项目核心概念

GridSim 是一个 **IEC104 从站模拟器**，支持多实例、多协议（IEC104/Modbus TCP）、自动变化策略、微电网仿真。

## 关键文件速查

### 需要修改实例管理逻辑时

| 文件 | 职责 |
|------|------|
| `cmd/gridsim/main.go` | HTTP 路由注册、请求处理入口 |
| `internal/manager/manager.go` | 实例生命周期管理（创建/启动/停止/删除） |
| `internal/model/instance.go` | InstanceConfig 数据模型 |
| `internal/storage/store.go` | JSON 配置持久化 |

### 需要修改测点/自动变化逻辑时

| 文件 | 职责 |
|------|------|
| `internal/detail/engine.go` | 自动变化调度引擎 |
| `internal/detail/strategy.go` | 11 种策略计算逻辑 |
| `internal/detail/handler.go` | 详情页 HTTP API |
| `pkg/library/store.go` | 并发安全内存点表 |
| `pkg/config/loader.go` | Excel 点表加载 |

### 需要修改协议相关时

| 文件 | 职责 |
|------|------|
| `pkg/protocol/protocol.go` | Protocol 接口定义 |
| `pkg/protocol/factory.go` | 协议工厂 |
| `pkg/iec104/server.go` | IEC104 服务端 |
| `pkg/protocol/modbus/tcp_server.go` | Modbus TCP 服务端 |

### 需要修改微电网功能时

| 文件 | 职责 |
|------|------|
| `internal/microgrid/engine.go` | 微电网仿真引擎 |
| `internal/microgrid/handler.go` | 微电网 HTTP API |
| `internal/microgrid/model.go` | 设备模型定义 |

### 需要修改前端时

| 文件 | 职责 |
|------|------|
| `web/src/views/ConfigPage.vue` | 实例配置管理页 |
| `web/src/views/DetailPage.vue` | 实例详情页（测点操作） |
| `web/src/views/MicrogridEditor.vue` | 微电网编辑器 |
| `web/src/api/index.ts` | Axios API 客户端 |

## 常用代码模式

### 添加新的 API 端点

```go
// 1. 在 main.go 的 registerRoutes() 注册路由
mux.HandleFunc("/api/v1/your-endpoint", ws.handleYourEndpoint)

// 2. 实现处理函数
func (ws *webServer) handleYourEndpoint(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    switch r.Method {
    case http.MethodGet:
        // 处理 GET
    case http.MethodPost:
        // 处理 POST
    default:
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
    }
}
```

### 添加新的自动变化策略

```go
// 1. 在 internal/model/detail.go 添加常量
const StrategyYourNew StrategyType = "your_new"

// 2. 在 internal/detail/strategy.go 的 runOnce() 添加逻辑
case model.StrategyYourNew:
    // 计算新值
    newValue := calculateYourStrategy(s, cfg)
    runner.store.SetValue(cfg.PointIOA, newValue)
```

### 访问实例的 Store 和 Engine

```go
// 通过 Manager 获取
store := ws.mgr.GetStore(instanceID)
engine := ws.mgr.GetEngine(instanceID)

// 获取测点值
point, ok := store.Get(ioa)

// 设置测点值（会触发变化上送）
store.SetValue(ioa, newValue)
if point, ok := store.Get(ioa); ok {
    proto.Publish(point)
}
```

## 数据模型速查

### InstanceConfig 关键字段

```go
ID         string  // 实例 ID
Name       string  // 实例名称
IEC104Port int     // IEC104 端口
Protocol   string  // "iec104" / "modbus" / "microgrid"
XLSXFile   string  // 点表文件路径
```

### Point 关键字段

```go
IOA       uint32    // 信息体地址
Name      string    // 测点名称
PointType PointType // AI/DI/PI/DO/AO
ValueType ValueType // FLOAT/DOUBLE/INT/BIT
Value     float64   // 当前值（AI/AO）
BoolValue bool      // 当前值（DI/DO）
IntValue  int32     // 当前值（PI）
```

### AutoChangeConfig 关键字段

```go
PointIOA uint32        // 目标测点 IOA
Strategy StrategyType  // 策略类型
Enabled  bool          // 是否启用
Params   StrategyParams // 策略参数
```

## 前端常用模式

### 调用 API

```typescript
import { getPoints, setValue } from '@/api'

// 获取测点列表
const points = await getPoints(instanceId)

// 置数
await setValue(instanceId, ioa, { value: 123.45 })
```

### 使用 composables

```typescript
import { useApiFeedback } from '@/composables/useApiFeedback'

const { withFeedback } = useApiFeedback()

// 带反馈的 API 调用
await withFeedback(
  () => setValue(id, ioa, data),
  '置数成功',
  '置数失败'
)
```

## 构建命令

```bash
# 本地开发
cd web && npm run build && cd ..
go build -o bin/gridsim ./cmd/gridsim/
./bin/gridsim serve --http :8989 --config-dir ./config

# 交叉编译
make build-linux-amd64
make build-windows
make dist  # 三平台发行包
```

## 调试技巧

### 查看日志

```bash
# 服务模式日志输出到控制台
./bin/gridsim serve --log debug

# 实例独立日志目录
logs/instances/{id}_{port}/
```

### 检查端口占用

```bash
# Linux
ss -tlnp | grep 2404

# Windows
netstat -ano | findstr 2404
```

### 测试 API

```bash
# 获取实例列表
curl http://localhost:8989/api/v1/instances

# 置数
curl -X PUT http://localhost:8989/api/v1/instances/{id}/points/16385 \
  -H 'Content-Type: application/json' \
  -d '{"value": 235.5}'
```
