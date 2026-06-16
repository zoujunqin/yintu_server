// Package ratelimit 提供进程内的轻量级限流：
//   - Cooldown：基于「上次触发时间」的冷却窗口，1 次 / window
//   - Window：基于「窗口内累计次数」的滑动计数
//
// 当前实现使用 sync.Map + 内存状态，仅适用于单实例部署。
// 多实例请替换为 Redis 集中式实现（接口保持不变）。
package ratelimit

import (
	"sync"
	"time"
)

// Limiter 限流器接口。
type Limiter interface {
	// Cooldown 记录一次触发；若距上次触发时间 < window 返回 false。
	Cooldown(key string, window time.Duration) bool
	// Allow 记录一次触发；返回是否超过 limit/次·window。
	Allow(key string, limit int, window time.Duration) bool
}

// MemoryLimiter 基于内存的 Limiter 实现。
type MemoryLimiter struct {
	mu       sync.Mutex
	last     map[string]time.Time
	counters map[string]*windowCounter
	now      func() time.Time
}

// NewMemoryLimiter 构造 MemoryLimiter。
func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		last:     make(map[string]time.Time),
		counters: make(map[string]*windowCounter),
		now:      time.Now,
	}
}

func (m *MemoryLimiter) Cooldown(key string, window time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	last, ok := m.last[key]
	if ok && now.Sub(last) < window {
		return false
	}
	m.last[key] = now
	return true
}

func (m *MemoryLimiter) Allow(key string, limit int, window time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	c, ok := m.counters[key]
	if !ok || now.Sub(c.start) >= window {
		m.counters[key] = &windowCounter{start: now, count: 1}
		return true
	}
	if c.count >= limit {
		return false
	}
	c.count++
	return true
}

type windowCounter struct {
	start time.Time
	count int
}
