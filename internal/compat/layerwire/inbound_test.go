package layerwire

import "testing"

// TestInboundUpgradeTableWellFormed checks every inbound upgrade maps an old id
// to a real canonical method id, and that the old id is genuinely historical
// (not already a canonical constructor).
func TestInboundUpgradeTableWellFormed(t *testing.T) {
	if len(inboundMethodUpgrades) == 0 {
		t.Fatal("inboundMethodUpgrades is empty")
	}
	for oldID, newID := range inboundMethodUpgrades {
		cl := canonical.byCRC[newID]
		if cl == nil {
			t.Errorf("upgrade target %#08x is not a canonical constructor", newID)
			continue
		}
		if !cl.isFunc {
			t.Errorf("upgrade target %s (%#08x) is not a method", cl.name, newID)
		}
		if oldID == newID {
			t.Errorf("%s: old id equals canonical id %#08x", cl.name, oldID)
		}
		if prev := canonical.byCRC[oldID]; prev != nil {
			t.Errorf("old id %#08x collides with canonical %s", oldID, prev.name)
		}
	}
}

// TestClientMethodAliasesWellFormed checks every hand-maintained client-drift
// alias maps to a real canonical method, and is reachable via UpgradeMethodCRC.
func TestClientMethodAliasesWellFormed(t *testing.T) {
	for oldID, newID := range clientMethodAliases {
		cl := canonical.byCRC[newID]
		if cl == nil || !cl.isFunc {
			t.Errorf("alias target %#08x is not a canonical method", newID)
		}
		if _, ok := inboundMethodUpgrades[oldID]; ok {
			t.Errorf("alias %#08x duplicates a generated upgrade entry", oldID)
		}
		if got, ok := UpgradeMethodCRC(oldID); !ok || got != newID {
			t.Errorf("UpgradeMethodCRC(%#08x) = (%#08x,%v), want (%#08x,true)", oldID, got, ok, newID)
		}
	}
}

// TestInboundDriftCoverage is the inbound drift gate: it statically proves every
// client-drift constructor in client-drift.tl can be upgraded to its canonical
// method — shared fields match (or have a converter), canonical-only required
// fields are defaultable, and renamed fields are mapped. Adding a TL line that
// isn't auto-upgradable fails here, telling the author exactly what converter or
// rename to declare (instead of discovering it at runtime).
func TestInboundDriftCoverage(t *testing.T) {
	defaultable := map[wireKind]bool{
		kindInt: true, kindLong: true, kindDouble: true, kindInt128: true, kindInt256: true,
		kindBytes: true, kindString: true, kindBool: true, kindVector: true, kindVectorBare: true,
	}
	for crc, old := range driftModel.byCRC {
		target := canonical.byName[old.name]
		if target == nil {
			t.Errorf("drift %s (%#08x): no canonical method of that name", old.name, crc)
			continue
		}
		oldHas := map[string]*fieldLayout{}
		for i := range old.fields {
			oldHas[old.fields[i].name] = &old.fields[i]
		}
		for i := range target.fields {
			nf := &target.fields[i]
			if nf.isFlags {
				continue
			}
			oldName := nf.name
			if m, ok := driftFieldRenames[old.name+"\x00"+nf.name]; ok {
				oldName = m
			}
			if of, ok := oldHas[oldName]; ok {
				if typeSig(of) != typeSig(nf) && fieldConverters[typeSig(of)+"->"+typeSig(nf)] == nil {
					t.Errorf("drift %s: field %q needs converter %s->%s", old.name, nf.name, typeSig(of), typeSig(nf))
				}
				continue
			}
			if nf.conditional() || nf.kind == kindTrue {
				continue // optional canonical-only field — left absent
			}
			if !defaultable[nf.kind] {
				t.Errorf("drift %s: canonical-only required field %q (kind %d) is not defaultable; declare a transform", old.name, nf.name, nf.kind)
			}
		}
	}
}

// TestInboundUpgradeSendMessage validates the full chain for the highest-value
// method: a layer-220 client's messages.sendMessage id upgrades to the 227 id.
func TestInboundUpgradeSendMessage(t *testing.T) {
	m220 := loadLayerModel(t, 220)
	old, ok := m220.byName["messages.sendMessage"]
	if !ok {
		t.Fatal("messages.sendMessage missing from layer-220 schema")
	}
	canon, ok := canonical.byName["messages.sendMessage"]
	if !ok {
		t.Fatal("messages.sendMessage missing from canonical schema")
	}
	if old.crc == canon.crc {
		t.Skip("sendMessage unchanged 220->227; nothing to upgrade")
	}
	newID, ok := UpgradeMethodCRC(old.crc)
	if !ok {
		t.Fatalf("sendMessage@220 (%#08x) not in upgrade table", old.crc)
	}
	if newID != canon.crc {
		t.Fatalf("sendMessage upgrade = %#08x, want canonical %#08x", newID, canon.crc)
	}
}
