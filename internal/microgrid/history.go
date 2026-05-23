package microgrid

import (
	"sync"
)

// HistoryBuffer 环形历史缓冲区
type HistoryBuffer struct {
	mu      sync.Mutex
	maxSize int
	frames  []SimSnapshot
	cursor  int
	wrapped bool
}

// NewHistoryBuffer 创建环形历史缓冲区
func NewHistoryBuffer(maxSize int) *HistoryBuffer {
	return &HistoryBuffer{
		maxSize: maxSize,
		frames:  make([]SimSnapshot, maxSize),
	}
}

// Push 添加新帧
func (hb *HistoryBuffer) Push(snap SimSnapshot) {
	hb.mu.Lock()
	defer hb.mu.Unlock()
	hb.frames[hb.cursor] = snap
	hb.cursor++
	if hb.cursor >= hb.maxSize {
		hb.cursor = 0
		hb.wrapped = true
	}
}

// Snapshots 返回所有帧（按时间顺序）
func (hb *HistoryBuffer) Snapshots() []SimSnapshot {
	hb.mu.Lock()
	defer hb.mu.Unlock()
	if !hb.wrapped {
		result := make([]SimSnapshot, hb.cursor)
		copy(result, hb.frames[:hb.cursor])
		return result
	}
	result := make([]SimSnapshot, hb.maxSize)
	copy(result, hb.frames[hb.cursor:])
	copy(result[hb.maxSize-hb.cursor:], hb.frames[:hb.cursor])
	return result
}

// Clear 清空缓冲区
func (hb *HistoryBuffer) Clear() {
	hb.mu.Lock()
	defer hb.mu.Unlock()
	hb.cursor = 0
	hb.wrapped = false
}
