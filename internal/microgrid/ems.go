package microgrid

import "math"

// emsUnitType EMS 类型 → 所管设备类型
var emsUnitType = map[string]ComponentType{
	"wind":    CompWind,
	"pv":      CompPV,
	"storage": CompBattery,
}

var emsKindCN = map[string]string{"wind": "风电", "pv": "光伏", "storage": "储能"}

// devPrefix 测点名前缀: 普通设备为"类型+全局序号", EMS 为"风电EMS3"等。
func devPrefix(dev Device, idx int) string {
	if dev.Type == CompEMS {
		return emsKindCN[dev.Params.EMSKind] + "EMS" + itoa(idx+1)
	}
	return typeChinese[dev.Type] + itoa(idx+1)
}

// emsUnits 返回 EMS 所管设备; EMSMembers 非空时只取列出的 ID。
func (t *Topology) emsUnits(ems Device) []Device {
	want, ok := emsUnitType[ems.Params.EMSKind]
	if !ok {
		return nil
	}
	var members map[string]bool
	if len(ems.Params.EMSMembers) > 0 {
		members = make(map[string]bool, len(ems.Params.EMSMembers))
		for _, id := range ems.Params.EMSMembers {
			members[id] = true
		}
	}
	var out []Device
	for _, d := range t.Devices {
		if d.Type != want || (members != nil && !members[d.ID]) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// unitAvail 风/光单元当前可发功率(受额定限制); 非限功率模式的光伏视为额定。
func (e *Engine) unitAvail(dev Device) float64 {
	rated := dev.Params.RatedPowerKW
	if rated <= 0 {
		rated = 100
	}
	if !dev.isLimitMode() {
		return rated
	}
	return math.Max(0, math.Min(rated, e.readPt(dev.ID+"_Theory")))
}

// dispatchEMSLocked 把各 EMS 的站级设定值拆分写入所管单元的 _Setpoint。
// 本地模式或通讯中断时不下发(单元保持上一次指令)。参与下发的单元记入 emsCmd。
func (e *Engine) dispatchEMSLocked() {
	e.emsCmd = make(map[string]bool)
	for _, ems := range e.topology.Devices {
		if ems.Type != CompEMS {
			continue
		}
		if e.readPt(ems.ID+"_RemoteCtl") < 0.5 || e.readPt(ems.ID+"_Comm") < 0.5 {
			continue
		}
		var units []Device
		var caps []float64 // 每个单元在本次下发方向上的可用能力
		total := e.readPt(ems.ID + "_Setpoint")
		storage := ems.Params.EMSKind == "storage"
		for _, u := range e.topology.emsUnits(ems) {
			if !u.Switch.Closed || u.ControlMode == ModeLocal {
				continue
			}
			var c float64
			switch {
			case storage && total >= 0:
				c, _ = e.batteryLimits(u)
			case storage:
				_, c = e.batteryLimits(u)
			default:
				c = e.unitAvail(u)
			}
			units = append(units, u)
			caps = append(caps, c)
		}
		sum := 0.0
		for _, c := range caps {
			sum += c
		}
		mag := math.Abs(total)
		if !storage && total < 0 {
			mag = 0
		}
		if mag > sum {
			mag = sum
		}
		sign := 1.0
		if total < 0 && storage {
			sign = -1
		}
		for i, u := range units {
			v := 0.0
			if sum > 0 {
				v = sign * mag * caps[i] / sum
			}
			e.writePt(u.ID+"_Setpoint", v)
			e.emsCmd[u.ID] = true
		}
	}
}

// syncEMSLocked 写 EMS 汇总测点(总出力、可调上下限、加权 SOC、远方状态)。
func (e *Engine) syncEMSLocked(ems Device) {
	storage := ems.Params.EMSKind == "storage"
	var power, rated, upper, lower, socNum, socDen float64
	for _, u := range e.topology.emsUnits(ems) {
		if storage {
			rated += u.Params.RatedPowerKW_B
			if !u.Switch.Closed {
				continue
			}
			power += e.batPower[u.ID]
			c, d := e.batteryLimits(u)
			upper += c
			lower -= d
			if s, ok := e.soc[u.ID]; ok && u.Params.CapacityKWH > 0 {
				socNum += s * u.Params.CapacityKWH
				socDen += u.Params.CapacityKWH
			}
			continue
		}
		rated += u.Params.RatedPowerKW
		if !u.Switch.Closed {
			continue
		}
		power += e.pvPower[u.ID]
		upper += e.unitAvail(u)
	}
	e.writePt(ems.ID+"_Power", power)
	e.writePt(ems.ID+"_Rated", rated)
	e.writePt(ems.ID+"_UpLimit", upper)
	e.writePt(ems.ID+"_LowLimit", lower)
	if storage && socDen > 0 {
		e.writePt(ems.ID+"_SOC", socNum/socDen)
	}
	e.writePt(ems.ID+"_RemoteSts", e.readPt(ems.ID+"_RemoteCtl"))
}
