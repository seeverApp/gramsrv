// Command layerwire-gen diffs the canonical gotd schema (Layer 227, the bytes
// telesrv actually emits) against historical TDesktop api.tl layers (220..226)
// and classifies every per-constructor change as either MECHANICAL (a pure
// append-only delta that can be downgraded by dropping trailing/optional fields
// and masking flag bits) or STRUCTURAL (field reorder / reinterpretation that
// needs a hand-written transform).
//
// It is the generate-time half of the layer-compat design
// (docs/layer-compat-220-227-design.md). Run from the telesrv module root:
//
//	go run ./internal/compat/layerwire/gen -report
//
// This first iteration only prints a report so the numbers can be validated
// against the design doc before any table is emitted.
package main

import (
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gotd/tl"
)

// canonicalLayer is the layer telesrv's gotd is pinned to.
const canonicalLayer = 227

// supportedFloor is the oldest client layer telesrv aims to serve.
const supportedFloor = 220

// spec is a single TL constructor or method with field-level metadata.
type spec struct {
	qname  string // qualified name, e.g. "messages.dialogs" or "message"
	crc    uint32
	params []tl.Parameter
	isFunc bool
}

// schema indexes one parsed .tl file by qualified name and by CRC.
type schema struct {
	layer   int
	byName  map[string]*spec
	byCRC   map[uint32]*spec
	ordered []*spec
}

func qualify(d tl.Definition) string {
	if len(d.Namespace) == 0 {
		return d.Name
	}
	return strings.Join(d.Namespace, ".") + "." + d.Name
}

func load(path string) (*schema, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	parsed, err := tl.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	s := &schema{
		layer:  parsed.Layer,
		byName: make(map[string]*spec),
		byCRC:  make(map[uint32]*spec),
	}
	for i := range parsed.Definitions {
		sd := parsed.Definitions[i]
		d := sd.Definition
		sp := &spec{
			qname:  qualify(d),
			crc:    d.ID,
			params: d.Params,
			isFunc: sd.Category == tl.CategoryFunction,
		}
		// Skip the implicit vector pseudo-definition if present.
		if sp.qname == "vector" {
			continue
		}
		s.byName[sp.qname] = sp
		s.byCRC[sp.crc] = sp
		s.ordered = append(s.ordered, sp)
	}
	return s, nil
}

// classifyResult describes how a changed constructor downgrades from canonical
// (227) to a target layer.
type classifyResult struct {
	mechanical bool
	drops      []string // canonical fields absent at the target layer
	reason     string   // populated when !mechanical
}

// classifyDowngrade aligns the target params as a subsequence (by name) of the
// canonical params. Success ⇒ mechanical drop of the unmatched canonical fields.
// Any name mismatch, type change, or flag-condition change ⇒ structural.
func classifyDowngrade(from, to *spec) classifyResult {
	var drops []string
	i, j := 0, 0
	fp, tp := from.params, to.params
	for j < len(tp) {
		// Advance over canonical fields until we reach the target field name.
		for i < len(fp) && fp[i].Name != tp[j].Name {
			drops = append(drops, fp[i].Name)
			i++
		}
		if i == len(fp) {
			return classifyResult{reason: fmt.Sprintf("target field %q not found in canonical (reorder/insert)", tp[j].Name)}
		}
		if reason := compatible(fp[i], tp[j]); reason != "" {
			return classifyResult{reason: fmt.Sprintf("field %q: %s", tp[j].Name, reason)}
		}
		i++
		j++
	}
	for ; i < len(fp); i++ {
		drops = append(drops, fp[i].Name)
	}
	return classifyResult{mechanical: true, drops: drops}
}

