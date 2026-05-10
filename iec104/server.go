package iec104

import (
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"iec104-sim/config"
	"iec104-sim/library"

	"github.com/wendy512/go-iecp5/asdu"
	"github.com/wendy512/go-iecp5/cs104"
)

type Server struct {
	port        int
	store       *library.Store
	mu          sync.RWMutex
	connect     asdu.Connect
	connected   bool
	publishCh   chan *config.Point
	stopCh      chan struct{}
	interrogCnt atomic.Int64
	controlCnt  atomic.Int64
	spontCnt    atomic.Int64
	startTime   time.Time
	server      *cs104.Server
}

func NewServer(port int, store *library.Store) *Server {
	return &Server{
		port:      port,
		store:     store,
		publishCh: make(chan *config.Point, 1024),
		stopCh:    make(chan struct{}),
	}
}

func (s *Server) Start() error {
	s.startTime = time.Now()

	handler := &serverHandler{srv: s}
	s.server = cs104.NewServer(handler)
	s.server.SetOnConnectionHandler(s.onConnect)
	s.server.SetConnectionLostHandler(s.onDisconnect)

	go s.publishLoop()

	go func() {
		slog.Info("IEC104 服务端已启动", "port", s.port)
		s.server.ListenAndServer(s.addr())
	}()

	return nil
}

func (s *Server) addr() string {
	return ":" + strconv.Itoa(s.port)
}

func (s *Server) onConnect(c asdu.Connect) {
	s.mu.Lock()
	if s.connected {
		s.mu.Unlock()
		slog.Warn("已有客户端连接，拒绝新连接", "remote", c.UnderlyingConn().RemoteAddr())
		return
	}
	s.connect = c
	s.connected = true
	s.mu.Unlock()

	slog.Info("客户端已连接", "remote", c.UnderlyingConn().RemoteAddr())
}

func (s *Server) onDisconnect(c asdu.Connect) {
	s.mu.Lock()
	s.connected = false
	s.connect = nil
	s.mu.Unlock()

	slog.Info("客户端已断开", "remote", c.UnderlyingConn().RemoteAddr())
}

func (s *Server) publishLoop() {
	for {
		select {
		case pt := <-s.publishCh:
			c := s.getConnect()
			if c == nil {
				continue
			}
			s.sendSpontaneous(c, pt)
		case <-s.stopCh:
			return
		}
	}
}

func (s *Server) sendSpontaneous(c asdu.Connect, pt *config.Point) {
	coa := asdu.CauseOfTransmission{Cause: asdu.Spontaneous}
	commonAddr := asdu.CommonAddr(1)

	var err error
	switch pt.PointType {
	case config.TypeAI, config.TypeAO:
		info := asdu.MeasuredValueFloatInfo{
			Ioa:   asdu.InfoObjAddr(pt.IOA),
			Value: float32(pt.Value),
			Qds:   qualityToQDS(pt.QDS),
		}
		err = asdu.MeasuredValueFloat(c, false, coa, commonAddr, info)

	case config.TypeDI, config.TypeDO:
		info := asdu.SinglePointInfo{
			Ioa:   asdu.InfoObjAddr(pt.IOA),
			Value: pt.BoolValue,
			Qds:   qualityToQDS(pt.QDS),
		}
		err = asdu.Single(c, false, coa, commonAddr, info)

	case config.TypePI:
		info := asdu.BinaryCounterReadingInfo{
			Ioa: asdu.InfoObjAddr(pt.IOA),
		}
		err = asdu.IntegratedTotals(c, false, coa, commonAddr, info)
	}

	if err != nil {
		slog.Warn("发送变化上送失败", "ioa", pt.IOA, "error", err)
	} else {
		s.spontCnt.Add(1)
		slog.Info("变化上送", "ioa", pt.IOA, "value", formatPointValue(pt))
	}
}

func (s *Server) getConnect() asdu.Connect {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connect
}

func (s *Server) Publish(point *config.Point) {
	s.mu.RLock()
	connected := s.connected
	s.mu.RUnlock()
	if !connected {
		return
	}
	select {
	case s.publishCh <- point:
	default:
		slog.Warn("发布通道已满，丢弃变化上送", "ioa", point.IOA)
	}
}

func (s *Server) Stop() {
	close(s.stopCh)
}

func (s *Server) ClientConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

func (s *Server) ClientAddr() string {
	c := s.getConnect()
	if c == nil {
		return ""
	}
	return c.UnderlyingConn().RemoteAddr().String()
}

func (s *Server) Stats() (interrog, control, spont int64) {
	return s.interrogCnt.Load(), s.controlCnt.Load(), s.spontCnt.Load()
}

