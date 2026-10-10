package service

import (
	"sync"
	"time"
)

// AttemptGuard はキー（正規化したメールアドレスなど）単位の試行制限。
// 「連続して max 回失敗したら window の間ロックし、成功（Reset）でカウントを戻す」。
// 単一インスタンス前提のメモリ実装(MemoryGuard)を使う。複数台に増やすときは共有ストア実装に差し替える。
type AttemptGuard interface {
	// Blocked はキーが現在ロック中かと、解除までの残り時間を返す。
	Blocked(key string) (blocked bool, retryAfter time.Duration)
	// Fail は失敗（またはカウント対象のイベント）を 1 回記録する。max 回に達するとロックする。
	Fail(key string)
	// Reset はカウントとロックを解除する。
	Reset(key string)
}

const defaultGuardMaxEntries = 100_000

type guardEntry struct {
	count       int
	windowStart time.Time
	lockedUntil time.Time // ゼロ値 = ロックなし
}

// MemoryGuard は AttemptGuard のメモリ実装。
type MemoryGuard struct {
	mu         sync.Mutex
	max        int
	window     time.Duration
	now        func() time.Time
	maxEntries int
	entries    map[string]*guardEntry
}

// GuardOption は MemoryGuard の設定。
type GuardOption func(*MemoryGuard)

func WithGuardClock(now func() time.Time) GuardOption { return func(g *MemoryGuard) { g.now = now } }

// WithGuardMaxEntries は保持するキー数の上限（大量の別キーを送られてもメモリが増えない）。
func WithGuardMaxEntries(n int) GuardOption { return func(g *MemoryGuard) { g.maxEntries = n } }

func NewMemoryGuard(max int, window time.Duration, opts ...GuardOption) *MemoryGuard {
	g := &MemoryGuard{max: max, window: window, now: time.Now, maxEntries: defaultGuardMaxEntries, entries: map[string]*guardEntry{}}
	for _, o := range opts {
		o(g)
	}
	return g
}

// Len は保持しているキー数（テスト用）。
func (g *MemoryGuard) Len() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.entries)
}

func (g *MemoryGuard) Blocked(key string) (bool, time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.entries[key]
	if !ok || e.lockedUntil.IsZero() {
		return false, 0
	}
	now := g.now()
	if now.Before(e.lockedUntil) {
		return true, e.lockedUntil.Sub(now)
	}
	delete(g.entries, key) // ロック期限切れ
	return false, 0
}

func (g *MemoryGuard) Fail(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()

	e, ok := g.entries[key]
	if ok && g.expired(e, now) {
		delete(g.entries, key)
		ok = false
	}
	if ok && !e.lockedUntil.IsZero() {
		return // ロック中はこれ以上積み上げない（ロック期間を延長しない）
	}
	if !ok {
		if !g.makeRoom(now) {
			return // 空きが作れない（全件ロック中）。既存のロックを守るため新規は記録しない
		}
		e = &guardEntry{windowStart: now}
		g.entries[key] = e
	}
	e.count++
	if e.count >= g.max {
		e.lockedUntil = now.Add(g.window)
	}
}

func (g *MemoryGuard) Reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}

// expired はカウント窓、またはロックが終わっているか。
func (g *MemoryGuard) expired(e *guardEntry, now time.Time) bool {
	if !e.lockedUntil.IsZero() {
		return !now.Before(e.lockedUntil)
	}
	return !now.Before(e.windowStart.Add(g.window))
}

// makeRoom は新規キーを入れる余地を作る。期限切れを掃除し、それでも満杯ならロックされていないキーを 1 つ追い出す。
func (g *MemoryGuard) makeRoom(now time.Time) bool {
	if len(g.entries) < g.maxEntries {
		return true
	}
	for k, e := range g.entries {
		if g.expired(e, now) {
			delete(g.entries, k)
		}
	}
	if len(g.entries) < g.maxEntries {
		return true
	}
	for k, e := range g.entries {
		if e.lockedUntil.IsZero() {
			delete(g.entries, k)
			return true
		}
	}
	return false
}
