package microgrid

import (
	"fmt"
	"sort"
	"strings"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

// ExpandPoints 将微电网拓扑展开为 IEC104 标准测点列表
// 测点 IOA 按设备索引固定分配，不因设备增删而变动。
// 分配方案：
//
//	关口表:        AI 1~3, DI 1001~1002
//	设备 i (从0):  AI 101+i*50+0..4, DI 1001+i*50+0..4, AO 3001+i*50, DO 4001+i*50
func (t *Topology) ExpandPoints() []*config.Point {
	var points []*config.Point

	// ── Grid meter fixed points ──
	gridPoints := []struct {
		name  string
		ptype config.PointType
		ioa   int
		alias string
	}{
		{"GRID_P", config.TypeAI, 1, "并网有功|kW"},
		{"GRID_Q", config.TypeAI, 2, "并网无功|kvar"},
		{"GRID_V", config.TypeAI, 3, "电压|kV"},
		{"GRID_F", config.TypeAI, 4, "频率|Hz"},
		{"GRID_Connected", config.TypeDI, 1001, "并网状态"},
		{"GRID_Island", config.TypeDI, 1002, "孤岛状态"},
	}
	for _, gp := range gridPoints {
		points = append(points, &config.Point{
			IOA:       uint32(gp.ioa),
			Name:      gp.name,
			ValueType: config.VTFloat,
			PointType: gp.ptype,
			Efficient: 1.0,
			BaseValue: 0,
			Alias:     gp.alias,
		})
	}

	// ── Per-device points with index-based IOA ──
	for idx, dev := range t.Devices {
		baseAI := 101 + idx*50
		baseDI := 1101 + idx*50
		baseAO := 3001 + idx*50
		baseDO := 4001 + idx*50

		swClosed := dev.Switch.Closed
		swVal := 0.0
		if swClosed {
			swVal = 1.0
		}

		switch dev.Type {
		case CompPV:
			points = append(points,
				&config.Point{IOA: uint32(baseAI), Name: dev.ID + "_Power", ValueType: config.VTFloat, PointType: config.TypeAI, Efficient: 1.0, BaseValue: 0, Alias: "发电功率|kW"},
				&config.Point{IOA: uint32(baseAI + 1), Name: dev.ID + "_DailyEnergy", ValueType: config.VTFloat, PointType: config.TypeAI, Efficient: 1.0, BaseValue: 0, Alias: "日发电量|kWh"},
				&config.Point{IOA: uint32(baseDI), Name: dev.ID + "_Status", ValueType: config.VTBit, PointType: config.TypeDI, Efficient: 1.0, BaseValue: swVal, Alias: "运行状态"},
				&config.Point{IOA: uint32(baseDI + 1), Name: dev.ID + "_SwStatus", ValueType: config.VTBit, PointType: config.TypeDI, Efficient: 1.0, BaseValue: swVal, Alias: "开关状态"},
				&config.Point{IOA: uint32(baseDO), Name: dev.ID + "_SwCtrl", ValueType: config.VTBit, PointType: config.TypeDO, Efficient: 1.0, BaseValue: swVal, Alias: "遥控分合"},
			)

		case CompBattery:
			points = append(points,
				&config.Point{IOA: uint32(baseAI), Name: dev.ID + "_SOC", ValueType: config.VTFloat, PointType: config.TypeAI, Efficient: 1.0, BaseValue: dev.Params.InitSOC, Alias: "荷电状态|%"},
				&config.Point{IOA: uint32(baseAI + 1), Name: dev.ID + "_Power", ValueType: config.VTFloat, PointType: config.TypeAI, Efficient: 1.0, BaseValue: 0, Alias: "充放电功率|kW"},
				&config.Point{IOA: uint32(baseDI), Name: dev.ID + "_Status", ValueType: config.VTBit, PointType: config.TypeDI, Efficient: 1.0, BaseValue: 1, Alias: "运行状态"},
				&config.Point{IOA: uint32(baseDI + 1), Name: dev.ID + "_SwStatus", ValueType: config.VTBit, PointType: config.TypeDI, Efficient: 1.0, BaseValue: swVal, Alias: "开关状态"},
				&config.Point{IOA: uint32(baseAO), Name: dev.ID + "_Setpoint", ValueType: config.VTFloat, PointType: config.TypeAO, Efficient: 1.0, BaseValue: 0, Alias: "功率设定值|kW"},
				&config.Point{IOA: uint32(baseDO), Name: dev.ID + "_SwCtrl", ValueType: config.VTBit, PointType: config.TypeDO, Efficient: 1.0, BaseValue: swVal, Alias: "遥控分合"},
			)

		case CompLoad:
			points = append(points,
				&config.Point{IOA: uint32(baseAI), Name: dev.ID + "_Power", ValueType: config.VTFloat, PointType: config.TypeAI, Efficient: 1.0, BaseValue: 0, Alias: "有功功率|kW"},
				&config.Point{IOA: uint32(baseDI), Name: dev.ID + "_Status", ValueType: config.VTBit, PointType: config.TypeDI, Efficient: 1.0, BaseValue: 1, Alias: "运行状态"},
				&config.Point{IOA: uint32(baseDI + 1), Name: dev.ID + "_SwStatus", ValueType: config.VTBit, PointType: config.TypeDI, Efficient: 1.0, BaseValue: swVal, Alias: "开关状态"},
				&config.Point{IOA: uint32(baseDO), Name: dev.ID + "_SwCtrl", ValueType: config.VTBit, PointType: config.TypeDO, Efficient: 1.0, BaseValue: swVal, Alias: "遥控分合"},
			)

		case CompCharger:
			points = append(points,
				&config.Point{IOA: uint32(baseAI), Name: dev.ID + "_Power", ValueType: config.VTFloat, PointType: config.TypeAI, Efficient: 1.0, BaseValue: 0, Alias: "充电功率|kW"},
				&config.Point{IOA: uint32(baseDI), Name: dev.ID + "_Status", ValueType: config.VTBit, PointType: config.TypeDI, Efficient: 1.0, BaseValue: 1, Alias: "运行状态"},
				&config.Point{IOA: uint32(baseDI + 1), Name: dev.ID + "_SwStatus", ValueType: config.VTBit, PointType: config.TypeDI, Efficient: 1.0, BaseValue: swVal, Alias: "开关状态"},
				&config.Point{IOA: uint32(baseDO), Name: dev.ID + "_SwCtrl", ValueType: config.VTBit, PointType: config.TypeDO, Efficient: 1.0, BaseValue: swVal, Alias: "遥控分合"},
			)
		}

		// Custom points
		for ci, cp := range dev.CustomPoints {
			var ptype config.PointType
			switch cp.Type {
			case "AI":
				ptype = config.TypeAI
			case "DI":
				ptype = config.TypeDI
			case "DO":
				ptype = config.TypeDO
			case "AO":
				ptype = config.TypeAO
			default:
				continue
			}
			vt := config.VTFloat
			if ptype == config.TypeDI || ptype == config.TypeDO {
				vt = config.VTBit
			}
			var ioa int
			switch ptype {
			case config.TypeAI:
				ioa = baseAI + 5 + ci*2
			case config.TypeDI:
				ioa = baseDI + 5 + ci*2
			case config.TypeAO:
				ioa = baseAO + 5 + ci*2
			case config.TypeDO:
				ioa = baseDO + 5 + ci*2
			}
			points = append(points, &config.Point{
				IOA:       uint32(ioa),
				Name:      dev.ID + "_" + cp.Name,
				ValueType: vt,
				PointType: ptype,
				Efficient: 1.0,
				BaseValue: 0,
				Alias:     "自定义:" + cp.Name,
			})
		}
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].IOA < points[j].IOA
	})
	return points
}

