package postgres

import (
	"context"
	"errors"
	"testing"

	"telesrv/internal/domain"
)

// TestSecretChatStorePostgres 验证密聊握手状态机 PG 实现的行为契约（与 memory 实现
// 同构）：create/get、幂等去重、accept CAS、double-accept、discard 幂等、
// accept-after-discard、部分唯一索引（discard 后同 random_id 可重建）。
// 门控于 TELESRV_TEST_POSTGRES_DSN。
func TestSecretChatStorePostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := NewSecretChatStore(pool)

	const (
		adminUser = int64(770001)
		partUser  = int64(770002)
		adminKey  = int64(0xA1)
		partKey   = int64(0xB2)
		base      = 7700001
	)
	// 隔离：清理本测试用到的行（测试库持久累积）。
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM secret_chats WHERE admin_user_id = $1`, adminUser)
	}
	cleanup()
	t.Cleanup(cleanup)

	mk := func(id int, randomID int32) domain.SecretChat {
		return domain.SecretChat{
			ID:                    id,
			AdminAccessHash:       111,
			ParticipantAccessHash: 222,
			AdminUserID:           adminUser,
			AdminAuthKeyID:        adminKey,
			ParticipantUserID:     partUser,
			State:                 domain.SecretChatStateRequested,
			GA:                    []byte{0x0a, 0x0b, 0x0c},
			RandomID:              randomID,
			Date:                  1000,
		}
	}

	// create + get round-trip。
	chat := mk(base, 555)
	if err := store.CreateSecretChat(ctx, chat); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, found, err := store.GetSecretChat(ctx, base)
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if got.AdminUserID != adminUser || got.RandomID != 555 || string(got.GA) != string(chat.GA) {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	// 重复 chat_id → ID conflict。
	if err := store.CreateSecretChat(ctx, mk(base, 556)); !errors.Is(err, domain.ErrSecretChatIDConflict) {
		t.Fatalf("duplicate chat_id err = %v, want ErrSecretChatIDConflict", err)
	}

	// 幂等查询（非终态）。
	idem, found, err := store.GetByAdminRandom(ctx, adminKey, 555)
	if err != nil || !found || idem.ID != base {
		t.Fatalf("GetByAdminRandom: found=%v id=%d err=%v", found, idem.ID, err)
	}

	// accept CAS。
	accepted, err := store.AcceptSecretChat(ctx, base, partKey, []byte{0x0d, 0x0e}, 0x99)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if accepted.State != domain.SecretChatStateNormal || accepted.ParticipantAuthKeyID != partKey ||
		accepted.KeyFingerprint != 0x99 || string(accepted.GB) != string([]byte{0x0d, 0x0e}) {
		t.Fatalf("accepted = %+v", accepted)
	}

	// double accept → already accepted。
	if _, err := store.AcceptSecretChat(ctx, base, int64(0xC3), []byte{0x01}, 1); !errors.Is(err, domain.ErrSecretChatAlreadyAccepted) {
		t.Fatalf("double accept err = %v, want ErrSecretChatAlreadyAccepted", err)
	}

	// discard。
	_, already, err := store.DiscardSecretChat(ctx, base, true)
	if err != nil || already {
		t.Fatalf("discard: already=%v err=%v", already, err)
	}
	// 幂等 discard。
	cur, already, err := store.DiscardSecretChat(ctx, base, false)
	if err != nil || !already || cur.State != domain.SecretChatStateDiscarded {
		t.Fatalf("idempotent discard: already=%v state=%v err=%v", already, cur.State, err)
	}

	// accept after discard → already declined。
	if _, err := store.AcceptSecretChat(ctx, base, partKey, []byte{0x01}, 1); !errors.Is(err, domain.ErrSecretChatAlreadyDeclined) {
		t.Fatalf("accept after discard err = %v, want ErrSecretChatAlreadyDeclined", err)
	}

	// 部分唯一索引：discarded 旧 chat 不阻塞同 (admin_auth_key_id, random_id) 重建。
	if err := store.CreateSecretChat(ctx, mk(base+1, 555)); err != nil {
		t.Fatalf("recreate after discard with same random_id: %v", err)
	}

	// MaxSecretChatID 反映最大 chat_id（≥ base+1）。
	maxID, err := store.MaxSecretChatID(ctx)
	if err != nil || maxID < base+1 {
		t.Fatalf("max chat id = %d err = %v, want >= %d", maxID, err, base+1)
	}

	// 不存在 chat：accept/discard 返回 not found。
	if _, err := store.AcceptSecretChat(ctx, base+999, partKey, []byte{0x01}, 1); !errors.Is(err, domain.ErrSecretChatNotFound) {
		t.Fatalf("accept missing err = %v, want ErrSecretChatNotFound", err)
	}
	if _, _, err := store.DiscardSecretChat(ctx, base+999, false); !errors.Is(err, domain.ErrSecretChatNotFound) {
		t.Fatalf("discard missing err = %v, want ErrSecretChatNotFound", err)
	}
}

// TestSecretChatListActiveByAuthKeyPostgres 验证 ListActiveSecretChatsByAuthKey（登出/撤销
// 级联 discard 的查询基础，P1 修复）：匹配 admin 或 participant auth_key、排除 discarded、
// 按 chat_id 升序；与 memory 实现同构。
func TestSecretChatListActiveByAuthKeyPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := NewSecretChatStore(pool)

	const (
		adminUser = int64(770101)
		partUser  = int64(770102)
		keyA      = int64(0xAA01) // admin 设备 key
		keyB      = int64(0xBB02) // participant 设备 key
		keyOther  = int64(0xCC03)
		base      = 7701001
	)
	cleanup := func() { _, _ = pool.Exec(ctx, `DELETE FROM secret_chats WHERE admin_user_id IN ($1, $2)`, adminUser, partUser) }
	cleanup()
	t.Cleanup(cleanup)

	mk := func(id int, randomID int32, adminUID, adminKey, partUID, partKeyV int64, state domain.SecretChatState) domain.SecretChat {
		return domain.SecretChat{
			ID: id, AdminAccessHash: 1, ParticipantAccessHash: 2,
			AdminUserID: adminUID, AdminAuthKeyID: adminKey,
			ParticipantUserID: partUID, ParticipantAuthKeyID: partKeyV,
			State: state, GA: []byte{0x01}, RandomID: randomID, Date: 1,
		}
	}
	idsOf := func(chats []domain.SecretChat) []int {
		out := make([]int, len(chats))
		for i, c := range chats {
			out[i] = c.ID
		}
		return out
	}
	eq := func(got, want []int) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// chat1 admin=keyA participant=keyB normal；chat2 admin=keyA 未绑定 requested；
	// chat3 admin=keyOther participant=keyB normal；chat4 admin=keyA participant=keyB 已 discard。
	for _, c := range []domain.SecretChat{
		mk(base+1, 1, adminUser, keyA, partUser, keyB, domain.SecretChatStateNormal),
		mk(base+2, 2, adminUser, keyA, partUser, 0, domain.SecretChatStateRequested),
		mk(base+3, 3, partUser, keyOther, adminUser, keyB, domain.SecretChatStateNormal),
		mk(base+4, 4, adminUser, keyA, partUser, keyB, domain.SecretChatStateNormal),
	} {
		if err := store.CreateSecretChat(ctx, c); err != nil {
			t.Fatalf("create %d: %v", c.ID, err)
		}
	}
	if _, _, err := store.DiscardSecretChat(ctx, base+4, false); err != nil {
		t.Fatalf("discard chat4: %v", err)
	}

	// keyA（作为 admin）→ chat1, chat2（chat4 已 discard 排除）。
	got, err := store.ListActiveSecretChatsByAuthKey(ctx, keyA)
	if err != nil {
		t.Fatalf("list keyA: %v", err)
	}
	if ids := idsOf(got); !eq(ids, []int{base + 1, base + 2}) {
		t.Fatalf("keyA active = %v, want [%d %d]", ids, base+1, base+2)
	}
	// keyB（作为 participant）→ chat1, chat3（chat4 已 discard 排除）。
	got, err = store.ListActiveSecretChatsByAuthKey(ctx, keyB)
	if err != nil {
		t.Fatalf("list keyB: %v", err)
	}
	if ids := idsOf(got); !eq(ids, []int{base + 1, base + 3}) {
		t.Fatalf("keyB active = %v, want [%d %d]", ids, base+1, base+3)
	}
	// 未知 key → 空；authKeyID 0 → nil。
	if got, err := store.ListActiveSecretChatsByAuthKey(ctx, 0x9999); err != nil || len(got) != 0 {
		t.Fatalf("unknown key = %d (err=%v), want 0", len(got), err)
	}
	if got, err := store.ListActiveSecretChatsByAuthKey(ctx, 0); err != nil || got != nil {
		t.Fatalf("zero key = %v (err=%v), want nil", got, err)
	}
}
