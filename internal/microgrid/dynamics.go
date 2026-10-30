package microgrid

// 设备动态特性: 响应延迟、爬坡率、风电理论功率曲线。
// 动态参数全为零时 shape 原样返回目标值, 与旧行为一致。

type delayedCmd struct {
	atMs  int64
	value float64
}

// rampToward 按爬坡率把 cur 推向 target。up/down 作用于数值上升/下降方向;
// 只配置一个方向时两个方向共用; 都为零表示不限速。
func rampToward(cur, target, up, down, dtSec float64) float64 {
	if up <= 0 && down <= 0 {
		return target
	}
	if up <= 0 {
		up = down
	}
	if down <= 0 {
		down = up
	}
	delta := target - cur
	switch {
	case delta > 0 && delta > up*dtSec:
		return cur + up*dtSec
	case delta < 0 && -delta > down*dtSec:
		return cur - down*dtSec
	}
	return target
}

// windTheoryPower 切入~额定风速之间按风速立方插值, 额定~切出恒定额定功率, 其余为 0。
func windTheoryPower(speed float64, p DeviceParams) float64 {
	cutIn, rated, cutOut := p.windCurve()
	if speed < cutIn || speed >= cutOut {
		return 0
	}
	if speed >= rated {
		return p.RatedPowerKW
	}
	cube := func(v float64) float64 { return v * v * v }
	return p.RatedPowerKW * (cube(speed) - cube(cutIn)) / (cube(rated) - cube(cutIn))
}

// delayed 返回已经"到期"的最新指令; 第二个返回值为 false 表示首条指令尚未到期(应保持当前功率)。
func (e *Engine) delayed(devID string, delayMs int, target float64) (float64, bool) {
	if e.dyn == nil {
		e.dyn = make(map[string][]delayedCmd)
	}
	q := append(e.dyn[devID], delayedCmd{atMs: e.simMs, value: target})
	cut := e.simMs - int64(delayMs)
	idx := -1
	for i, c := range q {
		if c.atMs <= cut {
			idx = i
		}
	}
	if idx < 0 {
		e.dyn[devID] = q
		return 0, false
	}
	e.dyn[devID] = q[idx:]
	return q[idx].value, true
}

// shape 对目标功率施加响应延迟与爬坡限制。
func (e *Engine) shape(dev Device, cur, target, dtSec float64) float64 {
	p := dev.Params
	if p.ResponseDelayMs <= 0 && p.RampUpKWPerS <= 0 && p.RampDownKWPerS <= 0 {
		return target
	}
	eff, ok := e.delayed(dev.ID, p.ResponseDelayMs, target)
	if !ok {
		return cur
	}
	return rampToward(cur, eff, p.RampUpKWPerS, p.RampDownKWPerS, dtSec)
}

// resetDyn 清除设备的待生效指令(开关断开、置数时调用)。
func (e *Engine) resetDyn(devID string) {
	delete(e.dyn, devID)
}

// tickSeconds 单个 tick 对应的仿真时间(秒), 与 updateSOC 使用同一口径。
func (e *Engine) tickSeconds() float64 {
	if e.cfg.TickMs <= 0 {
		return 1
	}
	return float64(e.cfg.TickMs) / 1000.0
}

// batteryLimits 返回 SOC 保护下的最大可充/可放功率(均为非负)。
// 剩余电量不足一个 tick 时按剩余电量折算, 保证 SOC 恰好停在限值。
func (e *Engine) batteryLimits(dev Device) (maxCharge, maxDischarge float64) {
	ratedP := dev.Params.RatedPowerKW_B
	if ratedP <= 0 {
		ratedP = 50
	}
	maxCharge, maxDischarge = ratedP, ratedP
	if !dev.Params.SOCProtection || dev.Params.CapacityKWH <= 0 {
		return
	}
	s, ok := e.soc[dev.ID]
	if !ok {
		return
	}
	dtH := e.tickSeconds() / 3600.0
	if room := (dev.Params.SOCMax - s) / 100 * dev.Params.CapacityKWH / dtH; room < maxCharge {
		maxCharge = room
	}
	if room := (s - dev.Params.SOCMin) / 100 * dev.Params.CapacityKWH / dtH; room < maxDischarge {
		maxDischarge = room
	}
	if maxCharge < 0 {
		maxCharge = 0
	}
	if maxDischarge < 0 {
		maxDischarge = 0
	}
	return
}
