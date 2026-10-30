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
	CompWind:    "风电",
	CompEMS:     "EMS",
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
	// IOA 分配基于设备自声明的 IOABase，与设备在数组中的位置无关
	for idx, dev := range t.Devices {
		prefix := devPrefix(dev, idx)

		// 使用设备声明的 IOABase，而非位置推导
		base := int(dev.IOABase)
		if base < 101 {
			base = 101 + idx*50 // fallback: 旧格式兼容
		}
		baseAI := base
		baseDI := base + 10
		baseAO := base + 20
		baseDO := base + 30

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
			pvSet := 0.0
			if dev.isLimitMode() {
				pvSet = dev.Params.RatedPowerKW
			}
			points = append(points,
				mkAI(baseAI, "_有功功率", "INV.GenActivePW|kW", 0),
				mkAI(baseAI+1, "_日发电量", "INV.APProductionKWH|kWh", 0),
				mkDI(baseDI, "_运行状态", "INV.State", 1),
				mkDI(baseDI+1, "_开关状态", "INV.CtrlState", swVal),
				mkAO(baseAO, "_功率设定", "INV.SysAPSetPoint|kW", pvSet),
				mkDO(baseDO, "_远程启机", "INV.Start", swVal),
			)
			if dev.isLimitMode() {
				points = append(points, mkAI(baseAI+2, "_理论功率", "INV.TheoryActivePW|kW", dev.Params.RatedPowerKW))
			}
		case CompWind: // WGEN.* / WNAC.* / WTUR.*
			points = append(points,
				mkAI(baseAI, "_有功功率", "WGEN.GenActivePW|kW", 0),
				mkAI(baseAI+1, "_日发电量", "PUB_WT.APProductionKWH|kWh", 0),
				mkAI(baseAI+2, "_理论功率", "WNAC.TheoryActivePW|kW", 0),
				mkAI(baseAI+3, "_风速", "WNAC.WindSpeed|m/s", 10),
				mkDI(baseDI, "_运行状态", "WTUR.State", 1),
				mkDI(baseDI+1, "_开关状态", "WTUR.CtrlState", swVal),
				mkDI(baseDI+2, "_通讯状态", "WTUR.AllConnectionSts", 1),
				mkAO(baseAO, "_功率设定", "WGEN.SysAPSetPoint|kW", dev.Params.RatedPowerKW),
				mkDO(baseDO, "_远程启机", "WTUR.Start", swVal),
			)
		case CompEMS: // EMS.*
			ratedSum, remote := 0.0, 1.0
			if dev.ControlMode == ModeLocal {
				remote = 0
			}
			for _, u := range t.emsUnits(dev) {
				if u.Type == CompBattery {
					ratedSum += u.Params.RatedPowerKW_B
				} else {
					ratedSum += u.Params.RatedPowerKW
				}
			}
			setInit := ratedSum // 风/光 EMS 默认不限发; 储能 EMS 默认 0
			if dev.Params.EMSKind == "storage" {
				setInit = 0
			}
			points = append(points,
				mkAI(baseAI, "_有功功率", "EMS.ActivePW|kW", 0),
				mkAI(baseAI+1, "_总额定功率", "EMS.RatedPW|kW", ratedSum),
				mkAI(baseAI+2, "_可调上限", "EMS.UpperLimitPW|kW", ratedSum),
				mkAI(baseAI+3, "_可调下限", "EMS.LowerLimitPW|kW", 0),
				mkDI(baseDI, "_运行状态", "EMS.State", 1),
				mkDI(baseDI+1, "_远方状态", "EMS.RemoteSts", remote),
				mkDI(baseDI+2, "_通讯状态", "EMS.CommSts", 1),
				mkAO(baseAO, "_功率设定", "EMS.SysAPSetPoint|kW", setInit),
				mkDO(baseDO, "_远方控制", "EMS.RemoteCtrl", remote),
			)
			if dev.Params.EMSKind == "storage" {
				points = append(points, mkAI(baseAI+4, "_加权SOC", "EMS.Soc|%", 50))
			}
		case CompBattery: // BS.*
			points = append(points,
				mkAI(baseAI, "_电池SOC", "BS.Soc|%", initSOC),
				mkAI(baseAI+1, "_充放电功率", "BS.ActivePW|kW", 0),
				mkDI(baseDI, "_运行状态", "BS.Status", 1),
				mkDI(baseDI+1, "_开关状态", "BS.CtrlState", swVal),
				mkAO(baseAO, "_功率设定", "BS.SysAPSetPoint|kW", 0),
				mkDO(baseDO, "_远程启机", "BS.Start", swVal),
			)
			if dev.Params.SOCProtection {
				ratedB := dev.Params.RatedPowerKW_B
				if ratedB <= 0 {
					ratedB = 50
				}
				points = append(points,
					mkAI(baseAI+2, "_最大可充功率", "BS.MaxChargePower|kW", ratedB),
					mkAI(baseAI+3, "_最大可放功率", "BS.MaxDischargePower|kW", ratedB),
				)
			}
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

		// Custom points: IOA 由系统在自定义区间自动分配
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
			ioa := base + 40 + ci*2
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
	"_理论功率":   "_Theory",
	"_风速":     "_WindSpeed",
	"_最大可充功率": "_MaxCharge",
	"_最大可放功率": "_MaxDischarge",
	"_通讯状态":   "_Comm",
	"_总额定功率":  "_Rated",
	"_可调上限":   "_UpLimit",
	"_可调下限":   "_LowLimit",
	"_加权SOC":  "_SOC",
	"_远方状态":   "_RemoteSts",
	"_远方控制":   "_RemoteCtl",
}

