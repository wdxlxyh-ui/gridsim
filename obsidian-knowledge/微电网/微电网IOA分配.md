---
title: 微电网 IOA 分配规则
tags:
  - microgrid
  - ioa
  - addressing
aliases:
  - Microgrid IOA Allocation
  - 微电网测点分配
created: 2026-05-23
---

# 微电网 IOA 分配规则

## 分配方案

```
关口表:      AI 1~4, DI 1001~1002
设备 i (从0): AI 101+i×50+0..4, DI 1101+i×50+0..4
             AO 3001+i×50,     DO 4001+i×50
自定义测点:   在各设备段末尾递增
```

## 测点命名规则

每个设备展开为以下测点：

| 名称 | 类型 | 说明 |
|------|------|------|
| `{devID}_Power` | AI | 设备功率 |
| `{devID}_SOC` | AI | 电池 SOC（仅储能） |
| `{devID}_SwStatus` | DI | 开关状态 |
| `{devID}_SwCtrl` | DO | 开关控制 |
| `{devID}_Status` | DI | 运行状态 |
| `{devID}_AO` | AO | 远方设定值 |
| `GRID_P` | AI | 并网点功率 |
| `GRID_SwStatus` | DI | 并网点开关 |
| `GRID_SwCtrl` | DO | 并网点开关控制 |

## 持久化

点表在拓扑保存时持久化到 `MicrogridConfig.PointsJSON`，引擎启动时加载。

---

## 相关笔记

- [[微电网/微电网概述|微电网概述]]
- [[微电网/微电网架构与引擎|微电网架构与引擎]]
