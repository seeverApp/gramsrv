package layerwire

import (
	"sort"
	"strings"
	"testing"
)

// isInbound reports whether a constructor is a client->server (Input*) type the
// server never emits, so it cannot appear in downgraded output.
func isInbound(cl *ctorLayout) bool {
	return strings.HasPrefix(cl.result, "Input") || strings.HasPrefix(cl.name, "input")
}

// collectReachableTypes returns abstract/bare type names that can appear in
// downgraded output at a layer: those referenced by a *retained* field of a
// constructor that is itself emittable (not a function, not a 227-only type that
// is replaced wholesale, not an inbound Input* type).
func collectReachableTypes(lt *layerTables) map[string]bool {
	refs := map[string]bool{}
	var addField func(f *fieldLayout)
	addField = func(f *fieldLayout) {
		switch f.kind {
		case kindObject, kindBareObject:
			refs[f.typeName] = true
		case kindVector, kindVectorBare:
			addField(f.elem)
		}
	}
	for crc, cl := range canonical.byCRC {
		if cl.isFunc || isInbound(cl) || lt.newTypes[crc] {
			continue
		}
		var keep map[string]bool
		if r := lt.rules[crc]; r != nil && r.structural == "" {
			keep = r.keep
		}
		for i := range cl.fields {
			f := &cl.fields[i]
			if f.isFlags {
				continue
			}
			if keep != nil && !keep[f.name] {
				continue // dropped by a mechanical rule
			}
			addField(f)
		}
	}
	return refs
}

// unemittedAllowlist is the curated set of reachable-but-unhandled 227-only
// constructors that telesrv does not actually emit (confirmed against the
// outbound-constructor scoping audit, 2026-06-25). They live behind features the
// server lacks (instant-view rich pages, AI compose, managed bots, web-browser
// settings, guest chat, star-gift rarity/craft, join-chat bot results). The gate
// fails if a NEW reachable type appears that is neither handled nor listed here,
// forcing a human to triage on every gotd bump / client upgrade.
var unemittedAllowlist = map[string]bool{
	"aiComposeTone":                                 true,
	"aiComposeToneDefault":                          true,
	"aiComposeToneExample":                          true,
	"botInlineMessageRichMessage":                   true,
	"channelAdminLogEventActionParticipantEditRank": true,
	"joinChatBotResultApproved":                     true,
	"joinChatBotResultDeclined":                     true,
	"joinChatBotResultQueued":                       true,
	"joinChatBotResultWebView":                      true,
	"messages.chatInviteJoinResultWebView":          true,
	"messages.emojiGameDiceInfo":                    true,
	"messages.emojiGameUnavailable":                 true,
	"requestPeerTypeCreateBot":                      true,
	"richMessage":                                   true,
	"sendMessageRichMessageDraftAction":             true,
	"starGiftAttributeRarity":                       true,
	"starGiftAttributeRarityEpic":                   true,
	"starGiftAttributeRarityLegendary":              true,
	"starGiftAttributeRarityRare":                   true,
	"starGiftAttributeRarityUncommon":               true,
	"topPeerCategoryBotsGuestChat":                  true,
	"updateAiComposeTones":                          true,
	"updateBotGuestChatQuery":                       true,
	"updateChatParticipantRank":                     true,
	"updateEmojiGameInfo":                           true,
	"updateJoinChatWebViewDecision":                 true,
	"updateManagedBot":                              true,
	"updateNewBotConnection":                        true,
	"updateStarGiftCraftFail":                       true,
	"updateWebBrowserException":                     true,
	"updateWebBrowserSettings":                      true,
	"webDomainException":                            true,
	"webPageAttributeAiComposeTone":                 true,
	// Structural changed-types telesrv does not emit (see design Appendix C and
	// the scoping audit); their hand transforms are deferred to CI-todo.
	"pageListOrderedItemText":   true,
	"pageListOrderedItemBlocks": true,
	"starGiftAttributeModel":    true,
	"starGiftAttributeBackdrop": true,
	"starGiftAttributePattern":  true,
	"urlAuthResultAccepted":     true,
	"inputMediaPoll":            true, // inbound only
}

func newTypeHandled(crc uint32, result string) bool {
	if newTypeFallbacks[crc] != nil {
		return true
	}
	return newTypeFallbacksByType[result] != nil
}

// TestCoverageGate is the drift gate. For every supported layer, each 227-only
// or structural constructor that can appear in downgraded output must be either
// handled (fallback / structural transform) or explicitly allowlisted as not
// emitted. A bare failure means new wire shape slipped in unhandled.
func TestCoverageGate(t *testing.T) {
	for layer := SupportedFloor; layer < CanonicalLayer; layer++ {
		lt := tables[layer]
		if lt == nil {
			t.Fatalf("no tables for layer %d", layer)
		}
		reach := collectReachableTypes(lt)

		// Structural rules that are reachable need a registered transform.
		for crc, r := range lt.rules {
			if r.structural == "" {
				continue
			}
			cl := canonical.byCRC[crc]
			reachable := cl != nil && reach[cl.result] && !isInbound(cl)
			handled := structuralTransforms[r.structural] != nil
			if reachable && !handled && !unemittedAllowlist[nameOf(crc)] {
				t.Errorf("layer %d: reachable structural %s (%#08x) has no transform", layer, nameOf(crc), crc)
			}
		}

		// New constructors reachable through a retained field need a fallback.
		var gaps []string
		for crc := range lt.newTypes {
			cl := canonical.byCRC[crc]
			if cl == nil || cl.isFunc || isInbound(cl) {
				continue
			}
			if !reach[cl.result] {
				continue
			}
			if newTypeHandled(crc, cl.result) || unemittedAllowlist[cl.name] {
				continue
			}
			gaps = append(gaps, cl.name)
		}
		if len(gaps) > 0 {
			sort.Strings(gaps)
			t.Errorf("layer %d: %d reachable 227-only types lack a fallback or allowlist entry:\n  %v", layer, len(gaps), gaps)
		}
	}
}

func nameOf(crc uint32) string {
	if cl := canonical.byCRC[crc]; cl != nil {
		return cl.name
	}
	return "?"
}
