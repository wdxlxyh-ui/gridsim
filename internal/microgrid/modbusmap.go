package microgrid

import (
	"sort"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

// Modbus 映射规则(确定性, 与 IOA 一一对应):
//   AI → FC04 输入寄存器, AO → FC03 保持寄存器(FC06/16 写), DI → FC02 离散输入, DO → FC01 线圈(FC05/15 写)
//   数值点为 float32, 占 2 个寄存器, 起始地址 = IOA×2; 遥信/遥控点地址 = IOA。
func modbusRegister(p *config.Point) (fc uint8, addr uint16) {
	switch p.PointType {
	case config.TypeAI:
		return 4, uint16(p.IOA * 2)
	case config.TypeAO:
		return 3, uint16(p.IOA * 2)
	case config.TypeDI:
		return 2, uint16(p.IOA)
	case config.TypeDO:
		return 1, uint16(p.IOA)
	}
	return 0, 0
}

// AssignModbusMap 为 store 中的所有点写入 FunctionCode/RegisterAddress 并建立 Modbus 索引。
// 点集合在启用 Modbus 的实例启动前一次性处理; 热加载新增的点需要重启实例后才有 Modbus 地址。
func AssignModbusMap(store *library.Store) {
	pts := store.GetAll()
	sort.Slice(pts, func(i, j int) bool { return pts[i].IOA < pts[j].IOA })
	for _, p := range pts {
		if p.IOA*2+1 > 0xFFFF {
			continue // 超出 16 位地址空间
		}
		p.FunctionCode, p.RegisterAddress = modbusRegister(p)
		store.ReindexModbus(p)
	}
}