// compatible reports "" if a kept field is wire-compatible between canonical and
// target, or a reason string otherwise.
func compatible(f, t tl.Parameter) string {
	if f.Flags != t.Flags {
		return "flags-int vs field mismatch"
	}
	if f.Flags {
		// Both are `#` flag integers; the name must match because conditional
		// fields reference it by name.
		if f.Name != t.Name {
			return fmt.Sprintf("flags int renamed %q->%q", t.Name, f.Name)
		}
		return ""
	}
	// Conditional-ness must match exactly (no flag-bit remap supported yet).
	fc, tc := f.Flag != nil, t.Flag != nil
	if fc != tc {
		return "conditional-ness changed"
	}
	if fc {
		if f.Flag.Name != t.Flag.Name || f.Flag.Index != t.Flag.Index {
			return fmt.Sprintf("flag moved %s.%d->%s.%d", t.Flag.Name, t.Flag.Index, f.Flag.Name, f.Flag.Index)
		}
	}
	if f.Type.String() != t.Type.String() {
		return fmt.Sprintf("type changed %s->%s", t.Type.String(), f.Type.String())
	}
	return ""
}

type changed struct {
	qname          string
	fromCRC, toCRC uint32
	res            classifyResult
}

// diff compares canonical (from) against a single target layer (to).
type diffResult struct {
	layer          int
	changedTypes   []changed
	changedMethods []changed
	newTypes       []string // exist in canonical, absent at target
	newMethods     []string
	removedTypes   []string // exist at target, absent in canonical
}

func diff(from, to *schema) diffResult {
	r := diffResult{layer: to.layer}
	for _, sp := range from.ordered {
		other, ok := to.byName[sp.qname]
		if !ok {
			if sp.isFunc {
				r.newMethods = append(r.newMethods, sp.qname)
			} else {
				r.newTypes = append(r.newTypes, sp.qname)
			}
			continue
		}
		if other.crc == sp.crc {
			continue
		}
		c := changed{qname: sp.qname, fromCRC: sp.crc, toCRC: other.crc, res: classifyDowngrade(sp, other)}
		if sp.isFunc {
			r.changedMethods = append(r.changedMethods, c)
		} else {
			r.changedTypes = append(r.changedTypes, c)
		}
	}
	for _, sp := range to.ordered {
		if _, ok := from.byName[sp.qname]; !ok {
			r.removedTypes = append(r.removedTypes, sp.qname)
		}
	}
	return r
}

func main() {
	var (
		schemaDir = flag.String("schema", "internal/compat/layerwire/_schema", "dir with layer-NNN.tl")
		canonical = flag.String("canonical", "internal/compat/layerwire/schema/canonical-227.tl", "gotd canonical 227 schema")
		emit      = flag.String("emit", "", "write generated tables_gen.go to this path")
		_         = flag.Bool("report", true, "print report")
	)
	flag.Parse()

	canon, err := load(*canonical)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load canonical:", err)
		os.Exit(1)
	}

	if *emit != "" {
		if err := emitTables(canon, *schemaDir, *emit); err != nil {
			fmt.Fprintln(os.Stderr, "emit:", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", *emit)
		return
	}
	fmt.Printf("canonical (gotd) layer=%d  defs=%d\n", canon.layer, len(canon.ordered))

	// Per-layer diff + union across the supported range.
	unionChangedTypes := map[string]bool{}
	unionChangedMethods := map[string]bool{}
	unionNewTypes := map[string]bool{}
	unionNewMethods := map[string]bool{}
	structuralTypes := map[string]string{} // qname -> reason (worst case seen)

	for L := supportedFloor; L < canonicalLayer; L++ {
		path := filepath.Join(*schemaDir, fmt.Sprintf("layer-%d.tl", L))
		tgt, err := load(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "load", path, ":", err)
			os.Exit(1)
		}
		r := diff(canon, tgt)
		mech, struc := 0, 0
		for _, c := range r.changedTypes {
			unionChangedTypes[c.qname] = true
			if c.res.mechanical {
				mech++
			} else {
				struc++
				structuralTypes[c.qname] = c.res.reason
			}
		}
		for _, c := range r.changedMethods {
			unionChangedMethods[c.qname] = true
		}
		for _, n := range r.newTypes {
			unionNewTypes[n] = true
		}
		for _, n := range r.newMethods {
			unionNewMethods[n] = true
		}
		fmt.Printf("layer %d: defs=%d  changedTypes=%d (mech=%d struc=%d)  changedMethods=%d  newTypes=%d  newMethods=%d  removed=%d\n",
			L, len(tgt.ordered), len(r.changedTypes), mech, struc, len(r.changedMethods), len(r.newTypes), len(r.newMethods), len(r.removedTypes))
	}

	fmt.Printf("\n=== UNION %d..%d vs %d ===\n", supportedFloor, canonicalLayer-1, canonicalLayer)
	fmt.Printf("changed types:   %d\n", len(unionChangedTypes))
	fmt.Printf("changed methods: %d\n", len(unionChangedMethods))
	fmt.Printf("new types:       %d\n", len(unionNewTypes))
	fmt.Printf("new methods:     %d\n", len(unionNewMethods))
	fmt.Printf("structural types (need hand transform): %d\n", len(structuralTypes))
	for _, q := range sortedKeys(structuralTypes) {
		fmt.Printf("  - %s : %s\n", q, structuralTypes[q])
	}

	// Detailed 220-vs-227 drop table (matches design doc Appendix A).
	fmt.Printf("\n=== 220 vs 227 changed-type drop table ===\n")
	tgt220, _ := load(filepath.Join(*schemaDir, "layer-220.tl"))
	r := diff(canon, tgt220)
	sort.Slice(r.changedTypes, func(a, b int) bool { return r.changedTypes[a].qname < r.changedTypes[b].qname })
	for _, c := range r.changedTypes {
		tag := "MECH"
		detail := "drop: " + strings.Join(c.res.drops, ", ")
		if !c.res.mechanical {
			tag = "STRUCT"
			detail = c.res.reason
		}
		fmt.Printf("  [%-6s] %-34s %#08x->%#08x  %s\n", tag, c.qname, c.toCRC, c.fromCRC, detail)
	}
}

