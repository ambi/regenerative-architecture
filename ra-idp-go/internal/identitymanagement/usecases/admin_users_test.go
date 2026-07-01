package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	idmusecases "ra-idp-go/internal/identitymanagement/usecases"
	"ra-idp-go/internal/shared/adapters/crypto"
	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"
)

func TestCreateUpdateAndDisableUser(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	historyRepo := memory.NewPasswordHistoryRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	var events []spec.DomainEvent
	deps := idmusecases.AdminUserDeps{
		UserRepo: userRepo, PasswordHasher: hasher, PasswordHistoryRepo: historyRepo,
		Emit: func(event spec.DomainEvent) { events = append(events, event) },
	}
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	email := "bob@example.com"
	user, err := idmusecases.CreateUser(ctx, deps, idmusecases.CreateUserInput{
		ActorSub: "admin", PreferredUsername: "bob", Password: "initial-password-9182",
		Email: &email, Roles: []string{"support", "support"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(user.Roles) != 1 || user.Roles[0] != "support" {
		t.Fatalf("roles=%v", user.Roles)
	}
	if events[0].EventType() != "UserCreated" {
		t.Fatalf("event=%s", events[0].EventType())
	}
	updatedName := "Bob"
	roles := []string{"admin", "support"}
	user, err = idmusecases.UpdateUser(ctx, deps, idmusecases.UpdateUserInput{
		ActorSub: "admin", Sub: user.Sub, Name: &updatedName, Roles: &roles, Now: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Name == nil || *user.Name != "Bob" || len(user.Roles) != 2 {
		t.Fatalf("updated user=%+v", user)
	}
	user, err = idmusecases.SetUserDisabled(
		ctx, deps, "admin", user.Sub, true, now.Add(2*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if user.Lifecycle.Status != spec.UserStatusDisabled {
		t.Fatal("status was not set to disabled")
	}
	if got := events[len(events)-1].EventType(); got != "UserDisabled" {
		t.Fatalf("last event=%s", got)
	}
	user, err = idmusecases.SetUserDisabled(
		ctx, deps, "admin", user.Sub, false, now.Add(3*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if user.Lifecycle.Status != spec.UserStatusActive {
		t.Fatal("status was not cleared to active")
	}
	if got := events[len(events)-1].EventType(); got != "UserEnabled" {
		t.Fatalf("last event=%s", got)
	}
}

func TestCreateUserRejectsDuplicateUsername(t *testing.T) {
	repo := memory.NewUserRepository()
	now := time.Now().UTC()
	repo.Seed(&spec.User{
		Sub: "existing", PreferredUsername: "bob", PasswordHash: "hash",
		CreatedAt: now, UpdatedAt: now,
	})
	_, err := idmusecases.CreateUser(context.Background(), idmusecases.AdminUserDeps{
		UserRepo: repo, PasswordHasher: crypto.NewArgon2idPasswordHasher(),
		PasswordHistoryRepo: memory.NewPasswordHistoryRepository(),
	}, idmusecases.CreateUserInput{
		PreferredUsername: "bob", Password: "initial-password-9182",
	})
	if !errors.Is(err, idmusecases.ErrUsernameConflict) {
		t.Fatalf("error=%v, want ErrUsernameConflict", err)
	}
}

func TestDeleteUserAnonymizesAndCascades(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	historyRepo := memory.NewPasswordHistoryRepository()
	consentRepo := memory.NewConsentRepository()
	refreshStore := memory.NewRefreshTokenStore()
	deviceStore := memory.NewDeviceCodeStore()
	sessionStore := memory.NewSessionStore()
	mfaRepo := memory.NewMfaFactorRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	var events []spec.DomainEvent
	deps := idmusecases.AdminUserDeps{
		UserRepo: userRepo, ConsentRepo: consentRepo, RefreshStore: refreshStore,
		DeviceCodeStore: deviceStore, SessionStore: sessionStore, MfaFactorRepo: mfaRepo,
		PasswordHasher: hasher, PasswordHistoryRepo: historyRepo,
		Emit: func(event spec.DomainEvent) { events = append(events, event) },
	}
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	user, err := idmusecases.CreateUser(ctx, deps, idmusecases.CreateUserInput{
		ActorSub: "admin", PreferredUsername: "alice", Password: "initial-password-9182",
		Roles: []string{"support"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Seed cascade artifacts.
	_ = consentRepo.Save(ctx, &spec.Consent{
		TenantID: spec.DefaultTenantID, Sub: user.Sub, ClientID: "client-a",
		Scopes: []string{"openid"}, State: spec.ConsentGranted,
		GrantedAt: now, ExpiresAt: now.AddDate(1, 0, 0),
	})
	_ = refreshStore.Save(ctx, &spec.RefreshTokenRecord{
		ID: "rt-1", TenantID: spec.DefaultTenantID, Hash: "hash-1",
		FamilyID: "fam-1", ClientID: "client-a", Sub: user.Sub,
		Scopes: []string{"openid"}, IssuedAt: now,
		ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.AddDate(0, 0, 30),
	})
	_ = sessionStore.Save(ctx, &spec.LoginSession{
		ID: "sess-1", TenantID: spec.DefaultTenantID, Sub: user.Sub,
		AuthTime: now.Unix(), AMR: []string{"pwd"}, ACR: "urn:mace:incommon:iap:silver",
		ExpiresAt: now.Add(time.Hour),
	})
	totpSecret := "JBSWY3DPEHPK3PXP"
	_ = mfaRepo.Save(ctx, &spec.MfaFactor{
		Sub: user.Sub, Type: spec.MfaFactorTOTP, Secret: &totpSecret, CreatedAt: now,
	})

	if err := idmusecases.DeleteUser(ctx, deps, idmusecases.DeleteUserInput{
		ActorSub: "admin", Sub: user.Sub, Reason: "leaving company", Now: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if last, ok := events[len(events)-1].(*spec.UserDeleted); !ok || last.TargetSub != user.Sub || last.Reason != "leaving company" {
		t.Fatalf("expected UserDeleted event with target=%s reason set, got %+v", user.Sub, events[len(events)-1])
	}
	tombstone, err := userRepo.FindBySubIncludingDeleted(ctx, user.Sub)
	if err != nil {
		t.Fatal(err)
	}
	if tombstone == nil || !tombstone.IsDeleted() {
		t.Fatalf("expected tombstone with status=deleted, got %+v", tombstone)
	}
	if tombstone.PreferredUsername != "deleted:"+user.Sub {
		t.Fatalf("preferred_username not anonymized: %s", tombstone.PreferredUsername)
	}
	if tombstone.Email != nil || tombstone.Name != nil || len(tombstone.Roles) != 0 || tombstone.MfaEnrolled {
		t.Fatalf("PII not anonymized: %+v", tombstone)
	}
	if seen, _ := userRepo.FindBySub(ctx, user.Sub); seen != nil {
		t.Fatalf("FindBySub returned deleted user")
	}
	// Cascade verification.
	if remaining, _ := consentRepo.FindAll(ctx, spec.DefaultTenantID); len(remaining) != 0 {
		t.Fatalf("consent cascade leaked: %+v", remaining)
	}
	if rec, _ := refreshStore.FindByHash(ctx, "hash-1"); rec != nil {
		t.Fatalf("refresh cascade leaked: %+v", rec)
	}
	if sess, _ := sessionStore.Find(ctx, "sess-1"); sess != nil {
		t.Fatalf("session cascade leaked: %+v", sess)
	}
	if factors, _ := mfaRepo.ListBySub(ctx, user.Sub); len(factors) != 0 {
		t.Fatalf("mfa cascade leaked: %+v", factors)
	}
	// Re-delete is no-op (no new UserDeleted event).
	prev := len(events)
	if err := idmusecases.DeleteUser(ctx, deps, idmusecases.DeleteUserInput{
		ActorSub: "admin", Sub: user.Sub, Now: now.Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("idempotent delete failed: %v", err)
	}
	if len(events) != prev {
		t.Fatalf("idempotent delete emitted extra events")
	}
}

func TestDeleteUserRejectsSelfDelete(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	now := time.Now().UTC()
	userRepo.Seed(&spec.User{
		Sub: "admin-1", PreferredUsername: "admin", PasswordHash: "hash",
		Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
	})
	err := idmusecases.DeleteUser(ctx, idmusecases.AdminUserDeps{UserRepo: userRepo},
		idmusecases.DeleteUserInput{ActorSub: "admin-1", Sub: "admin-1", Now: now})
	if !errors.Is(err, idmusecases.ErrSelfDeleteForbidden) {
		t.Fatalf("error=%v, want ErrSelfDeleteForbidden", err)
	}
}

func TestSetUserDisabledRejectsSelfDisable(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	now := time.Now().UTC()
	userRepo.Seed(&spec.User{
		Sub: "admin-1", PreferredUsername: "admin", PasswordHash: "hash",
		Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
	})
	deps := idmusecases.AdminUserDeps{UserRepo: userRepo}

	// admin が自身を無効化しようとすると自爆防止に弾かれる。
	_, err := idmusecases.SetUserDisabled(ctx, deps, "admin-1", "admin-1", true, now)
	if !errors.Is(err, idmusecases.ErrSelfDisableForbidden) {
		t.Fatalf("disable self error=%v, want ErrSelfDisableForbidden", err)
	}

	// enable 方向は自身に対しても許可する (アクセス回復のみで誤操作リスクが低い)。
	if _, err := idmusecases.SetUserDisabled(ctx, deps, "admin-1", "admin-1", false, now); err != nil {
		t.Fatalf("enable self error=%v, want nil", err)
	}
}

func TestSetUserDisabledAllowsDisablingOtherAdmin(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	now := time.Now().UTC()
	userRepo.Seed(&spec.User{
		Sub: "admin-2", PreferredUsername: "other-admin", PasswordHash: "hash",
		Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
	})
	deps := idmusecases.AdminUserDeps{UserRepo: userRepo}

	user, err := idmusecases.SetUserDisabled(ctx, deps, "admin-1", "admin-2", true, now)
	if err != nil {
		t.Fatalf("disable other admin error=%v, want nil", err)
	}
	if user.Lifecycle.Status != spec.UserStatusDisabled {
		t.Fatalf("status=%v, want disabled", user.Lifecycle.Status)
	}
}

// softDeleteTestDeps は soft-delete 系テスト用に cascade 対象リポジトリを揃えた
// deps と consent リポジトリ (cascade 温存の確認用) を返す。
func softDeleteTestDeps(events *[]spec.DomainEvent) (idmusecases.AdminUserDeps, *memory.ConsentRepository, *memory.UserRepository) {
	userRepo := memory.NewUserRepository()
	consentRepo := memory.NewConsentRepository()
	deps := idmusecases.AdminUserDeps{
		UserRepo: userRepo, ConsentRepo: consentRepo,
		RefreshStore: memory.NewRefreshTokenStore(), SessionStore: memory.NewSessionStore(),
		MfaFactorRepo: memory.NewMfaFactorRepository(), PasswordHistoryRepo: memory.NewPasswordHistoryRepository(),
		Emit: func(event spec.DomainEvent) { *events = append(*events, event) },
	}
	return deps, consentRepo, userRepo
}

func TestSoftDeleteUserSetsPendingDeletionWithoutCascade(t *testing.T) {
	ctx := context.Background()
	var events []spec.DomainEvent
	deps, consentRepo, userRepo := softDeleteTestDeps(&events)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	userRepo.Seed(&spec.User{
		Sub: "alice-1", PreferredUsername: "alice", PasswordHash: "hash",
		Roles: []string{"support"}, CreatedAt: now, UpdatedAt: now,
	})
	_ = consentRepo.Save(ctx, &spec.Consent{
		TenantID: spec.DefaultTenantID, Sub: "alice-1", ClientID: "client-a",
		Scopes: []string{"openid"}, State: spec.ConsentGranted,
		GrantedAt: now, ExpiresAt: now.AddDate(1, 0, 0),
	})

	if err := idmusecases.SoftDeleteUser(ctx, deps, idmusecases.SoftDeleteUserInput{
		ActorSub: "admin", Sub: "alice-1", Reason: "maybe leaving", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	last, ok := events[len(events)-1].(*spec.UserSoftDeleted)
	if !ok || last.TargetSub != "alice-1" || last.Reason != "maybe leaving" {
		t.Fatalf("expected UserSoftDeleted with target/reason, got %+v", events[len(events)-1])
	}
	// status は PendingDeletion で、FindBySub でまだ見える (tombstone と違い可視)。
	user, _ := userRepo.FindBySub(ctx, "alice-1")
	if user == nil || !user.IsSoftDeleted() || user.IsActive() || user.IsDeleted() {
		t.Fatalf("expected visible soft-deleted user, got %+v", user)
	}
	// PII / cascade artifact は温存される。
	if user.Email != nil && *user.Email == "deleted:alice-1" {
		t.Fatal("PII was anonymized on soft-delete")
	}
	if remaining, _ := consentRepo.FindAll(ctx, spec.DefaultTenantID); len(remaining) != 1 {
		t.Fatalf("consent must be preserved on soft-delete, got %+v", remaining)
	}
	// 冪等: 再 soft-delete は追加イベントを出さない。
	prev := len(events)
	if err := idmusecases.SoftDeleteUser(ctx, deps, idmusecases.SoftDeleteUserInput{
		ActorSub: "admin", Sub: "alice-1", Now: now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if len(events) != prev {
		t.Fatal("idempotent soft-delete emitted extra events")
	}
}

func TestRestoreUserReturnsToActive(t *testing.T) {
	ctx := context.Background()
	var events []spec.DomainEvent
	deps, _, userRepo := softDeleteTestDeps(&events)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	email := "alice@example.com"
	userRepo.Seed(&spec.User{
		Sub: "alice-1", PreferredUsername: "alice", PasswordHash: "hash", Email: &email,
		Roles: []string{"support"}, CreatedAt: now, UpdatedAt: now,
	})
	if err := idmusecases.SoftDeleteUser(ctx, deps, idmusecases.SoftDeleteUserInput{
		ActorSub: "admin", Sub: "alice-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	restored, err := idmusecases.RestoreUser(ctx, deps, "admin", "alice-1", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !restored.IsActive() || restored.Email == nil || *restored.Email != email {
		t.Fatalf("expected active restored user with PII intact, got %+v", restored)
	}
	if got := events[len(events)-1].EventType(); got != "UserRestored" {
		t.Fatalf("last event=%s, want UserRestored", got)
	}
}

func TestRestoreUserRejectsNonPendingAndExpired(t *testing.T) {
	ctx := context.Background()
	var events []spec.DomainEvent
	deps, _, userRepo := softDeleteTestDeps(&events)
	deps.SoftDeleteGraceSeconds = 60
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	userRepo.Seed(&spec.User{
		Sub: "alice-1", PreferredUsername: "alice", PasswordHash: "hash",
		CreatedAt: now, UpdatedAt: now,
	})
	// Active user への restore は ErrUserNotPendingDeletion。
	if _, err := idmusecases.RestoreUser(ctx, deps, "admin", "alice-1", now); !errors.Is(err, idmusecases.ErrUserNotPendingDeletion) {
		t.Fatalf("error=%v, want ErrUserNotPendingDeletion", err)
	}
	if err := idmusecases.SoftDeleteUser(ctx, deps, idmusecases.SoftDeleteUserInput{
		ActorSub: "admin", Sub: "alice-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	// 猶予期間 (60s) 経過後の restore は ErrRestoreGracePeriodExpired。
	if _, err := idmusecases.RestoreUser(ctx, deps, "admin", "alice-1", now.Add(2*time.Minute)); !errors.Is(err, idmusecases.ErrRestoreGracePeriodExpired) {
		t.Fatalf("error=%v, want ErrRestoreGracePeriodExpired", err)
	}
}

func TestSoftDeleteAndRestoreRejectSelf(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	now := time.Now().UTC()
	userRepo.Seed(&spec.User{
		Sub: "admin-1", PreferredUsername: "admin", PasswordHash: "hash",
		Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
	})
	deps := idmusecases.AdminUserDeps{UserRepo: userRepo}
	if err := idmusecases.SoftDeleteUser(ctx, deps, idmusecases.SoftDeleteUserInput{
		ActorSub: "admin-1", Sub: "admin-1", Now: now,
	}); !errors.Is(err, idmusecases.ErrSelfDeleteForbidden) {
		t.Fatalf("soft-delete self error=%v, want ErrSelfDeleteForbidden", err)
	}
	if _, err := idmusecases.RestoreUser(ctx, deps, "admin-1", "admin-1", now); !errors.Is(err, idmusecases.ErrSelfDeleteForbidden) {
		t.Fatalf("restore self error=%v, want ErrSelfDeleteForbidden", err)
	}
}

func TestPurgeExpiredSoftDeletedAnonymizesAfterGrace(t *testing.T) {
	ctx := context.Background()
	var events []spec.DomainEvent
	deps, _, userRepo := softDeleteTestDeps(&events)
	deps.SoftDeleteGraceSeconds = 1
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	userRepo.Seed(&spec.User{
		Sub: "alice-1", PreferredUsername: "alice", PasswordHash: "hash",
		CreatedAt: now, UpdatedAt: now,
	})
	if err := idmusecases.SoftDeleteUser(ctx, deps, idmusecases.SoftDeleteUserInput{
		ActorSub: "admin", Sub: "alice-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	// 猶予期間内 (grace=1s) の purge は no-op。
	if err := idmusecases.PurgeExpiredSoftDeleted(ctx, deps, now); err != nil {
		t.Fatal(err)
	}
	if user, _ := userRepo.FindBySub(ctx, "alice-1"); user == nil || !user.IsSoftDeleted() {
		t.Fatal("user must remain pending within grace")
	}
	// 猶予期間経過後の purge は anonymize cascade を実行し UserDeleted(auto_purge) を emit。
	if err := idmusecases.PurgeExpiredSoftDeleted(ctx, deps, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	tombstone, _ := userRepo.FindBySubIncludingDeleted(ctx, "alice-1")
	if tombstone == nil || !tombstone.IsDeleted() {
		t.Fatalf("expected tombstone after auto-purge, got %+v", tombstone)
	}
	last, ok := events[len(events)-1].(*spec.UserDeleted)
	if !ok || last.Reason != "auto_purge" {
		t.Fatalf("expected UserDeleted(auto_purge), got %+v", events[len(events)-1])
	}
}
