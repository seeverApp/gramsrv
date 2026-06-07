package userprojection

import (
	"context"

	"telesrv/internal/domain"
	"telesrv/internal/store"
)

// ForViewer applies the owner-specific user view that Telegram clients expect.
// In particular, phone is visible for self and contacts; non-contacts should not
// receive a phone field because TDesktop will prefer it over the public name.
func ForViewer(ctx context.Context, contacts store.ContactStore, viewerUserID int64, users []domain.User) ([]domain.User, error) {
	if contacts == nil || viewerUserID == 0 || len(users) == 0 {
		return users, nil
	}
	out := make([]domain.User, len(users))
	copy(out, users)
	cache := make(map[int64]domain.User, len(users))
	for i := range out {
		u := out[i]
		if u.ID == 0 || u.ID == viewerUserID || u.ID == domain.OfficialSystemUserID {
			continue
		}
		if projected, ok := cache[u.ID]; ok {
			out[i] = projected
			continue
		}
		projected, err := projectOne(ctx, contacts, viewerUserID, u)
		if err != nil {
			return nil, err
		}
		cache[u.ID] = projected
		out[i] = projected
	}
	return out, nil
}

// One applies ForViewer to a single user.
func One(ctx context.Context, contacts store.ContactStore, viewerUserID int64, user domain.User) (domain.User, error) {
	projected, err := ForViewer(ctx, contacts, viewerUserID, []domain.User{user})
	if err != nil || len(projected) == 0 {
		return domain.User{}, err
	}
	return projected[0], nil
}

func projectOne(ctx context.Context, contacts store.ContactStore, viewerUserID int64, user domain.User) (domain.User, error) {
	contact, found, err := contacts.Get(ctx, viewerUserID, user.ID)
	if err != nil {
		return domain.User{}, err
	}
	if !found {
		user.Phone = ""
		user.Contact = false
		user.Mutual = false
		return user, nil
	}
	projected := user
	projected.Contact = true
	projected.Mutual = contact.Mutual || contact.User.Mutual
	if contact.User.Phone != "" {
		projected.Phone = contact.User.Phone
	} else {
		projected.Phone = contact.Phone
	}
	if contact.User.FirstName != "" || contact.User.LastName != "" {
		projected.FirstName = contact.User.FirstName
		projected.LastName = contact.User.LastName
	} else if contact.FirstName != "" || contact.LastName != "" {
		projected.FirstName = contact.FirstName
		projected.LastName = contact.LastName
	}
	return projected, nil
}
