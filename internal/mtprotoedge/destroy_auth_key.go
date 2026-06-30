package mtprotoedge

import (
	"fmt"

	"github.com/gotd/td/bin"
)

const (
	destroyAuthKeyRequestTypeID = 0xd1435160
	destroyAuthKeyOkTypeID      = 0xf660e1d4
	destroyAuthKeyFailTypeID    = 0xea109b13
)

type destroyAuthKeyRequest struct{}

func (*destroyAuthKeyRequest) Encode(b *bin.Buffer) error {
	b.PutID(destroyAuthKeyRequestTypeID)
	return nil
}

func (*destroyAuthKeyRequest) Decode(b *bin.Buffer) error {
	if err := b.ConsumeID(destroyAuthKeyRequestTypeID); err != nil {
		return fmt.Errorf("decode destroy_auth_key: %w", err)
	}
	return nil
}

type destroyAuthKeyOk struct{}

func (*destroyAuthKeyOk) Encode(b *bin.Buffer) error {
	b.PutID(destroyAuthKeyOkTypeID)
	return nil
}

type destroyAuthKeyFail struct{}

func (*destroyAuthKeyFail) Encode(b *bin.Buffer) error {
	b.PutID(destroyAuthKeyFailTypeID)
	return nil
}
