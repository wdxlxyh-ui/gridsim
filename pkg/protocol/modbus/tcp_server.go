package modbus

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

type ModbusTCPServer struct {
	port      int
	slaveID   uint8
	byteOrder string
	store     *library.Store

	mu        sync.Mutex
	conn      net.Conn
	connected bool
	connMu    sync.RWMutex

	startTime   time.Time
	interrogCnt atomic.Int64
	controlCnt  atomic.Int64
	spontCnt    atomic.Int64

	listener net.Listener
	stopCh   chan struct{}
}

func NewTCPServer(port int, slaveID uint8, byteOrder string) *ModbusTCPServer {
	return &ModbusTCPServer{
		port:      port,
		slaveID:   slaveID,
		byteOrder: byteOrder,
		stopCh:    make(chan struct{}),
	}
}

func (s *ModbusTCPServer) Name() string { return "modbus_tcp" }

func (s *ModbusTCPServer) SetStore(store *library.Store) {
	s.store = store
}

func (s *ModbusTCPServer) Start() error {
	s.startTime = time.Now()
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("listen tcp :%d: %w", s.port, err)
	}
	s.listener = ln
	slog.Info("Modbus TCP 服务端已启动", "port", s.port, "slave_id", s.slaveID)
	go s.acceptLoop(ln)
	return nil
}

func (s *ModbusTCPServer) Stop() {
	slog.Info("正在停止 Modbus TCP 服务端", "port", s.port)
	if s.listener != nil {
		s.listener.Close()
	}
	close(s.stopCh)
	s.connMu.Lock()
	if s.conn != nil {
		s.conn.Close()
	}
	s.connected = false
	s.conn = nil
	s.connMu.Unlock()
	slog.Info("Modbus TCP 服务端已停止", "port", s.port)
}

func (s *ModbusTCPServer) ClientConnected() bool {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.connected
}

func (s *ModbusTCPServer) ClientAddr() string {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	if s.conn == nil {
		return ""
	}
	return s.conn.RemoteAddr().String()
}

func (s *ModbusTCPServer) Stats() (interrog, control, spont int64) {
	return s.interrogCnt.Load(), s.controlCnt.Load(), s.spontCnt.Load()
}

func (s *ModbusTCPServer) Uptime() int64 {
	return int64(time.Since(s.startTime).Seconds())
}

func (s *ModbusTCPServer) Publish(point *config.Point) {
	s.spontCnt.Add(1)
}

func (s *ModbusTCPServer) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				slog.Warn("Modbus TCP accept error", "error", err)
				continue
			}
		}

		s.connMu.Lock()
		if s.connected {
			s.connMu.Unlock()
			slog.Warn("Modbus TCP: 已有客户端连接，拒绝新连接", "remote", conn.RemoteAddr())
			conn.Close()
			continue
		}
		s.conn = conn
		s.connected = true
		s.connMu.Unlock()

		slog.Info("Modbus TCP 客户端已连接", "remote", conn.RemoteAddr())
		s.handleConnection(conn)

		s.connMu.Lock()
		s.connected = false
		s.conn = nil
		s.connMu.Unlock()
		slog.Info("Modbus TCP 客户端已断开", "remote", conn.RemoteAddr())
	}
}

func (s *ModbusTCPServer) handleConnection(conn net.Conn) {
	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		mbap := make([]byte, 7)
		if _, err := io.ReadFull(conn, mbap); err != nil {
			return
		}

		// Modbus TCP requires protocol ID 0 and an MBAP length containing
		// one unit-ID byte plus a non-empty PDU (maximum PDU size: 253).
		if binary.BigEndian.Uint16(mbap[2:4]) != 0 {
			return
		}
		length := binary.BigEndian.Uint16(mbap[4:6])
		if length < 2 || length > 254 {
			return
		}
		pdu := make([]byte, int(length)-1)
		if _, err := io.ReadFull(conn, pdu); err != nil {
			return
		}

		unitID := mbap[6]
		functionCode := pdu[0]
		data := pdu[1:]

		response := s.handleRequest(functionCode, data)
		if response == nil {
			continue
		}

		respLen := len(response) + 1
		respMBAP := make([]byte, 7)
		copy(respMBAP[0:2], mbap[0:2])
		binary.BigEndian.PutUint16(respMBAP[2:4], 0)
		binary.BigEndian.PutUint16(respMBAP[4:6], uint16(respLen))
		respMBAP[6] = unitID

		resp := append(respMBAP, response...)
		if _, err := conn.Write(resp); err != nil {
			return
		}
	}
}

func (s *ModbusTCPServer) handleRequest(fc uint8, data []byte) []byte {
	switch fc {
	case 0x01:
		s.interrogCnt.Add(1)
		return s.readCoils(data)
	case 0x02:
		s.interrogCnt.Add(1)
		return s.readDiscreteInputs(data)
	case 0x03:
		s.interrogCnt.Add(1)
		return s.readHoldingRegisters(data)
	case 0x04:
		s.interrogCnt.Add(1)
		return s.readInputRegisters(data)
	case 0x05:
		return s.writeSingleCoil(data)
	case 0x06:
		return s.writeSingleRegister(data)
	case 0x0F:
		return s.writeMultipleCoils(data)
	case 0x10:
		return s.writeMultipleRegisters(data)
	default:
		return s.errorResponse(fc, 0x01)
	}
}

