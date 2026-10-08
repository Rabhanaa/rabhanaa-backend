package service

import (
	"context"
	"errors"
	"testing"

	authctx "rabhana/auth/context"
	"rabhana/auth/model"
	"rabhana/pkg/errs"
)

// fakeInterestRepo implements only what UpdateInterests touches; the embedded
// interface panics on anything else, which would flag a new dependency.
type fakeInterestRepo struct {
	AuthRepository
	active      map[int32]bool
	replaced    bool
	replacedIDs []int32
}

func (f *fakeInterestRepo) CountActiveInterestsByIDs(_ context.Context, ids []int32) (int64, error) {
	var n int64
	for _, id := range ids {
		if f.active[id] {
			n++
		}
	}
	return n, nil
}

func (f *fakeInterestRepo) ReplaceUserInterests(_ context.Context, _ int32, ids []int32) error {
	f.replaced = true
	f.replacedIDs = ids
	return nil
}

func newInterestService(min int, active ...int32) (*AuthService, *fakeInterestRepo) {
	repo := &fakeInterestRepo{active: map[int32]bool{}}
	for _, id := range active {
		repo.active[id] = true
	}
	return NewAuthService(repo, &authctx.AuthConfig{MinInterests: min}, nil, nil), repo
}

func TestUpdateInterests(t *testing.T) {
	ctx := context.Background()

	t.Run("saves the set without duplicates", func(t *testing.T) {
		svc, repo := newInterestService(1, 4, 5, 9)
		err := svc.UpdateInterests(ctx, 1, model.InterestsRequest{InterestIDs: []int32{4, 9, 4, 5, 9}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []int32{4, 9, 5}
		if !equalIDs(repo.replacedIDs, want) {
			t.Fatalf("replaced %v, want %v", repo.replacedIDs, want)
		}
	})

	t.Run("duplicates do not count towards the minimum", func(t *testing.T) {
		svc, repo := newInterestService(2, 4, 5)
		err := svc.UpdateInterests(ctx, 1, model.InterestsRequest{InterestIDs: []int32{4, 4}})
		if !errors.Is(err, errs.ErrInsufficientInterests) {
			t.Fatalf("got %v, want ErrInsufficientInterests", err)
		}
		if repo.replaced {
			t.Fatal("interests were replaced despite the error")
		}
	})

	t.Run("rejects inactive or unknown interests and keeps the old set", func(t *testing.T) {
		svc, repo := newInterestService(1, 4, 5)
		err := svc.UpdateInterests(ctx, 1, model.InterestsRequest{InterestIDs: []int32{4, 1, 999}})
		if !errors.Is(err, errs.ErrInvalidInterests) {
			t.Fatalf("got %v, want ErrInvalidInterests", err)
		}
		if repo.replaced {
			t.Fatal("interests were replaced despite the error")
		}
	})
}

func equalIDs(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
