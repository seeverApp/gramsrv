package rpc

import (
	"context"
	"fmt"

	"telesrv/internal/domain"
)

func (r *Router) pinnedDialogsList(ctx context.Context, userID int64, folderID int) (domain.DialogList, error) {
	if r == nil || r.deps.Dialogs == nil {
		return domain.DialogList{}, nil
	}
	key := fmt.Sprintf("%d:%d", userID, folderID)
	value, err, _ := r.dialogsPinnedSF.Do(key, func() (any, error) {
		return r.deps.Dialogs.GetDialogs(ctx, userID, domain.DialogFilter{
			PinnedOnly:  true,
			HasFolderID: true,
			FolderID:    folderID,
			Limit:       100,
		})
	})
	if err != nil {
		return domain.DialogList{}, err
	}
	if list, ok := value.(domain.DialogList); ok {
		return list, nil
	}
	return domain.DialogList{}, nil
}
