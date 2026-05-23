---
title: 策略 - AO 关联
tags:
  - strategy
  - aofollow
aliases:
  - AOFollow Strategy
created: 2026-05-23
---

# AO 关联策略

英文代号：`aofollow`

## 说明

**事件驱动型**策略（非定时触发），跟随指定 AO 点的遥控值变化。

## 适用场景

遥调联动模拟，当某 AO 被遥控时，关联的 AI 点同步更新。

## 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `follow_ao_ioa` | uint32 | 是 | 关联的 AO 点 IOA |

## 工作流程

```
IEC104 C_SE_NC_1 (AO遥控)
  → iec104.Server.AO 控制
  → store.SetValue(AO)
  → 引擎.HandleAOFollow(aoIOA)
  → 找到所有 FollowAOIOA == aoIOA 的 AI 点
  → 复制 AO 值到各 AI 点 + Publish
```

> [!warning] 历史 Bug
> v2.2.0 之前，IEC104 服务端收到 AO 控制后未触发 `HandleAOFollow`。原因：`iec104.Server` 缺少 `aoFollowFn` 回调字段。v2.2.0 已修复。

## 配置示例

```json
{
  "strategy": "aofollow",
  "enabled": true,
  "params": {
    "follow_ao_ioa": 16400
  }
}
```

---

相关笔记：[[功能特性/自动变化策略|自动变化策略总览]]
