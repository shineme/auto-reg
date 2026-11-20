package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPoolPlatformIsolation 验证同一邮箱在 MiniMax / Cursor 上互不影响。
func TestPoolPlatformIsolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pool.json")
	if err := SaveHotmailPool(path, &HotmailPool{Accounts: []HotmailAccount{
		{Email: "a@x.com", Password: "p", ClientID: "c", RefreshToken: "r", Status: "available"},
		{Email: "b@x.com", Password: "p", ClientID: "c", RefreshToken: "r", Status: "available"},
	}}); err != nil {
		t.Fatal(err)
	}
	store := NewHotmailPoolStore(path)

	// 先给 MiniMax 用掉第一个号。
	acc, idx, err := store.ClaimFor(PlatformMiniMax)
	if err != nil {
		t.Fatal(err)
	}
	store.FinalizeFor(PlatformMiniMax, idx, acc.Email, "used", "", acc.RefreshToken)

	// Cursor 应该还能领到同一个邮箱（跨平台复用）。
	cacc, cidx, err := store.ClaimFor(PlatformCursor)
	if err != nil {
		t.Fatal(err)
	}
	if cacc.Email != acc.Email {
		t.Fatalf("Cursor 领到的邮箱 %s 与 MiniMax 用过的 %s 不同，跨平台复用失效", cacc.Email, acc.Email)
	}
	store.FinalizeFor(PlatformCursor, cidx, cacc.Email, "used", "", cacc.RefreshToken)

	// MiniMax 侧 a 已经用完，只能领到还没用过的 b。
	macc, _, err := store.ClaimFor(PlatformMiniMax)
	if err != nil {
		t.Fatal(err)
	}
	if macc.Email == acc.Email {
		t.Fatalf("MiniMax 又领到了已用过的 %s", macc.Email)
	}
	store.FinalizeFor(PlatformMiniMax, 1, macc.Email, "used", "", macc.RefreshToken)
	// b 号 MiniMax 用过了，但 Cursor 还没用过，仍可跨平台复用。
	cacc2, cidx2, err := store.ClaimFor(PlatformCursor)
	if err != nil {
		t.Fatal(err)
	}
	if cacc2.Email != macc.Email {
		t.Fatalf("Cursor 复用失败：领到 %s，期望 %s", cacc2.Email, macc.Email)
	}
	store.FinalizeFor(PlatformCursor, cidx2, cacc2.Email, "used", "", cacc2.RefreshToken)
	// 两个平台都各自用完了自己的号，才算耗尽。
	if _, _, err := store.ClaimFor(PlatformMiniMax); err == nil {
		t.Fatal("MiniMax 号池没有耗尽")
	}
	if _, _, err := store.ClaimFor(PlatformCursor); err == nil {
		t.Fatal("Cursor 号池没有耗尽")
	}

	pool, err := LoadHotmailPool(path)
	if err != nil {
		t.Fatal(err)
	}
	stats := pool.StatsByPlatform()
	// 两个邮箱在两个平台上各用了一次。
	if stats[PlatformMiniMax].Used != 2 || stats[PlatformCursor].Used != 2 {
		t.Fatalf("按平台统计不对: %+v", stats)
	}
	if stats[PlatformMiniMax].Available != 0 || stats[PlatformCursor].Available != 0 {
		t.Fatalf("两个平台都应该没有可用号了: %+v", stats)
	}
	_ = os.Remove(path)
}

// TestPoolMigrateKeepsCursorAvailable 验证老号池迁移后 Cursor 仍可用。
func TestPoolMigrateKeepsCursorAvailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pool.json")
	raw := `{"version":1,"accounts":[{"email":"a@x.com","status":"used","updatedAt":"2026-01-01T00:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	pool, err := LoadHotmailPool(path)
	if err != nil {
		t.Fatal(err)
	}
	acc := pool.Accounts[0]
	if got := acc.PlatformStatus(PlatformMiniMax); got != "used" {
		t.Fatalf("MiniMax 状态应为 used，实际 %s", got)
	}
	if got := acc.PlatformStatus(PlatformCursor); got != "available" {
		t.Fatalf("Cursor 状态应为 available，实际 %s", got)
	}
}
