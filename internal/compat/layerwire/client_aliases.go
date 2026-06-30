package layerwire

// clientMethodAliases maps method constructor ids emitted by a specific client's
// hand-maintained TL (constructor *drift*, NOT official api.tl layer drift) to
// the canonical (227) id — for the subset whose request body is byte-identical
// to canonical so a 4-byte id swap suffices.
//
// This is the second, hand-maintained half of the inbound compat table; the
// generated inboundMethodUpgrades (tables_gen.go) covers official layer drift
// derived from TDesktop api.tl, which by construction never contains these
// client-private ids. Entries here are sourced from client source (e.g. DrKLO
// TLRPC.java), each verified body-compatible against the canonical layout.
//
// Client-drift constructors whose body differs structurally (a missing flags
// integer, a different field type, or that need business logic such as
// access_hash resolution or a legacy-shaped response) are NOT here — they remain
// dedicated decode handlers in internal/rpc (dispatchCompat), because the body
// cannot be reused as-is and the transform needs more than an id swap.
var clientMethodAliases = map[uint32]uint32{
	// DrKLO Android (post-Layer225) messages.forwardMessages. Wire layout is
	// identical to canonical #13704a7c for every flag bit the client can set
	// (the only schema delta is flags it never sets), so the body decodes as-is.
	0x41d41ade: 0x13704a7c,
	// DrKLO Android channels.inviteToChannel. Body is still
	// channel:InputChannel users:Vector<InputUser> = canonical #c9e33d54.
	0x199f3a6c: 0xc9e33d54,
	// DrKLO Android updates.getDifference. Old layout only uses flags.0
	// (pts_total_limit); canonical #19c2f763 adds pts_limit(flags.1)/
	// qts_limit(flags.2) which the client leaves clear ⇒ zero wire bytes, so the
	// old body decodes byte-for-byte as canonical.
	0x25939651: 0x19c2f763,
	// DrKLO Android messages.createChat. Body is byte-identical to canonical
	// #92ceddd4; the legacy-shaped response is produced by ClientType==Android
	// (createChatNeedsLegacyChat), so no dedicated handler is needed.
	0x0034a818: 0x92ceddd4,
}
