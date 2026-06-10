---
title: 策略 - MIN
tags:
  - strategy
  - min
aliases:
  - MIN Strategy
created: 2026-05-23
---

# MIN 策略

英文代号：`min`

## 说明

取多个 IOA 的最小值作为当前值。

## 适用场景

联锁逻辑模拟，多个条件取最小值。

## 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `para_a` | string | 是 | IOA 列表，分号分隔 |
| `para_b` | string | 否 | 关联 IOA（布尔开关） |
| `period_ms` | int | 是 | 周期（毫秒，≥100） |

## 计算逻辑

与 MAX 类似，取 `min(values)` 而非 `max(values)`。

## 配置示例

```json
{
  "strategy": "min",
  "enabled": true,
  "params": {
    "para_a": "16385;16386;16387",
    "period_ms": 1000
  }
}
```

---

相关笔记：[[功能特性/自动变化策略|自动变化策略总览]]、[[功能特性/策略-MAX|MAX 策略]]
