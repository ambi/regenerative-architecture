package usecases

// self-service のセッション一覧と失効 (wi-20 スライス 2)。actor.sub == target.sub に固定し、
// 本人は自分の LoginSession のみ参照・失効できる。失効は LoginSession を物理削除して SSO
// セッションを終了する。OAuth クライアントへ発行済みの refresh token はセッションに 1:1 で
// 紐づかないため本スライスでは失効しない (per-session の refresh 失効は後続スライス)。

import (
	"context"
	"errors"
	"sort"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/shared/spec"
)

// ErrSessionNotFound は対象セッションが存在しないか、本人のものでない場合。
var ErrSessionNotFound = errors.New("session not found")

// SessionDeps はセッション use case の依存。
type SessionDeps struct {
	Store authnports.SessionStore
	Emit  func(spec.DomainEvent)
}

// SessionView は一覧表示用のセッション射影。secret は持たず、識別子と認証情報のみ。
type SessionView struct {
	ID        string
	Current   bool
	AMR       []string
	ACR       string
	StartedAt time.Time
	ExpiresAt time.Time
}

// ListSessions は sub の有効なセッションを開始時刻の降順で返す。currentSessionID に
// 一致するものを Current=true でマークする。
func ListSessions(
	ctx context.Context,
	store authnports.SessionStore,
	sub, currentSessionID string,
) ([]SessionView, error) {
	if store == nil {
		return []SessionView{}, nil
	}
	sessions, err := store.ListBySub(ctx, sub)
	if err != nil {
		return nil, err
	}
	views := make([]SessionView, 0, len(sessions))
	for _, sess := range sessions {
		views = append(views, SessionView{
			ID:        sess.ID,
			Current:   sess.ID == currentSessionID,
			AMR:       sess.AMR,
			ACR:       sess.ACR,
			StartedAt: time.Unix(sess.AuthTime, 0).UTC(),
			ExpiresAt: sess.ExpiresAt,
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].StartedAt.After(views[j].StartedAt) })
	return views, nil
}

// RevokeOwnSession は本人のセッション 1 件を失効する。対象が存在しないか本人のもので
// なければ ErrSessionNotFound。
func RevokeOwnSession(
	ctx context.Context,
	deps SessionDeps,
	sub, sessionID string,
	now time.Time,
) error {
	sess, err := deps.Store.Find(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess == nil || sess.Sub != sub {
		return ErrSessionNotFound
	}
	if err := deps.Store.Delete(ctx, sessionID); err != nil {
		return err
	}
	emitSessionEnded(deps.Emit, sess, sub, spec.SessionEndSelfRevoke, now)
	return nil
}

// RevokeOtherSessions は keepSessionID を除く本人の全セッションを失効する
// ("他のセッションを全て終了")。
func RevokeOtherSessions(
	ctx context.Context,
	deps SessionDeps,
	sub, keepSessionID string,
	now time.Time,
) error {
	sessions, err := deps.Store.ListBySub(ctx, sub)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		if sess.ID == keepSessionID {
			continue
		}
		if err := deps.Store.Delete(ctx, sess.ID); err != nil {
			return err
		}
		emitSessionEnded(deps.Emit, sess, sub, spec.SessionEndSelfRevoke, now)
	}
	return nil
}

func emitSessionEnded(
	emit func(spec.DomainEvent),
	sess *spec.LoginSession,
	actorSub string,
	reason spec.SessionEndReason,
	now time.Time,
) {
	if emit == nil {
		return
	}
	emit(&spec.SessionEnded{
		At: normalizedNow(now), TenantID: sess.TenantID, Sub: sess.Sub,
		SessionID: sess.ID, ActorSub: actorSub, Reason: reason,
	})
}
