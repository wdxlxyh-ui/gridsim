package iec104

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"gridsim/internal/model"
	"gridsim/pkg/config"
	"gridsim/pkg/library"

	"github.com/wendy512/go-iecp5/asdu"
	"github.com/wendy512/go-iecp5/cs104"
)

// Client implements protocol.Protocol as an IEC104 master/client.
type Client struct {
	cfg    model.IEC104ClientConfig
	store  *library.Store
	client *cs104.Client

	mu        sync.RWMutex
	connected bool
	startTime time.Time
	started   bool
	stopCh    chan struct{}

	interrogCnt atomic.Int64
	controlCnt  atomic.Int64
	spontCnt    atomic.Int64
}

// NewClient creates a new IEC104 client.
func NewClient(cfg model.IEC104ClientConfig) *Client {
	if cfg.RemotePort == 0 {
		cfg.RemotePort = 2404
	}
	if cfg.CommonAddr == 0 {
		cfg.CommonAddr = 1
	}
	if cfg.ReconnectDelay == 0 {
		cfg.ReconnectDelay = 5
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 10
	}
	if cfg.InterrogPeriod == 0 {
		cfg.InterrogPeriod = 600
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 20
	}
	return &Client{cfg: cfg}
}

func (c *Client) Name() string { return "iec104_client" }

func (c *Client) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return nil
	}

	c.startTime = time.Now()
	c.connected = false
	c.stopCh = make(chan struct{})

	addr := fmt.Sprintf("%s:%d", c.cfg.RemoteAddr, c.cfg.RemotePort)
	opt := cs104.NewOption().
		SetAutoReconnect(true).
		SetReconnectInterval(time.Duration(c.cfg.ReconnectDelay) * time.Second).
		SetConfig(cs104.Config{
			ConnectTimeout0: time.Duration(c.cfg.ConnectTimeout) * time.Second,
		})
	if err := opt.AddRemoteServer(addr); err != nil {
		return fmt.Errorf("invalid remote address %s: %w", addr, err)
	}

	handler := &clientHandler{cl: c}
	cl := cs104.NewClient(handler, opt)
	cl.SetOnConnectHandler(c.onConnect)
	cl.SetConnectionLostHandler(c.onDisconnect)
	cl.SetServerActiveHandler(c.onServerActive)

	c.client = cl
	if err := cl.Start(); err != nil {
		return fmt.Errorf("client start failed: %w", err)
	}

	c.started = true
	go c.interrogationLoop()
	slog.Info("IEC104 客户端已启动", "remote", addr,
		"interrog_period", c.cfg.InterrogPeriod,
		"retry_delay", c.cfg.RetryDelay,
		"control_mode", c.cfg.ControlMode)
	return nil
}

func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return
	}

	slog.Info("正在停止 IEC104 客户端",
		"remote", fmt.Sprintf("%s:%d", c.cfg.RemoteAddr, c.cfg.RemotePort))

	if c.stopCh != nil {
		close(c.stopCh)
		c.stopCh = nil
	}

	if c.client != nil {
		_ = c.client.Close()
		c.client = nil
	}

	c.connected = false
	c.started = false
	slog.Info("IEC104 客户端已停止")
}

func (c *Client) ClientConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) ClientAddr() string {
	return fmt.Sprintf("%s:%d", c.cfg.RemoteAddr, c.cfg.RemotePort)
}

func (c *Client) Stats() (interrog, control, spont int64) {
	return c.interrogCnt.Load(), c.controlCnt.Load(), c.spontCnt.Load()
}

func (c *Client) Uptime() int64 {
	return int64(time.Since(c.startTime).Seconds())
}

func (c *Client) Publish(point *config.Point) {
	c.mu.RLock()
	cl := c.client
	connected := c.connected
	c.mu.RUnlock()

	if !connected || cl == nil {
		slog.Warn("IEC104 客户端未就绪，丢弃命令", "ioa", point.IOA)
		return
	}

	coa := asdu.CauseOfTransmission{Cause: asdu.Activation}
	ca := asdu.CommonAddr(c.cfg.CommonAddr)

	switch point.PointType {
	case config.TypeDO:
		cmd := asdu.SingleCommandInfo{
			Ioa:   asdu.InfoObjAddr(point.IOA),
			Value: point.BoolValue,
		}
		if err := asdu.SingleCmd(cl, asdu.C_SC_NA_1, coa, ca, cmd); err != nil {
			slog.Warn("发送遥控失败", "ioa", point.IOA, "error", err)
			return
		}
		c.controlCnt.Add(1)
		slog.Info("遥控已发送", "ioa", point.IOA, "value", point.BoolValue)

	case config.TypeAO:
		cmd := asdu.SetpointCommandFloatInfo{
			Ioa:   asdu.InfoObjAddr(point.IOA),
			Value: float32(point.Value),
		}
		if err := asdu.SetpointCmdFloat(cl, asdu.C_SE_NC_1, coa, ca, cmd); err != nil {
			slog.Warn("发送遥调失败", "ioa", point.IOA, "error", err)
			return
		}
		c.controlCnt.Add(1)
		slog.Info("遥调已发送", "ioa", point.IOA, "value", point.Value)
	}
}

