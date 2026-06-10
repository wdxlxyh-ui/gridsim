---
title: GridSim 架构总览
tags:
  - architecture
  - system-design
aliases:
  - 系统架构
created: 2026-05-23
---

# GridSim 架构总览

## 整体架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                        gridsim (main)                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────────────┐    ┌─────────────────────┐                 │
│  │   Config Loader     │    │   Point Library     │                 │
│  │  (excelize .xlsx)   │───▶│  map[uint32]Point   │                 │
│  │  解析 → 校验         │    │  sync.RWMutex       │                 │
│  └─────────────────────┘    └──────────┬──────────┘                 │
│                                        │                              │
│  ┌──────────────────────────────────────────────────────────┐       │
│  │                    Core Engine                            │       │
│  │  ┌────────────────────┐  ┌──────────────────────────┐    │       │
│  │  │  Protocol Server   │  │   Change Publisher       │    │       │
│  │  │  (IEC104/Modbus)   │  │   (值变更→组包→发送)      │    │       │
│  │  │  - 总召响应          │  │                           │    │       │
│  │  │  - AO/DO 控制       │  │                           │    │       │
│  │  │  - 变化上送          │  │                           │    │       │
│  │  └────────────────────┘  └──────────────────────────┘    │       │
│  └──────────────────────────────────────────────────────────┘       │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────┐       │
│  │              HTTP API Server (net/http)                   │       │
│  │  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │       │
│  │  │ Instance    │  │ Detail/Points│  │ Auto-Change    │  │       │
│  │  │ Management  │  │ CRUD         │  │ Config + Batch │  │       │
│  │  └─────────────┘  └──────────────┘  └────────────────┘  │       │
│  └──────────────────────────────────────────────────────────┘       │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────┐       │
│  │              MCP Server (28 tools)                        │       │
│  │  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │       │
│  │  │ Instance    │  │ Data         │  │ File/CSV       │  │       │
│  │  │ Management  │  │ Interface    │  │ Management     │  │       │
│  │  └─────────────┘  └──────────────┘  └────────────────┘  │       │
│  └──────────────────────────────────────────────────────────┘       │
│                                                                      │
│  CLI: gridsim serve --http :8989 --config-dir ./config            │
│       gridsim -p 2404 -c point.xlsx -H :8080                      │
└─────────────────────────────────────────────────────────────────────┘
```

## 模块结构

```
cmd/gridsim/             入口（传统模式 + 服务模式）
│
├── internal/
│   ├── detail/           Detail 页 & 自动变化引擎
│   │   ├── engine.go      自动变化调度引擎
│   │   ├── strategy.go    11 种策略计算逻辑
│   │   ├── handler.go     Detail 页 HTTP API
│   │   └── store.go       自动变化配置持久化
│   ├── manager/          多实例生命周期管理
│   ├── model/            数据模型
│   ├── storage/          JSON 配置持久化
│   └── mcp/              MCP Server
│
├── pkg/
│   ├── api/              HTTP API 处理器
│   ├── config/           Excel 加载器 + 测点模型
│   ├── iec104/           IEC104 服务端
│   ├── library/          并发安全内存点表
│   ├── protocol/         多规约协议支持
│   │   ├── protocol.go    协议接口定义
│   │   ├── factory.go     协议工厂
│   │   └── modbus/        Modbus TCP 实现
│   └── middleware/       HTTP 中间件
│
└── web/                  Vue 3 + Element Plus 前端
    └── src/views/
        ├── ConfigPage.vue    实例配置管理
        ├── MonitorPage.vue   运行监控
        ├── DetailPage.vue    实例详情 / CSV 回放
        └── TrendPage.vue     趋势对比
```

## 核心数据流

### 写入数据流
```
HTTP API / MCP write_points → Store.SetValue()
  → Point.Changed = true
  → Publisher.Publish(point)
  → Protocol.Publish() → ASDU(COT=3) → TCP Send
```

### 总召数据流
```
IEC104 C_IC_NA_1 (COT=6) received
  → Store.ByType 分组 (AI/DI/PI)
  → 每组遍历构造 ASDU (COT=7)
  → ACT_TERM (COT=10)
```

### 自动变化引擎
```
实例启动 → load auto_changes/{id}.json
  → 为每个 enabled=true 的配置启动 ChangeTask goroutine
  → 周期触发 calc() → write() → Publish()
实例停止 → cancel 所有 goroutine → save state
```

详见：[[架构设计/数据模型|数据模型]]、[[协议支持/多规约架构|多规约架构]]、[[功能特性/自动变化策略|自动变化策略]]
