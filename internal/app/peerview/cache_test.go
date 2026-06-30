package peerview

import (
	"context"
	"reflect"
	"testing"

	"telesrv/internal/domain"
)

func TestBatchCacheCachesPerViewer(t *testing.T) {
	resolver := &captureUserResolver{
		users: map[int64]domain.User{
			1000000001: {ID: 1000000001, FirstName: "Alice"},
		},
	}
	cache := NewBatchCache(resolver)

	got, err := cache.UsersForView(context.Background(), 1000000002, []int64{1000000001, 1000000001})
	if err != nil {
		t.Fatalf("UsersForView: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1000000001 {
		t.Fatalf("users = %+v, want Alice once", got)
	}
	got, err = cache.UsersForView(context.Background(), 1000000002, []int64{1000000001})
	if err != nil {
		t.Fatalf("UsersForView cached: %v", err)
	}
	got, err = cache.UsersForView(context.Background(), 1000000003, []int64{1000000001})
	if err != nil {
		t.Fatalf("UsersForView other viewer: %v", err)
	}

	wantCalls := []resolverCall{
		{viewerUserID: 1000000002, ids: []int64{1000000001}},
		{viewerUserID: 1000000003, ids: []int64{1000000001}},
	}
	if !reflect.DeepEqual(resolver.calls, wantCalls) {
		t.Fatalf("resolver calls = %+v, want %+v", resolver.calls, wantCalls)
	}
}

// TestBatchCachePrimeServesWithoutResolver：Prime 预热的 viewer 用户被 UsersForView 直接命中，
// 不再回落 resolver（fan-out 跨 viewer 投影预热把 per-recipient ByIDs 折叠成一次 ForViewers 的前提）；
// 且 Prime 不覆盖已解析的同 id（按需解析优先）。
func TestBatchCachePrimeServesWithoutResolver(t *testing.T) {
	resolver := &captureUserResolver{
		users: map[int64]domain.User{
			1000000001: {ID: 1000000001, FirstName: "Resolved"},
		},
	}
	cache := NewBatchCache(resolver)

	const viewer = int64(1000000002)
	cache.Prime(viewer, []domain.User{{ID: 1000000001, FirstName: "Primed"}, {ID: 1000000009, FirstName: "PrimedOnly"}})

	got, err := cache.UsersForView(context.Background(), viewer, []int64{1000000001, 1000000009})
	if err != nil {
		t.Fatalf("UsersForView: %v", err)
	}
	byID := map[int64]domain.User{}
	for _, u := range got {
		byID[u.ID] = u
	}
	if byID[1000000001].FirstName != "Primed" || byID[1000000009].FirstName != "PrimedOnly" {
		t.Fatalf("primed users = %+v, want Primed/PrimedOnly served from cache", got)
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("resolver called %d times, want 0 (all served from prime)", len(resolver.calls))
	}

	// 已解析的 id 不被后续 Prime 覆盖。
	other := &captureUserResolver{users: map[int64]domain.User{1000000003: {ID: 1000000003, FirstName: "Resolved3"}}}
	c2 := NewBatchCache(other)
	if _, err := c2.UsersForView(context.Background(), viewer, []int64{1000000003}); err != nil {
		t.Fatalf("resolve 3: %v", err)
	}
	c2.Prime(viewer, []domain.User{{ID: 1000000003, FirstName: "ShouldNotOverwrite"}})
	got2, err := c2.UsersForView(context.Background(), viewer, []int64{1000000003})
	if err != nil {
		t.Fatalf("UsersForView after prime: %v", err)
	}
	if len(got2) != 1 || got2[0].FirstName != "Resolved3" {
		t.Fatalf("after prime = %+v, want resolved value preserved (no overwrite)", got2)
	}
}

type resolverCall struct {
	viewerUserID int64
	ids          []int64
}

type captureUserResolver struct {
	users map[int64]domain.User
	calls []resolverCall
}

func (r *captureUserResolver) ByIDs(_ context.Context, viewerUserID int64, userIDs []int64) ([]domain.User, error) {
	r.calls = append(r.calls, resolverCall{viewerUserID: viewerUserID, ids: append([]int64(nil), userIDs...)})
	out := make([]domain.User, 0, len(userIDs))
	for _, id := range userIDs {
		if u, ok := r.users[id]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}
