# GridSim 微电网 — 架构图

## 系统架构

```mermaid
graph TB
    subgraph Frontend["🖥 Vue3 前端"]
        ME[MicrogridEditor<br/>设备管理+SVG拓扑]
        DP[DetailPage<br/>测点/置数/策略]
        TP[TrendPage<br/>ECharts趋势]
        CP[ConfigPage<br/>实例管理]
    end

    subgraph API["🔌 REST API"]
        T[/microgrid/topology]
        D[/microgrid/dashboard]
        P[/microgrid/points]
        E[/instances/export-xlsx]
        CR[/instances/csv-replay]
        BR[/instances/batch-replay]
        MT[/instances/metrics]
    end

    subgraph Engine["⚙ Go 引擎"]
        MGE[微电网Engine<br/>tick/功率平衡/SOC/IOA]
        DE[Detail Engine<br/>auto-change/策略]
        FE[公式Engine<br/>{name}引用+表达式]
    end

    subgraph Store["💾 数据层"]
        S[(Store<br/>mapIOA→Point)]
        TJ[(TopologyJSON)]
        AC[(AutoChangeStore)]
    end

    ME --> T
    ME --> D
    DP --> P
    DP --> CR
    CP --> BR
    CP --> MT

    T --> MGE
    D --> MGE
    P --> MGE
    CR --> DE
    BR --> DE
    MT --> DE

    MGE --> S
    DE --> S
    FE --> S
    MGE --> TJ
    DE --> AC
```

## 引擎 Tick 流程

```mermaid
flowchart LR
    PV[1.PV功率] --> Load[2.负荷功率]
    Load --> Bat[3.储能功率]
    Bat --> SOC[3.5 SOC更新]
    SOC --> Bal[4.功率平衡]
    Bal --> Sync[5.Store同步]
    Sync --> Form[5.5 公式评估]
    Form --> Snap[6.快照记录]
```

## 功率约定

```mermaid
graph LR
    PV[S☀ PV≥0] -->|发电| BUS((10kV母线))
    Bat[🔋 Bat>0充电] -->|用电| BUS
    Bat2[🔋 Bat<0放电] -->|发电| BUS
    Load[💡 Load] -->|用电| BUS
    BUS -->|Grid>0用电| GRID[⚡电网]
    BUS -->|Grid<0送电| GRID
```

## API 全景

```mermaid
graph TD
    subgraph "微电网 API"
        T1[topology GET/PUT]
        T2[device POST/PUT/DEL]
        T3[control POST]
        T4[dashboard GET]
        T5[points GET]
        T6[formulas CRUD]
        T7[export-xlsx GET]
    end
    subgraph "通用 API"
        G1[points GET/PUT]
        G2[auto-change CRUD]
        G3[csv-replay POST]
        G4[batch-replay POST/GET]
        G5[metrics GET]
    end
```