func (s *Server) Uptime() int64 {
	return int64(time.Since(s.startTime).Seconds())
}

func (s *Server) Port() int {
	return s.port
}

func formatPointValue(pt *config.Point) interface{} {
	switch pt.PointType {
	case config.TypeAI, config.TypeAO:
		return pt.Value
	case config.TypeDI, config.TypeDO:
		return pt.BoolValue
	case config.TypePI:
		return pt.IntValue
	default:
		return pt.Value
	}
}

// serverHandler implements cs104.ServerHandlerInterface
type serverHandler struct {
	srv *Server
}

func (h *serverHandler) InterrogationHandler(c asdu.Connect, a *asdu.ASDU, qoi asdu.QualifierOfInterrogation) error {
	slog.Info("收到总召", "remote", c.UnderlyingConn().RemoteAddr(), "qoi", qoi)
	h.srv.interrogCnt.Add(1)

	// 1) 先发 ACT_CON（C_IC_NA_1 COT=7）
	if err := a.SendReplyMirror(c, asdu.ActivationCon); err != nil {
		slog.Warn("发送总召 ACT_CON 失败", "error", err)
	}

	// 2) 分组发送数据，COT 必须为"响应站召唤(20)"，
	//    wendy512/go-iecp5 对 Single/MeasuredValueFloat/IntegratedTotals 的 COT 做了严格校验，
	//    用 ActivationCon 会直接被拒绝返回 ErrCmdCause
	coa := asdu.CauseOfTransmission{Cause: asdu.InterrogatedByStation}
	commonAddr := a.CommonAddr

	sendPointsByType(c, h.srv.store, config.TypeAI, coa, commonAddr)
	sendPointsByType(c, h.srv.store, config.TypeDI, coa, commonAddr)
	sendPointsByType(c, h.srv.store, config.TypeAO, coa, commonAddr)
	sendPointsByType(c, h.srv.store, config.TypeDO, coa, commonAddr)

	// 3) 发 ACT_TERM（C_IC_NA_1 COT=10）
	if err := a.SendReplyMirror(c, asdu.ActivationTerm); err != nil {
		slog.Warn("发送总召 ACT_TERM 失败", "error", err)
	}

	slog.Info("总召完成", "remote", c.UnderlyingConn().RemoteAddr(), "totalPoints", h.srv.store.TotalCount())
	return nil
}

func (h *serverHandler) CounterInterrogationHandler(c asdu.Connect, a *asdu.ASDU, qcc asdu.QualifierCountCall) error {
	slog.Info("收到电度召唤", "remote", c.UnderlyingConn().RemoteAddr())

	// 先发 ACT_CON
	if err := a.SendReplyMirror(c, asdu.ActivationCon); err != nil {
		slog.Warn("发送电度召唤 ACT_CON 失败", "error", err)
	}

	// COT = RequestByGeneralCounter(37) 响应总计数量召唤
	coa := asdu.CauseOfTransmission{Cause: asdu.RequestByGeneralCounter}
	commonAddr := a.CommonAddr
	sendPointsByType(c, h.srv.store, config.TypePI, coa, commonAddr)

	// 发 ACT_TERM
	if err := a.SendReplyMirror(c, asdu.ActivationTerm); err != nil {
		slog.Warn("发送电度召唤 ACT_TERM 失败", "error", err)
	}
	return nil
}

func (h *serverHandler) ReadHandler(c asdu.Connect, a *asdu.ASDU, ioa asdu.InfoObjAddr) error {
	return nil
}

func (h *serverHandler) ClockSyncHandler(c asdu.Connect, a *asdu.ASDU, t time.Time) error {
	slog.Debug("收到时钟同步", "remote", c.UnderlyingConn().RemoteAddr())
	return nil
}

func (h *serverHandler) ResetProcessHandler(c asdu.Connect, a *asdu.ASDU, qrp asdu.QualifierOfResetProcessCmd) error {
	return nil
}

func (h *serverHandler) DelayAcquisitionHandler(c asdu.Connect, a *asdu.ASDU, delay uint16) error {
	return nil
}

func (h *serverHandler) ASDUHandler(c asdu.Connect, a *asdu.ASDU) error {
	switch a.Type {
	case asdu.C_SC_NA_1:
		return h.handleSingleCommand(c, a)
	case asdu.C_SE_NC_1:
		return h.handleSetpointCommand(c, a)
	default:
		slog.Debug("收到未处理的ASDU", "type", a.Type)
	}
	return nil
}