// StoreFromTopology 从拓扑创建 Store
func (t *Topology) StoreFromTopology() *library.Store {
	points := t.ExpandPoints()
	return library.NewStore(points)
}

// FormatPointTable 格式化点表为便于展示的结构
func (t *Topology) FormatPointTable() []map[string]interface{} {
	points := t.ExpandPoints()
	var result []map[string]interface{}
	for _, p := range points {
		unit := ""
		if idx := strings.Index(p.Alias, "|"); idx >= 0 {
			unit = p.Alias[idx+1:]
		}
		result = append(result, map[string]interface{}{
			"ioa":  p.IOA,
			"name": p.Name,
			"type": string(p.PointType),
			"unit": unit,
			"desc": p.Alias,
		})
	}
	return result
}

// ToggleSwitch 根据设备 ID 切换开关状态
func (t *Topology) ToggleSwitch(devID string) (bool, error) {
	for i := range t.Devices {
		if t.Devices[i].ID == devID {
			t.Devices[i].Switch.Closed = !t.Devices[i].Switch.Closed
			return t.Devices[i].Switch.Closed, nil
		}
	}
	return false, fmt.Errorf("device %s not found", devID)
}

// Validate 检查拓扑是否可启动
func (t *Topology) Validate() error {
	if len(t.Devices) == 0 {
		return fmt.Errorf("至少需要一个设备")
	}
	for _, d := range t.Devices {
		switch d.Type {
		case CompPV:
			if d.Params.RatedPowerKW <= 0 {
				return fmt.Errorf("设备 %s 额定功率未设置", d.Name)
			}
		case CompBattery:
			if d.Params.CapacityKWH <= 0 {
				return fmt.Errorf("设备 %s 额定容量未设置", d.Name)
			}
		case CompLoad:
			if d.Params.LoadRatedKW <= 0 {
				return fmt.Errorf("设备 %s 额定功率未设置", d.Name)
			}
		case CompCharger:
			if d.Params.ChargerRatedKW <= 0 {
				return fmt.Errorf("设备 %s 额定功率未设置", d.Name)
			}
		}
	}
	return nil
}
