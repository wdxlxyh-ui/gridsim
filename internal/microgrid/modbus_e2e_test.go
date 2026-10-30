package microgrid

import (
	"encoding/binary"
	"math"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"gridsim/pkg/protocol/modbus"
)

// 原始 Modbus TCP 客户端: 只依赖标准库, 独立于被测服务端的编解码实现之外手工拼帧。
type mbClient struct {
	t    *testing.T
	conn net.Conn
	tid  uint16
	ord  string
}

func dialModbus(t *testing.T, port int, order string) *mbClient {
	t.Helper()
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return &mbClient{t: t, conn: c, ord: order}
}

func (m *mbClient) call(pdu []byte) []byte {
	m.t.Helper()
	m.tid++
	req := make([]byte, 7+len(pdu))
	binary.BigEndian.PutUint16(req[0:], m.tid)
	binary.BigEndian.PutUint16(req[4:], uint16(1+len(pdu)))
	req[6] = 1 // unit id
	copy(req[7:], pdu)
	m.conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := m.conn.Write(req); err != nil {
		m.t.Fatalf("write: %v", err)
	}
	hdr := make([]byte, 7)
	if _, err := io.ReadFull(m.conn, hdr); err != nil {
		m.t.Fatalf("read header: %v", err)
	}
	body := make([]byte, int(binary.BigEndian.Uint16(hdr[4:6]))-1)
	if _, err := io.ReadFull(m.conn, body); err != nil {
		m.t.Fatalf("read body: %v", err)
	}
	return body
}

func (m *mbClient) readFloat(fc byte, ioa uint32) float64 {
	m.t.Helper()
	addr := uint16(ioa * 2)
	resp := m.call([]byte{fc, byte(addr >> 8), byte(addr), 0, 2})
	if resp[0] != fc {
		m.t.Fatalf("FC%d read IOA %d: exception %x", fc, ioa, resp)
	}
	regs := []uint16{binary.BigEndian.Uint16(resp[2:]), binary.BigEndian.Uint16(resp[4:])}
	b := []byte{byte(regs[0] >> 8), byte(regs[0]), byte(regs[1] >> 8), byte(regs[1])}
	if m.ord == "CDAB" {
		b = []byte{b[2], b[3], b[0], b[1]}
	}
	return float64(float32frombytes(b))
}

func float32frombytes(b []byte) float32 {
	return math.Float32frombits(uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]))
}

func (m *mbClient) writeFloat(ioa uint32, v float32) {
	m.t.Helper()
	addr := uint16(ioa * 2)
	regs := modbus.Float32ToRegisters(v, m.ord)
	pdu := []byte{0x10, byte(addr >> 8), byte(addr), 0, 2, 4,
		byte(regs[0] >> 8), byte(regs[0]), byte(regs[1] >> 8), byte(regs[1])}
	if resp := m.call(pdu); resp[0] != 0x10 {
		m.t.Fatalf("FC16 write IOA %d: exception %x", ioa, resp)
	}
}

func (m *mbClient) readBits(fc byte, ioa uint32) bool {
	m.t.Helper()
	resp := m.call([]byte{fc, byte(ioa >> 8), byte(ioa), 0, 1})
	if resp[0] != fc {
		m.t.Fatalf("FC%d read IOA %d: exception %x", fc, ioa, resp)
	}
	return resp[2]&1 == 1
}

