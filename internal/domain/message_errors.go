package domain

import "errors"

var (
	ErrMessageIDInvalid       = errors.New("message id invalid")
	ErrMessageEmpty           = errors.New("message empty")
	ErrMessageAuthorRequired  = errors.New("message author required")
	ErrMessageNotModified     = errors.New("message not modified")
	ErrMessageNotReadYet      = errors.New("message not read yet")
	ErrReplyMessageIDInvalid  = errors.New("reply message id invalid")
	ErrChatForwardsRestricted = errors.New("chat forwards restricted")
	// ErrPinnedSavedDialogsTooMuch 映射 PINNED_TOO_MUCH：收藏夹子会话置顶
	// 数量达到 MaxPinnedSavedDialogs 上限。
	ErrPinnedSavedDialogsTooMuch = errors.New("pinned saved dialogs too much")
	// ErrPinnedDialogsTooMuch 映射 PINNED_DIALOGS_TOO_MUCH：目标 folder 内
	// 置顶会话数量达到上限。
	ErrPinnedDialogsTooMuch = errors.New("pinned dialogs too much")
)