// itoa converts int to string without importing strconv in hot path
func itoa(n int) string { return fmt.Sprintf("%d", n) }

// nextAvailableIOABase 查找下一个可用的 50-IOA 空闲块
// 从 101 开始步进 50，返回第一个完全空闲的起始地址
func nextAvailableIOABase(devices []Device) uint32 {
	occupied := make(map[uint32]bool)
	// 关口表保留区 1-100
	for ioa := uint32(1); ioa <= 100; ioa++ {
		occupied[ioa] = true
	}
	// 已有设备占用的 IOA 范围
	for _, d := range devices {
		for off := uint32(0); off < 50; off++ {
			occupied[d.IOABase+off] = true
		}
	}
	// 查找第一个连续 50 个空闲 IOA 的块
	for base := uint32(101); base <= 65000; base += 50 {
		free := true
		for off := uint32(0); off < 50; off++ {
			if occupied[base+off] {
				free = false
				break
			}
		}
		if free {
			return base
		}
	}
	return 0 // 地址空间已满
}

// validateIOAUnique 检查拓扑中所有设备的 IOA 分配是否唯一且不与关口表冲突
func validateIOAUnique(topo *Topology) error {
	seen := make(map[uint32]string) // IOA → device name
	// 关口表保留区
	for ioa := uint32(1); ioa <= 100; ioa++ {
		seen[ioa] = "(关口表保留区)"
	}
	for _, d := range topo.Devices {
		base := d.IOABase
		if base == 0 {
			return fmt.Errorf("设备 %q IOABase 未分配", d.Name)
		}
		if base < 101 {
			return fmt.Errorf("设备 %q IOABase %d 与关口表保留区 (1-100) 冲突", d.Name, base)
		}
		// 检查该设备 50 个 IOA 是否与其他设备重叠
		for off := uint32(0); off < 50; off++ {
			ioa := base + off
			if existing, ok := seen[ioa]; ok {
				return fmt.Errorf("IOA %d 冲突: 设备 %q 与 %s", ioa, d.Name, existing)
			}
			seen[ioa] = d.Name
		}
	}
	return nil
}

// StoreFromTopology 从拓扑创建 Store
func (t *Topology) StoreFromTopology() *library.Store {
	pts := t.ExpandPoints()
	// 只对新功能涉及的设备应用点表初始值(风速、EMS 设定值、理论功率等需要非零初值);
	// 其余设备保持"启动时全 0、由首个 tick 写入"的既有行为。
	var prefixes []string
	for idx, dev := range t.Devices {
		if dev.Type == CompWind || dev.Type == CompEMS || dev.isLimitMode() || dev.Params.SOCProtection {
			prefixes = append(prefixes, devPrefix(dev, idx)+"_")
		}
	}
	for _, p := range pts {
		for _, pre := range prefixes {
			if strings.HasPrefix(p.Name, pre) {
				applyInitialValue(p)
				break
			}
		}
	}
	return library.NewStore(pts)
}

// applyInitialValue 把点表初始值(BaseValue)写入当前值; library.NewStore 本身不做这一步。
func applyInitialValue(p *config.Point) {
	switch p.PointType {
	case config.TypeAI, config.TypeAO:
		p.Value = p.BaseValue * p.Efficient
	case config.TypeDI, config.TypeDO:
		p.Value = p.BaseValue
		p.BoolValue = p.BaseValue != 0
	}
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
		case CompWind:
			if d.Params.RatedPowerKW <= 0 { return fmt.Errorf("设备 %s 额定功率未设置", d.Name) }
		case CompEMS:
			if _, ok := emsUnitType[d.Params.EMSKind]; !ok {
				return fmt.Errorf("设备 %s 的 EMS 类型 %q 无效(wind/pv/storage)", d.Name, d.Params.EMSKind)
			}
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