func (h *serverHandler) handleSingleCommand(c asdu.Connect, a *asdu.ASDU) error {
	// 先 Clone 一份用于 ACT_TERM：Get* 方法会消费原 ASDU 的 infoObj，
	// 后续若用原 ASDU 发镜像回复，报文将不含 IOA/值/QoS。
	mirror := a.Clone()

	// 1) ACT_CON：在 infoObj 被消费前发送，回复携带完整信息体
	if err := a.SendReplyMirror(c, asdu.ActivationCon); err != nil {
		slog.Warn("发送DO ACT_CON失败", "error", err)
	}

	cmd := a.GetSingleCmd()
	ioa := uint32(cmd.Ioa)
	slog.Info("收到DO控制", "ioa", ioa, "value", cmd.Value)

	// 2) 执行：更新 Store
	pt, err := h.srv.store.SetBoolValue(ioa, cmd.Value)
	if err != nil {
		slog.Warn("DO控制更新失败", "ioa", ioa, "error", err)
	}

	// 3) ACT_TERM
	if err := mirror.SendReplyMirror(c, asdu.ActivationTerm); err != nil {
		slog.Warn("发送DO ACT_TERM失败", "error", err)
	}

	// 4) 变化上送（COT=3 spontaneous，使用监视方向 TypeID M_SP_NA_1）
	//    按 IEC104 规范流程：ACT_CON → 执行 → ACT_TERM → 变化上送
	h.srv.controlCnt.Add(1)
	if pt != nil {
		h.srv.Publish(pt)
	}
	return nil
}

func (h *serverHandler) handleSetpointCommand(c asdu.Connect, a *asdu.ASDU) error {
	mirror := a.Clone()

	// 1) ACT_CON：在 infoObj 被消费前发送，回复携带完整信息体
	if err := a.SendReplyMirror(c, asdu.ActivationCon); err != nil {
		slog.Warn("发送AO ACT_CON失败", "error", err)
	}

	cmd := a.GetSetpointFloatCmd()
	ioa := uint32(cmd.Ioa)
	slog.Info("收到AO控制", "ioa", ioa, "value", cmd.Value)

	// 2) 执行：更新 Store
	pt, err := h.srv.store.SetValue(ioa, float64(cmd.Value))
	if err != nil {
		slog.Warn("AO控制更新失败", "ioa", ioa, "error", err)
	}

	// 3) ACT_TERM
	if err := mirror.SendReplyMirror(c, asdu.ActivationTerm); err != nil {
		slog.Warn("发送AO ACT_TERM失败", "error", err)
	}

	// 4) 变化上送（COT=3 spontaneous，使用监视方向 TypeID M_ME_NC_1）
	//    按 IEC104 规范流程：ACT_CON → 执行 → ACT_TERM → 变化上送
	h.srv.controlCnt.Add(1)
	if pt != nil {
		h.srv.Publish(pt)
	}
	return nil
}

func sendPointsByType(c asdu.Connect, store *library.Store, pt config.PointType, coa asdu.CauseOfTransmission, commonAddr asdu.CommonAddr) {
	// 使用快照，避免与并发写者竞争读取 Point 内部字段
	for _, point := range store.SnapshotByType(pt) {
		switch pt {
		case config.TypeAI, config.TypeAO:
			info := asdu.MeasuredValueFloatInfo{
				Ioa:   asdu.InfoObjAddr(point.IOA),
				Value: float32(point.Value),
				Qds:   qualityToQDS(point.QDS),
			}
			asdu.MeasuredValueFloat(c, false, coa, commonAddr, info)

		case config.TypeDI, config.TypeDO:
			info := asdu.SinglePointInfo{
				Ioa:   asdu.InfoObjAddr(point.IOA),
				Value: point.BoolValue,
				Qds:   qualityToQDS(point.QDS),
			}
			asdu.Single(c, false, coa, commonAddr, info)

		case config.TypePI:
			info := asdu.BinaryCounterReadingInfo{
				Ioa: asdu.InfoObjAddr(point.IOA),
			}
			asdu.IntegratedTotals(c, false, coa, commonAddr, info)
		}
	}
}

func qualityToQDS(q config.QualityDescriptor) asdu.QualityDescriptor {
	var qds asdu.QualityDescriptor
	if q.Invalid {
		qds |= asdu.QDSInvalid
	}
	if q.NotTopical {
		qds |= asdu.QDSNotTopical
	}
	if q.Substituted {
		qds |= asdu.QDSSubstituted
	}
	if q.Overflow {
		qds |= asdu.QDSOverflow
	}
	if q.Blocked {
		qds |= asdu.QDSBlocked
	}
	return qds
}
