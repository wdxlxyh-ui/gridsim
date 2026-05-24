package microgrid

import (
	"fmt"
	"sort"
	"strings"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

// typeChinese maps ComponentType to Chinese prefix used in point names
var typeChinese = map[ComponentType]string{
	CompPV:      "光伏",
	CompBattery: "储能",
	CompLoad:    "负荷",
	CompCharger: "充电桩",
}

// ExpandPoints 将微电网拓扑展开为 IEC104 标准测点列表（EGC 兼容命名）
func (t *Topology) ExpandPoints() []*config.Point {
	var points []*config.Point

	// ── Grid meter (关口表) → METER.* ──
	gridDefs := []struct {
		name  string
		ptype config.PointType
		ioa   int
		alias string
	}{
		{"关口表_有功功率", config.TypeAI, 1, "METER.ActivePW|kW"},
		{"关口表_无功功率", config.TypeAI, 2, "METER.ReactivePW|kvar"},
		{"关口表_电压", config.TypeAI, 3, "METER.Voltage|kV"},
		{"关口表_频率", config.TypeAI, 4, "METER.Frequency|Hz"},
		{"关口表_运行状态", config.TypeDI, 1001, "METER.OemState"},
		{"关口表_孤岛状态", config.TypeDI, 1002, "METER.Island"},
	}
	for _, gd := range gridDefs {
		points = append(points, &config.Point{
			IOA: uint32(gd.ioa), Name: gd.name, ValueType: config.VTFloat,
			PointType: gd.ptype, Efficient: 1.0, BaseValue: 0, Alias: gd.alias,
		})
	}

	// ── Per-device points ──
	for idx, dev := range t.Devices {
		n := idx + 1
		prefix := typeChinese[dev.Type] + itoa(n)
		baseAI := 101 + idx*50
		baseDI := 1101 + idx*50
		baseAO := 3001 + idx*50
		baseDO := 4001 + idx*50

		swVal := 0.0
		if dev.Switch.Closed { swVal = 1.0 }
		initSOC := dev.Params.InitSOC
		if initSOC <= 0 { initSOC = 50 }

		mk := func(ioa int, name, alias string, pt config.PointType, vt config.ValueType, bv float64) *config.Point {
			return &config.Point{IOA: uint32(ioa), Name: prefix + name, ValueType: vt, PointType: pt, Efficient: 1.0, BaseValue: bv, Alias: alias}
		}
		mkAI := func(ioa int, name, alias string, bv float64) *config.Point { return mk(ioa, name, alias, config.TypeAI, config.VTFloat, bv) }
		mkDI := func(ioa int, name, alias string, bv float64) *config.Point { return mk(ioa, name, alias, config.TypeDI, config.VTBit, bv) }
		mkDO := func(ioa int, name, alias string, bv float64) *config.Point { return mk(ioa, name, alias, config.TypeDO, config.VTBit, bv) }
		mkAO := func(ioa int, name, alias string, bv float64) *config.Point { return mk(ioa, name, alias, config.TypeAO, config.VTFloat, bv) }

		switch dev.Type {
		case CompPV: // INV.*
			points = append(points,
				mkAI(baseAI, "_有功功率", "INV.GenActivePW|kW", 0),
				mkAI(baseAI+1, "_日发电量", "INV.APProductionKWH|kWh", 0),
				mkDI(baseDI, "_运行状态", "INV.State", 1),
				mkDI(baseDI+1, "_开关状态", "INV.CtrlState", swVal),
				mkAO(baseAO, "_功率设定", "INV.SysAPSetPoint|kW", 0),
				mkDO(baseDO, "_远程启机", "INV.Start", swVal),
			)
		case CompBattery: // BS.*
			points = append(points,
				mkAI(baseAI, "_电池SOC", "BS.Soc|%", initSOC),
				mkAI(baseAI+1, "_充放电功率", "BS.ActivePW|kW", 0),
				mkDI(baseDI, "_运行状态", "BS.Status", 1),
				mkDI(baseDI+1, "_开关状态", "BS.CtrlState", swVal),
				mkAO(baseAO, "_功率设定", "BS.SysAPSetPoint|kW", 0),
				mkDO(baseDO, "_远程启机", "BS.Start", swVal),
			)
		case CompLoad: // LOAD.*
			points = append(points,
				mkAI(baseAI, "_有功功率", "LOAD.ActivePW|kW", 0),
				mkDI(baseDI, "_运行状态", "LOAD.State", 1),
				mkDI(baseDI+1, "_开关状态", "LOAD.CtrlState", swVal),
				mkAO(baseAO, "_功率设定", "LOAD.SysAPSetPoint|kW", 0),
				mkDO(baseDO, "_遥控分合", "LOAD.SwCtrl", swVal),
			)
		case CompCharger: // PUB_CONN.*
			points = append(points,
				mkAI(baseAI, "_充电功率", "PUB_CONN.ChargePW|kW", 0),
				mkDI(baseDI, "_运行状态", "PUB_CONN.State", 1),
				mkDI(baseDI+1, "_开关状态", "PUB_CONN.CtrlState", swVal),
				mkAO(baseAO, "_功率设定", "PUB_CONN.ChargePWSet|kW", 0),
				mkDO(baseDO, "_遥控分合", "PUB_CONN.SwCtrl", swVal),
			)
		}

		// Custom points
		for ci, cp := range dev.CustomPoints {
			var ptype config.PointType
			switch cp.Type {
			case "AI": ptype = config.TypeAI
			case "DI": ptype = config.TypeDI
			case "DO": ptype = config.TypeDO
			case "AO": ptype = config.TypeAO
			default: continue
			}
			vt := config.VTFloat
			if ptype == config.TypeDI || ptype == config.TypeDO { vt = config.VTBit }
			var ioa int
			switch ptype {
			case config.TypeAI: ioa = baseAI + 5 + ci*2
			case config.TypeDI: ioa = baseDI + 5 + ci*2
			case config.TypeAO: ioa = baseAO + 5 + ci*2
			case config.TypeDO: ioa = baseDO + 5 + ci*2
			}
				alias := cp.Name
				if cp.Alias != "" { alias = cp.Alias }
				points = append(points, &config.Point{
					IOA: uint32(ioa), Name: prefix + "_" + cp.Name, ValueType: vt,
					PointType: ptype, Efficient: 1.0, BaseValue: 0, Alias: alias,
			})
		}
	}

	sort.Slice(points, func(i, j int) bool { return points[i].IOA < points[j].IOA })
	return points
}

// internalSuffix maps EGC Chinese name suffixes back to internal engine suffixes
var internalSuffixes = map[string]string{
	"_有功功率":   "_Power",
	"_日发电量":   "_DailyEnergy",
	"_电池SOC":  "_SOC",
	"_充放电功率": "_Power",
	"_充电功率":   "_Power",
	"_运行状态":   "_Status",
	"_开关状态":   "_SwStatus",
	"_远程启机":   "_SwCtrl",
	"_遥控分合":   "_SwCtrl",
	"_功率设定":   "_Setpoint",
}

// itoa converts int to string without importing strconv in hot path
func itoa(n int) string { return fmt.Sprintf("%d", n) }

// StoreFromTopology 从拓扑创建 Store
func (t *Topology) StoreFromTopology() *library.Store {
	return library.NewStore(t.ExpandPoints())
}

// FormatPointTable 格式化点表
func (t *Topology) FormatPointTable() []map[string]interface{} {
	points := t.ExpandPoints()
	var result []map[string]interface{}
	for _, p := range points {
		unit := ""
		desc := p.Alias
		if idx := strings.Index(p.Alias, "|"); idx >= 0 {
			desc = p.Alias[:idx]
			unit = p.Alias[idx+1:]
		}
		result = append(result, map[string]interface{}{
			"ioa": p.IOA, "name": p.Name, "type": string(p.PointType), "unit": unit, "desc": desc,
		})
	}
	return result
}

// ToggleSwitch 切换开关状态
func (t *Topology) ToggleSwitch(devID string) (bool, error) {
	for i := range t.Devices {
		if t.Devices[i].ID == devID {
			t.Devices[i].Switch.Closed = !t.Devices[i].Switch.Closed
			return t.Devices[i].Switch.Closed, nil
		}
	}
	return false, fmt.Errorf("device %s not found", devID)
}

// Validate 检查拓扑
func (t *Topology) Validate() error {
	if len(t.Devices) == 0 { return fmt.Errorf("至少需要一个设备") }
	for _, d := range t.Devices {
		switch d.Type {
		case CompPV:
			if d.Params.RatedPowerKW <= 0 { return fmt.Errorf("设备 %s 额定功率未设置", d.Name) }
		case CompBattery:
			if d.Params.CapacityKWH <= 0 { return fmt.Errorf("设备 %s 额定容量未设置", d.Name) }
		case CompLoad:
			if d.Params.LoadRatedKW <= 0 { return fmt.Errorf("设备 %s 额定功率未设置", d.Name) }
		case CompCharger:
			if d.Params.ChargerRatedKW <= 0 { return fmt.Errorf("设备 %s 额定功率未设置", d.Name) }
		}
	}
	return nil
}
