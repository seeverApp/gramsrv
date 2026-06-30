package account

import (
	"context"
	"errors"
	"testing"

	"telesrv/internal/domain"
	"telesrv/internal/store/memory"
)

func newLoginEmailService(t *testing.T) (*Service, *memory.UserStore) {
	t.Helper()
	users := memory.NewUserStore()
	svc := NewService(memory.NewPasswordStore(), WithUsers(users))
	return svc, users
}

func createUser(t *testing.T, users *memory.UserStore, phone string) domain.User {
	t.Helper()
	u, err := users.Create(context.Background(), domain.User{Phone: phone, FirstName: "Test"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

// TestSetLoginEmailPersistsAndMasks 设置登录邮箱后，GetPassword 下发掩码 pattern，原始
// 地址只在 LoginEmail 读路径可见。
func TestSetLoginEmailPersistsAndMasks(t *testing.T) {
	ctx := context.Background()
	svc, users := newLoginEmailService(t)
	u := createUser(t, users, "15550010001")

	if err := svc.SetLoginEmail(ctx, u.ID, "alice@example.com"); err != nil {
		t.Fatalf("SetLoginEmail: %v", err)
	}

	settings, err := svc.GetPassword(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetPassword: %v", err)
	}
	if got, want := settings.LoginEmailPattern, "a***e@example.com"; got != want {
		t.Fatalf("LoginEmailPattern = %q, want %q", got, want)
	}
	if settings.LoginEmail != "alice@example.com" {
		t.Fatalf("LoginEmail = %q, want raw address", settings.LoginEmail)
	}

	email, found, err := svc.LoginEmail(ctx, u.ID)
	if err != nil || !found || email != "alice@example.com" {
		t.Fatalf("LoginEmail = %q found=%v err=%v", email, found, err)
	}
}

// TestLoginEmailByPhoneAndClear 验证按手机号读取/清除登录邮箱（sendCode 检测 + reset 用）。
func TestLoginEmailByPhoneAndClear(t *testing.T) {
	ctx := context.Background()
	svc, users := newLoginEmailService(t)
	createUser(t, users, "15550010002")

	if err := svc.SetLoginEmailByPhone(ctx, "+1 555 001 0002", "bob@mail.com"); err != nil {
		t.Fatalf("SetLoginEmailByPhone: %v", err)
	}
	email, found, err := svc.LoginEmailByPhone(ctx, "15550010002")
	if err != nil || !found || email != "bob@mail.com" {
		t.Fatalf("LoginEmailByPhone = %q found=%v err=%v", email, found, err)
	}

	if err := svc.ClearLoginEmailByPhone(ctx, "15550010002"); err != nil {
		t.Fatalf("ClearLoginEmailByPhone: %v", err)
	}
	if _, found, _ := svc.LoginEmailByPhone(ctx, "15550010002"); found {
		t.Fatal("login email still present after clear")
	}
}

// TestSetLoginEmailRejectsInvalid 空/无 @ 的邮箱被拒。
func TestSetLoginEmailRejectsInvalid(t *testing.T) {
	ctx := context.Background()
	svc, users := newLoginEmailService(t)
	u := createUser(t, users, "15550010003")

	for _, bad := range []string{"", "   ", "not-an-email"} {
		if err := svc.SetLoginEmail(ctx, u.ID, bad); !errors.Is(err, domain.ErrEmailInvalid) {
			t.Fatalf("SetLoginEmail(%q) err = %v, want ErrEmailInvalid", bad, err)
		}
	}
}

// TestRecoveryEmailDoesNotLeakIntoLoginEmailPattern 是核心解耦回归：设置 2FA 恢复邮箱
// 不得把恢复邮箱掩码写进 login_email_pattern（历史 bug）。
func TestRecoveryEmailDoesNotLeakIntoLoginEmailPattern(t *testing.T) {
	ctx := context.Background()
	svc, users := newLoginEmailService(t)
	u := createUser(t, users, "15550010004")

	// 设置 2FA 恢复邮箱（email-only 路径即可触发历史 bug 的写入点）。
	if err := svc.UpdatePasswordSettings(ctx, u.ID, domain.PasswordCheck{Empty: true}, domain.PasswordInputSettings{
		Email:    "recovery@secret.com",
		HasEmail: true,
	}); err != nil {
		t.Fatalf("UpdatePasswordSettings: %v", err)
	}

	settings, err := svc.GetPassword(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetPassword: %v", err)
	}
	if settings.LoginEmailPattern != "" {
		t.Fatalf("LoginEmailPattern = %q, want empty (recovery email must not leak into login email)", settings.LoginEmailPattern)
	}
	if !settings.HasRecovery {
		t.Fatal("HasRecovery = false, want true after setting recovery email")
	}
}
