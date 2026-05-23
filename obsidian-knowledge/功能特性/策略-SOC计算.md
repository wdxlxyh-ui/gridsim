---
title: 策略 - SOC 计算
tags:
  - strategy
  - soc
aliases:
  - SOC Strategy
created: 2026-05-23
---

# SOC 计算策略

英文代号：`soc`

## 说明

基于功率积分计算电池荷电状态（State of Charge）。

## 适用场景

储能系统仿真，模拟电池充放电过程。

## 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `init_soc` | float64 | 是 | 初始 SOC（%） |
| `rated_cap` | float64 | 是 | 额定容量（kWh） |
| `power_ioa` | uint32 | 是 | 功率测点 IOA |
| `integral_ms` | int | 是 | 积分周期（毫秒） |

## 计算逻辑

```
周期触发:
  power = store.Get(powerIOA).Value  // 当前功率
  energy = power * (integral_ms / 3600000.0)  // kWh
  SOC = SOC + energy / ratedCap * 100
  SOC = clamp(SOC, 0, 100)
  store.SetValue(ioa, SOC)
  publisher.Publish(point)
```

## 配置示例

```json
{
  "strategy": "soc",
  "enabled": true,
  "params": {
    "init_soc": 50,
    "rated_cap": 100,
    "power_ioa": 16385,
    "integral_ms": 1000
  }
}
```

---

相关笔记：[[功能特性/自动变化策略|自动变化策略总览]]、[[功能特性/策略-电量统计|电量统计策略]]