func (s *ModbusTCPServer) readCoils(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x01, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity < 1 || quantity > 2000 {
		return s.errorResponse(0x01, 0x03)
	}

	points := s.store.GetByFunctionCodeRange(0x01, startAddr, quantity)
	byteCount := (quantity + 7) / 8
	coilBytes := make([]byte, byteCount)

	for _, p := range points {
		offset := p.RegisterAddress - startAddr
		if offset < quantity {
			if p.BoolValue {
				coilBytes[offset/8] |= (1 << (offset % 8))
			}
		}
	}

	resp := make([]byte, 2+byteCount)
	resp[0] = 0x01
	resp[1] = byte(byteCount)
	copy(resp[2:], coilBytes)
	return resp
}

func (s *ModbusTCPServer) readDiscreteInputs(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x02, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity < 1 || quantity > 2000 {
		return s.errorResponse(0x02, 0x03)
	}

	points := s.store.GetByFunctionCodeRange(0x02, startAddr, quantity)
	byteCount := (quantity + 7) / 8
	coilBytes := make([]byte, byteCount)

	for _, p := range points {
		offset := p.RegisterAddress - startAddr
		if offset < quantity {
			if p.BoolValue {
				coilBytes[offset/8] |= (1 << (offset % 8))
			}
		}
	}

	resp := make([]byte, 2+byteCount)
	resp[0] = 0x02
	resp[1] = byte(byteCount)
	copy(resp[2:], coilBytes)
	return resp
}

func (s *ModbusTCPServer) readHoldingRegisters(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x03, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity < 1 || quantity > 125 {
		return s.errorResponse(0x03, 0x03)
	}

	points := s.store.GetByFunctionCodeRange(0x03, startAddr, quantity)
	regBytes := make([]byte, quantity*2)

	for _, p := range points {
		offset := p.RegisterAddress - startAddr
		if offset < quantity {
			var regs []uint16
			switch p.PointType {
			case config.TypeAI, config.TypeAO:
				regs = Float32ToRegisters(float32(p.Value), s.byteOrder)
			case config.TypePI:
				regs = Int32ToRegisters(p.IntValue, s.byteOrder)
			case config.TypeDI, config.TypeDO:
				val := uint16(0)
				if p.BoolValue {
					val = 0xFFFF
				}
				regs = []uint16{val}
			}
			for i, r := range regs {
				idx := offset + uint16(i)
				if idx < quantity {
					binary.BigEndian.PutUint16(regBytes[idx*2:], r)
				}
			}
		}
	}

	resp := make([]byte, 2+quantity*2)
	resp[0] = 0x03
	resp[1] = byte(quantity * 2)
	copy(resp[2:], regBytes)
	return resp
}

func (s *ModbusTCPServer) readInputRegisters(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x04, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity < 1 || quantity > 125 {
		return s.errorResponse(0x04, 0x03)
	}

	points := s.store.GetByFunctionCodeRange(0x04, startAddr, quantity)
	regBytes := make([]byte, quantity*2)

	for _, p := range points {
		offset := p.RegisterAddress - startAddr
		if offset < quantity {
			var regs []uint16
			switch p.PointType {
			case config.TypeAI, config.TypeAO:
				regs = Float32ToRegisters(float32(p.Value), s.byteOrder)
			case config.TypePI:
				regs = Int32ToRegisters(p.IntValue, s.byteOrder)
			case config.TypeDI, config.TypeDO:
				val := uint16(0)
				if p.BoolValue {
					val = 0xFFFF
				}
				regs = []uint16{val}
			}
			for i, r := range regs {
				idx := offset + uint16(i)
				if idx < quantity {
					binary.BigEndian.PutUint16(regBytes[idx*2:], r)
				}
			}
		}
	}

	resp := make([]byte, 2+quantity*2)
	resp[0] = 0x04
	resp[1] = byte(quantity * 2)
	copy(resp[2:], regBytes)
	return resp
}

func (s *ModbusTCPServer) writeSingleCoil(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x05, 0x02)
	}
	addr := binary.BigEndian.Uint16(data[0:2])
	value := binary.BigEndian.Uint16(data[2:4])
	if value != 0x0000 && value != 0xFF00 {
		return s.errorResponse(0x05, 0x03)
	}

	pt, err := s.store.GetByFunctionCodeAndAddress(0x05, addr)
	if err != nil {
		pt, err = s.store.GetByFunctionCodeAndAddress(0x01, addr)
	}
	if err != nil {
		return s.errorResponse(0x05, 0x02)
	}

	on := value == 0xFF00
	s.store.SetBoolValue(pt.IOA, on)
	s.controlCnt.Add(1)
	slog.Info("Modbus TCP 写线圈", "ioa", pt.IOA, "addr", addr, "value", on)

	resp := make([]byte, 4)
	resp[0] = 0x05
	copy(resp[1:], data[:4])
	return resp
}

