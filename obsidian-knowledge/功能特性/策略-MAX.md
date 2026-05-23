---
title: 策略 - MAX
tags:
  - strategy
  - max
aliases:
  - MAX Strategy
created: 2026-05-23
---

# MAX 策略

英文代号：`max`

## 说明

取多个 IOA 的最大值作为当前值。

## 适用场景

联锁逻辑模拟，多个条件取最大值。

## 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `para_a` | string | 是 | IOA 列表，分号分隔 |
| `para_b` | string | 否 | 关联 IOA（布尔开关，为 0 时输出 0） |
| `period_ms` | int | 是 | 周期（毫秒，≥100） |

## 计算逻辑

```
周期触发:
  ioas = parseParaA(paraA)  // 分号分割
  values = []
  for _, ioa := range ioas:
      values = append(values, store.Get(ioa).Value)
  result = max(values)
  
  if paraB != "":
     关联点 = store.Get(paraB)
     if 关联点.BoolValue == false:
         result = 0
  
  store.SetValue(targetIOA, result)
  publisher.Publish(point)
```

## 配置示例

```json
{
  "strategy": "max",
  "enabled": true,
  "params": {
    "para_a": "16385;16386;16387",
    "para_b": "16400",
    "period_ms": 1000
  }
}
```

---

相关笔记：[[功能特性/自动变化策略|自动变化策略总览]]、[[功能特性/策略-MIN|MIN 策略]]