func (c *Client) SetStore(store *library.Store) {
	c.store = store
}

func (c *Client) onConnect(cl *cs104.Client) {
	slog.Info("已连接到远端，发送 STARTDT 激活", "remote", cl.UnderlyingConn().RemoteAddr())
	cl.SendStartDt()
}

func (c *Client) onServerActive(cl *cs104.Client) {
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	slog.Info("STARTDT 确认，连接就绪",
		"remote", cl.UnderlyingConn().RemoteAddr())

	// 连接就绪后立即发送首次总召
	c.sendInterrogation()
}

func (c *Client) onDisconnect(cl *cs104.Client) {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	slog.Warn("与远端断开连接",
		"remote", fmt.Sprintf("%s:%d", c.cfg.RemoteAddr, c.cfg.RemotePort))
}

// interrogationLoop 周期性发送总召唤
func (c *Client) interrogationLoop() {
	period := time.Duration(c.cfg.InterrogPeriod) * time.Second
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if !c.ClientConnected() {
				continue
			}
			c.sendInterrogation()
		}
	}
}

// sendInterrogation 发送一次总召唤，失败时在 RetryDelay 后重试一次
func (c *Client) sendInterrogation() {
	c.mu.RLock()
	cl := c.client
	c.mu.RUnlock()
	if cl == nil {
		return
	}

	coa := asdu.CauseOfTransmission{Cause: asdu.Activation}
	ca := asdu.CommonAddr(c.cfg.CommonAddr)
	if err := cl.InterrogationCmd(coa, ca, 20); err != nil {
		slog.Warn("总召唤发送失败，等待重试", "error", err, "retry_delay", c.cfg.RetryDelay)
		retryDelay := time.Duration(c.cfg.RetryDelay) * time.Second
		select {
		case <-c.stopCh:
			return
		case <-time.After(retryDelay):
		}
		// 重试
		c.mu.RLock()
		cl = c.client
		c.mu.RUnlock()
		if cl == nil {
			return
		}
		if err := cl.InterrogationCmd(coa, ca, 20); err != nil {
			slog.Warn("总召唤重试失败", "error", err)
		} else {
			c.interrogCnt.Add(1)
			slog.Info("总召唤重试成功")
		}
	} else {
		c.interrogCnt.Add(1)
		slog.Info("周期总召唤已发送")
	}
}

type clientHandler struct {
	cl *Client
}

func (h *clientHandler) InterrogationHandler(_ asdu.Connect, a *asdu.ASDU) error {
	h.cl.interrogCnt.Add(1)
	return nil
}

func (h *clientHandler) CounterInterrogationHandler(_ asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) ReadHandler(_ asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) TestCommandHandler(_ asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) ClockSyncHandler(_ asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) ResetProcessHandler(_ asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) DelayAcquisitionHandler(_ asdu.Connect, a *asdu.ASDU) error {
	return nil
}

func (h *clientHandler) ASDUHandler(_ asdu.Connect, a *asdu.ASDU) error {
	store := h.cl.store
	if store == nil {
		return nil
	}

	switch a.Type {
	case asdu.M_ME_NC_1:
		for _, info := range a.GetMeasuredValueFloat() {
			if _, err := store.SetValue(uint32(info.Ioa), float64(info.Value)); err != nil {
				slog.Debug("更新遥测失败", "ioa", info.Ioa, "error", err)
			} else {
				h.cl.spontCnt.Add(1)
			}
		}

	case asdu.M_SP_NA_1:
		for _, info := range a.GetSinglePoint() {
			if _, err := store.SetBoolValue(uint32(info.Ioa), info.Value); err != nil {
				slog.Debug("更新遥信失败", "ioa", info.Ioa, "error", err)
			} else {
				h.cl.spontCnt.Add(1)
			}
		}

	case asdu.M_IT_NA_1:
		for _, info := range a.GetIntegratedTotals() {
			if _, err := store.SetIntValue(uint32(info.Ioa), info.Value.CounterReading); err != nil {
				slog.Debug("更新遥脉失败", "ioa", info.Ioa, "error", err)
			} else {
				h.cl.spontCnt.Add(1)
			}
		}
	}

	return nil
}