func (m *mbClient) writeCoil(ioa uint32, on bool) {
	m.t.Helper()
	v := byte(0)
	if on {
		v = 0xFF
	}
	if resp := m.call([]byte{0x05, byte(ioa >> 8), byte(ioa), v, 0}); resp[0] != 0x05 {
		m.t.Fatalf("FC05 write IOA %d: exception %x", ioa, resp)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startModbusEngine(t *testing.T, topo *Topology, order string) (*Engine, *mbClient) {
	t.Helper()
	store := topo.StoreFromTopology()
	AssignModbusMap(store)
	port := freePort(t)
	srv := modbus.NewTCPServer(port, 1, order)
	srv.SetStore(store)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Stop)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()
	return eng, dialModbus(t, port, order)
}

// 调度经 Modbus 下发站级设定, 引擎 tick 后再经 Modbus 读回各设备与并网点功率。
func TestModbus_DispatchRoundTrip(t *testing.T) {
	topo := emsTopology()
	eng, mb := startModbusEngine(t, topo, "ABCD")
	ioa := func(name string) uint32 { return eng.pointIOA[name] }

	mb.writeFloat(ioa(nm(topo, "w1", "_风速")), 12)
	mb.writeFloat(ioa(nm(topo, "w2", "_风速")), 12)
	mb.writeFloat(ioa(nm(topo, "ems-w", "_功率设定")), 150)
	mb.writeFloat(ioa(nm(topo, "load1", "_功率设定")), 300)
	mb.writeFloat(ioa(nm(topo, "ems-p", "_功率设定")), 0) // 光伏 EMS 默认不限发, 本用例只看风电
	eng.tick()
	eng.tick()

	if got := mb.readFloat(4, ioa(nm(topo, "w1", "_有功功率"))); !near(got, 100, 0.01) {
		t.Errorf("wind1 via FC04 = %v, want 100", got)
	}
	if got := mb.readFloat(4, ioa(nm(topo, "ems-w", "_有功功率"))); !near(got, 150, 0.01) {
		t.Errorf("wind EMS via FC04 = %v, want 150", got)
	}
	if got := mb.readFloat(4, ioa("关口表_有功功率")); !near(got, 150, 0.11) {
		t.Errorf("grid meter via FC04 = %v, want 150 (load 300 - wind 150)", got)
	}
	if got := mb.readFloat(3, ioa(nm(topo, "ems-w", "_功率设定"))); !near(got, 150, 0.01) {
		t.Errorf("setpoint readback via FC03 = %v, want 150", got)
	}
}

func TestModbus_ByteOrderCDAB(t *testing.T) {
	topo := emsTopology()
	eng, mb := startModbusEngine(t, topo, "CDAB")
	ioa := eng.pointIOA[nm(topo, "ems-w", "_功率设定")]
	mb.writeFloat(ioa, 123.5)
	if got := eng.readPt(nm(topo, "ems-w", "_功率设定")); !near(got, 123.5, 1e-6) {
		t.Errorf("store value = %v, want 123.5", got)
	}
	if got := mb.readFloat(3, ioa); !near(got, 123.5, 1e-6) {
		t.Errorf("readback CDAB = %v, want 123.5", got)
	}
}

func TestModbus_CoilsAndDiscreteInputs(t *testing.T) {
	topo := emsTopology()
	topo.GridMeter.BreakerPoints = true
	eng, mb := startModbusEngine(t, topo, "ABCD")
	ioa := func(name string) uint32 { return eng.pointIOA[name] }
	eng.set(nm(topo, "ems-w", "_功率设定"), 0)
	eng.tick()

	if !mb.readBits(2, ioa("关口表_并网开关状态")) {
		t.Error("breaker status (FC02) should be closed")
	}
	mb.writeCoil(ioa("关口表_并网开关遥控"), false) // FC05 分闸
	eng.tick()
	if mb.readBits(2, ioa("关口表_并网开关状态")) {
		t.Error("breaker status should be open after FC05 write")
	}
	if !mb.readBits(2, ioa("关口表_孤岛状态")) {
		t.Error("island status (FC02) should be 1 when breaker is open")
	}
	if mb.readBits(1, ioa("关口表_并网开关遥控")) {
		t.Error("breaker coil (FC01) readback should be 0")
	}
}

// Modbus 只能写 AO(FC03/06/16)与 DO(FC01/05); 测量点(AI, FC04)只读。
// 经 Modbus 下发设定值生效, 置数(直接改写功率点, REST/测点 API 路径)仍立即生效。
func TestModbus_SetpointWriteAndManualSet(t *testing.T) {
	topo := dispatchTopology()
	eng, mb := startModbusEngine(t, topo, "ABCD")
	mb.writeFloat(eng.pointIOA["储能3_功率设定"], 50)
	eng.tick()
	if eng.batPower["bat1"] != 50 {
		t.Fatalf("setpoint over Modbus: battery=%v want 50", eng.batPower["bat1"])
	}
	// 试图经 Modbus 写 AI 点: 服务端忽略, 功率不受影响
	mb.writeFloat(eng.pointIOA["储能3_充放电功率"], 0)
	eng.tick()
	if eng.batPower["bat1"] != 50 {
		t.Errorf("AI point is read-only over Modbus: battery=%v want 50", eng.batPower["bat1"])
	}
	// 置数走 store 写入(与 REST 一致): 设定值 0 + 功率点写 0 → 立即为 0
	eng.set("储能3_功率设定", 0)
	eng.set("储能3_充放电功率", 0)
	eng.tick()
	if eng.batPower["bat1"] != 0 {
		t.Errorf("manual set to 0: battery=%v want 0", eng.batPower["bat1"])
	}
}

func TestModbus_MapIsDeterministicAndUnique(t *testing.T) {
	topo := emsTopology()
	topo.GridMeter.BreakerPoints = true
	store := topo.StoreFromTopology()
	AssignModbusMap(store)
	seen := map[[2]uint16]string{}
	for _, p := range store.GetAll() {
		if p.FunctionCode == 0 {
			t.Errorf("point %s has no Modbus mapping", p.Name)
			continue
		}
		span := uint16(1)
		if p.PointType == "AI" || p.PointType == "AO" {
			span = 2
		}
		for i := uint16(0); i < span; i++ {
			k := [2]uint16{uint16(p.FunctionCode), p.RegisterAddress + i}
			if prev, dup := seen[k]; dup {
				t.Errorf("Modbus address clash FC%d@%d: %s vs %s", k[0], k[1], prev, p.Name)
			}
			seen[k] = p.Name
		}
	}
}
