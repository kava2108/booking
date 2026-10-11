package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// アカウント単位のログイン試行制限（REQ-002-AC-3 をアカウント単位に変更）。
// 「連続 max 回失敗で window の間ロック、成功でリセット」。

var g0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func newGuard(max int, window time.Duration, clock *time.Time) *MemoryGuard {
	return NewMemoryGuard(max, window, WithGuardClock(func() time.Time { return *clock }))
}

func TestMemoryGuard_BlocksAfterMaxFailures(t *testing.T) {
	now := g0
	g := newGuard(5, 15*time.Minute, &now)

	for i := 1; i <= 5; i++ {
		blocked, _ := g.Blocked("a@example.com")
		assert.False(t, blocked, "%d 回目の試行前はブロックされない", i)
		g.Fail("a@example.com")
	}
	blocked, retry := g.Blocked("a@example.com")
	assert.True(t, blocked, "5 回失敗したらブロック")
	assert.Equal(t, 15*time.Minute, retry)
}

// 境界値: 4 回の失敗ではブロックしない
func TestMemoryGuard_FourFailuresNotBlocked(t *testing.T) {
	now := g0
	g := newGuard(5, 15*time.Minute, &now)
	for i := 0; i < 4; i++ {
		g.Fail("a@example.com")
	}
	blocked, _ := g.Blocked("a@example.com")
	assert.False(t, blocked)
}

func TestMemoryGuard_RetryAfterShrinksAndUnlocks(t *testing.T) {
	now := g0
	g := newGuard(2, 10*time.Minute, &now)
	g.Fail("k")
	g.Fail("k")

	now = g0.Add(4 * time.Minute)
	blocked, retry := g.Blocked("k")
	assert.True(t, blocked)
	assert.Equal(t, 6*time.Minute, retry)

	now = g0.Add(10*time.Minute - time.Nanosecond)
	blocked, _ = g.Blocked("k")
	assert.True(t, blocked, "期限の直前はまだロック中")

	now = g0.Add(10 * time.Minute)
	blocked, _ = g.Blocked("k")
	assert.False(t, blocked, "期限ちょうどで解除")

	// 解除後はカウントが 0 から始まる
	g.Fail("k")
	blocked, _ = g.Blocked("k")
	assert.False(t, blocked)
}

// 連続失敗が対象: 成功（Reset）でカウントが戻る
func TestMemoryGuard_ResetClearsCounter(t *testing.T) {
	now := g0
	g := newGuard(5, 15*time.Minute, &now)
	for i := 0; i < 4; i++ {
		g.Fail("k")
	}
	g.Reset("k")
	for i := 0; i < 4; i++ {
		g.Fail("k")
	}
	blocked, _ := g.Blocked("k")
	assert.False(t, blocked, "リセット後は 4 回ではブロックされない")
}

// ロック中でも Reset（＝成功）はロックを解除しない設計ではなく、サービス側が Blocked を先に見る。
// ここでは Reset がロックも解除することだけを確認（パスワードリセット完了時に使う）
func TestMemoryGuard_ResetUnlocks(t *testing.T) {
	now := g0
	g := newGuard(1, time.Hour, &now)
	g.Fail("k")
	blocked, _ := g.Blocked("k")
	assert.True(t, blocked)

	g.Reset("k")
	blocked, _ = g.Blocked("k")
	assert.False(t, blocked)
}

// 失敗が窓の外に出たらカウントは新しく始まる（古い失敗の積み上げでロックしない）
func TestMemoryGuard_FailuresOutsideWindowDoNotAccumulate(t *testing.T) {
	now := g0
	g := newGuard(3, 15*time.Minute, &now)
	g.Fail("k")
	g.Fail("k")

	now = g0.Add(16 * time.Minute)
	g.Fail("k") // 窓が切れたので 1 回目として数える
	blocked, _ := g.Blocked("k")
	assert.False(t, blocked)
}

func TestMemoryGuard_KeysAreIndependent(t *testing.T) {
	now := g0
	g := newGuard(2, time.Minute, &now)
	g.Fail("alice")
	g.Fail("alice")

	a, _ := g.Blocked("alice")
	b, _ := g.Blocked("bob")
	assert.True(t, a)
	assert.False(t, b)
}

func TestMemoryGuard_Concurrent(t *testing.T) {
	now := g0
	g := newGuard(50, time.Hour, &now)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.Blocked("k")
			g.Fail("k")
		}()
	}
	wg.Wait()
	blocked, _ := g.Blocked("k")
	assert.True(t, blocked, "100 回の失敗（上限 50）はブロックされる")
}

// 攻撃者が大量の別キーを送ってもメモリが無制限に増えない
func TestMemoryGuard_BoundedMemory(t *testing.T) {
	now := g0
	g := NewMemoryGuard(5, time.Hour, WithGuardClock(func() time.Time { return now }), WithGuardMaxEntries(100))

	for i := 0; i < 1000; i++ {
		g.Fail(fmt.Sprintf("spray-%d@example.com", i))
	}
	assert.LessOrEqual(t, g.Len(), 100)
}

// 期限切れのエントリは掃除され、上限に達しても正規のキーを記録できる
func TestMemoryGuard_ExpiredEntriesAreSweptBeforeEviction(t *testing.T) {
	now := g0
	g := NewMemoryGuard(5, time.Minute, WithGuardClock(func() time.Time { return now }), WithGuardMaxEntries(10))
	for i := 0; i < 10; i++ {
		g.Fail(fmt.Sprintf("old-%d", i))
	}

	now = g0.Add(2 * time.Minute) // すべて期限切れ
	g.Fail("new")
	assert.Equal(t, 1, g.Len())
}

// 上限による追い出しがあっても、直近でロックされたキーは守られる
func TestMemoryGuard_EvictionKeepsLockedEntries(t *testing.T) {
	now := g0
	g := NewMemoryGuard(1, time.Hour, WithGuardClock(func() time.Time { return now }), WithGuardMaxEntries(5))
	g.Fail("victim") // ロックされる
	for i := 0; i < 50; i++ {
		g.Fail(fmt.Sprintf("noise-%d", i))
	}
	// noise もすべて max=1 でロックされるため全件ロック中。上限を超えて新規を入れられない場合でも、
	// 既存のロックは外れない
	blocked, _ := g.Blocked("victim")
	assert.True(t, blocked)
}

func TestMemoryGuard_ResetAccountClearsEveryIPKey(t *testing.T) {
	g := NewMemoryGuard(1, time.Minute)
	g.Fail("user@example.com|203.0.113.9")
	g.Fail("user@example.com|198.51.100.7")
	g.Fail("user@example.com")
	g.Fail("other@example.com|203.0.113.9")

	g.ResetAccount("user@example.com")

	for _, k := range []string{"user@example.com|203.0.113.9", "user@example.com|198.51.100.7", "user@example.com"} {
		blocked, _ := g.Blocked(k)
		assert.False(t, blocked, k)
	}
	blocked, _ := g.Blocked("other@example.com|203.0.113.9")
	assert.True(t, blocked, "別アカウントは解除しない")
}