// emitTables writes the runtime downgrade tables (tables_gen.go) for every
// supported layer: per changed constructor a mechanical keep-list or a
// structural marker, plus the set of canonical CRCs absent at that layer.
func emitTables(canon *schema, schemaDir, outPath string) error {
	var b strings.Builder
	b.WriteString("// Code generated by ./internal/compat/layerwire/gen; DO NOT EDIT.\n")
	b.WriteString("// Source: gotd canonical schema (Layer 227) diffed against TDesktop api.tl@N.\n\n")
	b.WriteString("package layerwire\n\n")
	b.WriteString("// generatedTables maps a supported client layer to its canonical(227)->layer\n")
	b.WriteString("// downgrade table. See docs/layer-compat-220-227-design.md.\n")
	b.WriteString("var generatedTables = map[int]layerRaw{\n")

	// inbound 方法升级（扁平：老方法 CRC -> 227 CRC）。老 CRC 本身编码了格式，故无需 layer 维度。
	// 仅收"升级安全"的方法：227 新增字段全为 flag-gated 条件字段（老客户端清零位=零字节，
	// 其 body 本就是合法 227 body，换 4 字节 CRC 即可交给 227 handler）。
	inboundUpgrades := map[uint32]uint32{} // oldCRC -> 227CRC
	inboundUnsafe := map[string]string{}   // qname -> reason

	for L := supportedFloor; L < canonicalLayer; L++ {
		tgt, err := load(filepath.Join(schemaDir, fmt.Sprintf("layer-%d.tl", L)))
		if err != nil {
			return err
		}
		r := diff(canon, tgt)
		for _, c := range r.changedMethods {
			canonSpec := canon.byName[c.qname]
			if reason := methodUpgradeSafe(canonSpec, c.res); reason == "" {
				inboundUpgrades[c.toCRC] = c.fromCRC // client(old) -> canonical(227)
			} else if _, done := inboundUpgrades[c.toCRC]; !done {
				inboundUnsafe[c.qname] = reason
			}
		}
		fmt.Fprintf(&b, "\t%d: {\n", L)

		sort.Slice(r.changedTypes, func(i, j int) bool { return r.changedTypes[i].fromCRC < r.changedTypes[j].fromCRC })
		b.WriteString("\t\trules: map[uint32]ruleRaw{\n")
		for _, c := range r.changedTypes {
			canonSpec := canon.byName[c.qname]
			if c.res.mechanical {
				dropSet := map[string]bool{}
				for _, d := range c.res.drops {
					dropSet[d] = true
				}
				var keep []string
				for _, p := range canonSpec.params {
					if !dropSet[p.Name] {
						keep = append(keep, p.Name)
					}
				}
				fmt.Fprintf(&b, "\t\t\t0x%08x: {target: 0x%08x, keep: %s}, // %s\n", c.fromCRC, c.toCRC, goStrSlice(keep), c.qname)
			} else {
				fmt.Fprintf(&b, "\t\t\t0x%08x: {target: 0x%08x, structural: %q}, // %s\n", c.fromCRC, c.toCRC, c.qname, c.res.reason)
			}
		}
		b.WriteString("\t\t},\n")

		var newCRC []uint32
		for _, q := range r.newTypes {
			if sp := canon.byName[q]; sp != nil {
				newCRC = append(newCRC, sp.crc)
			}
		}
		sort.Slice(newCRC, func(i, j int) bool { return newCRC[i] < newCRC[j] })
		b.WriteString("\t\tnewTypes: []uint32{")
		for i, c := range newCRC {
			if i%6 == 0 {
				b.WriteString("\n\t\t\t")
			}
			fmt.Fprintf(&b, "0x%08x, ", c)
		}
		if len(newCRC) > 0 {
			b.WriteString("\n\t\t")
		}
		b.WriteString("},\n")
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n\n")

	// Flat inbound method CRC upgrade table.
	b.WriteString("// inboundMethodUpgrades maps an old client method constructor id to the\n")
	b.WriteString("// canonical (227) id. Only upgrade-safe changes (all 227 additions flag-gated)\n")
	b.WriteString("// are listed: rewriting the 4-byte id yields a valid 227 request body.\n")
	if len(inboundUnsafe) > 0 {
		b.WriteString("// NOT upgrade-safe as a pure id swap (declare a body transform in client-drift.tl when needed):\n")
		for _, q := range sortedKeys(inboundUnsafe) {
			fmt.Fprintf(&b, "//   %s: %s\n", q, inboundUnsafe[q])
		}
	}
	b.WriteString("var inboundMethodUpgrades = map[uint32]uint32{\n")
	oldCRCs := make([]uint32, 0, len(inboundUpgrades))
	for old := range inboundUpgrades {
		oldCRCs = append(oldCRCs, old)
	}
	sort.Slice(oldCRCs, func(i, j int) bool { return oldCRCs[i] < oldCRCs[j] })
	for _, old := range oldCRCs {
		fmt.Fprintf(&b, "\t0x%08x: 0x%08x, // %s\n", old, inboundUpgrades[old], canon.byCRC[inboundUpgrades[old]].qname)
	}
	b.WriteString("}\n")

	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		_ = os.WriteFile(outPath, []byte(b.String()), 0o644)
		return fmt.Errorf("gofmt: %w", err)
	}
	return os.WriteFile(outPath, formatted, 0o644)
}

// methodUpgradeSafe reports "" if a layer-N request body for a changed method
// is also a valid 227 body after only swapping the constructor id — i.e. the
// downgrade is mechanical and every 227-only field is flag-gated (a conditional
// field the old client leaves clear ⇒ zero wire bytes). A 227-only non-conditional
// field or an inserted flags integer breaks the byte alignment ⇒ unsafe.
func methodUpgradeSafe(canonSpec *spec, res classifyResult) string {
	if !res.mechanical {
		return res.reason
	}
	byName := map[string]tl.Parameter{}
	for _, p := range canonSpec.params {
		byName[p.Name] = p
	}
	for _, d := range res.drops {
		p, ok := byName[d]
		if !ok {
			return fmt.Sprintf("dropped field %q not in canonical", d)
		}
		if p.Flags {
			return fmt.Sprintf("227 inserts flags integer %q", d)
		}
		if p.Flag == nil {
			return fmt.Sprintf("227-only field %q is non-conditional", d)
		}
	}
	return ""
}

func goStrSlice(ss []string) string {
	var b strings.Builder
	b.WriteString("[]string{")
	for i, s := range ss {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(s))
	}
	b.WriteString("}")
	return b.String()
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