func (s *ModbusTCPServer) writeSingleRegister(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x06, 0x02)
	}
	addr := binary.BigEndian.Uint16(data[0:2])

	pt, err := s.store.GetByFunctionCodeAndAddress(0x06, addr)
	if err != nil {
		pt, err = s.store.GetByFunctionCodeAndAddress(0x03, addr)
	}
	if err != nil {
		return s.errorResponse(0x06, 0x02)
	}

	// FC06 carries exactly one 16-bit register. GridSim numeric points are
	// represented as 32-bit FLOAT/INT values, so writing them with FC06
	// would be incomplete. Require FC16 instead of reading past the buffer.
	switch pt.PointType {
	case config.TypeAI, config.TypeAO, config.TypePI:
		return s.errorResponse(0x06, 0x03)
	default:
		return s.errorResponse(0x06, 0x02)
	}
}

func (s *ModbusTCPServer) writeMultipleCoils(data []byte) []byte {
	if len(data) < 5 {
		return s.errorResponse(0x0F, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	byteCount := data[4]
	if quantity < 1 || quantity > 1968 {
		return s.errorResponse(0x0F, 0x03)
	}
	expectedBytes := int((quantity + 7) / 8)
	if int(byteCount) != expectedBytes || len(data) != 5+expectedBytes {
		return s.errorResponse(0x0F, 0x03)
	}

	coilData := data[5:]
	written := 0
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		on := (coilData[i/8] & (1 << (i % 8))) != 0

		pt, err := s.store.GetByFunctionCodeAndAddress(0x0F, addr)
		if err != nil {
			pt, err = s.store.GetByFunctionCodeAndAddress(0x05, addr)
		}
		if err != nil {
			pt, err = s.store.GetByFunctionCodeAndAddress(0x01, addr)
		}
		if err != nil {
			continue
		}
		s.store.SetBoolValue(pt.IOA, on)
		written++
	}

	s.controlCnt.Add(int64(written))
	slog.Info("Modbus TCP 写多个线圈", "start", startAddr, "quantity", quantity, "written", written)

	resp := make([]byte, 5)
	resp[0] = 0x0F
	binary.BigEndian.PutUint16(resp[1:3], startAddr)
	binary.BigEndian.PutUint16(resp[3:5], quantity)
	return resp
}

func (s *ModbusTCPServer) writeMultipleRegisters(data []byte) []byte {
	if len(data) < 5 {
		return s.errorResponse(0x10, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	byteCount := data[4]
	if quantity < 1 || quantity > 123 {
		return s.errorResponse(0x10, 0x03)
	}
	expectedBytes := int(quantity) * 2
	if int(byteCount) != expectedBytes || len(data) != 5+expectedBytes {
		return s.errorResponse(0x10, 0x03)
	}

	type pendingRegisterWrite struct {
		point    *config.Point
		value    float64
		intValue int32
		isInt    bool
	}

	regData := data[5:]
	pending := make([]pendingRegisterWrite, 0)
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i

		pt, err := s.store.GetByFunctionCodeAndAddress(0x10, addr)
		if err != nil {
			pt, err = s.store.GetByFunctionCodeAndAddress(0x03, addr)
		}
		if err != nil {
			continue
		}

		// Validate and decode the entire request before modifying the store.
		if i+1 >= quantity {
			return s.errorResponse(0x10, 0x03)
		}
		regs := []uint16{
			binary.BigEndian.Uint16(regData[i*2 : i*2+2]),
			binary.BigEndian.Uint16(regData[(i+1)*2 : (i+1)*2+2]),
		}
		switch pt.PointType {
		case config.TypeAI, config.TypeAO:
			pending = append(pending, pendingRegisterWrite{
				point: pt,
				value: float64(RegistersToFloat32(regs, s.byteOrder)),
			})
		case config.TypePI:
			pending = append(pending, pendingRegisterWrite{
				point: pt, intValue: RegistersToInt32(regs, s.byteOrder), isInt: true,
			})
		default:
			return s.errorResponse(0x10, 0x02)
		}
		i++ // second register is part of the same 32-bit value
	}

	for _, update := range pending {
		if update.isInt {
			s.store.SetIntValue(update.point.IOA, update.intValue)
		} else {
			s.store.SetValue(update.point.IOA, update.value)
		}
	}
	written := len(pending)

	s.controlCnt.Add(int64(written))
	slog.Info("Modbus TCP 写多个寄存器", "start", startAddr, "quantity", quantity, "written", written)

	resp := make([]byte, 5)
	resp[0] = 0x10
	binary.BigEndian.PutUint16(resp[1:3], startAddr)
	binary.BigEndian.PutUint16(resp[3:5], quantity)
	return resp
}

func (s *ModbusTCPServer) errorResponse(fc uint8, code uint8) []byte {
	return []byte{fc | 0x80, code}
}

func (s *ModbusTCPServer) Port() int {
	return s.port
}

func (s *ModbusTCPServer) Addr() string {
	return ":" + strconv.Itoa(s.port)
}
