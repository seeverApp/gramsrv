// Package layerwire downgrades canonical (Layer 227, the bytes gotd actually
// emits) TL objects to the wire shape expected by older clients (down to
// Layer 220), and is the runtime half of docs/layer-compat-220-227-design.md.
//
// The package is schema-driven: at init it parses the embedded canonical-227
// schema into a per-constructor field layout used by a generic walker; the
// generate-time tables (tables_gen.go, produced by ./gen) describe the
// per-layer downgrade rules. Business handlers and gotd are never touched —
// they always produce Layer 227, and transcoding happens only at the edge and
// only when the negotiated client layer is below 227.
package layerwire

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/gotd/tl"
)

// CanonicalLayer is the layer telesrv's pinned gotd emits.
const CanonicalLayer = 227

// SupportedFloor is the oldest client layer the transcoder targets.
const SupportedFloor = 220

// vectorTypeID is the boxed Vector constructor id.
const vectorTypeID = 0x1cb5c415

//go:embed schema/canonical-227.tl
var canonicalSchema string

// wireKind is the on-wire representation of a single TL value.
type wireKind uint8

const (
	kindInt        wireKind = iota // 4 bytes
	kindLong                       // 8 bytes
	kindDouble                     // 8 bytes
	kindInt128                     // 16 bytes
	kindInt256                     // 32 bytes
	kindBytes                      // TL bytes (length-prefixed, padded)
	kindString                     // TL string (same wire as bytes)
	kindBool                       // boxed Bool (4-byte CRC)
	kindTrue                       // flag-only pseudo value, 0 bytes
	kindVector                     // boxed Vector<elem> (0x1cb5c415 + n + elems)
	kindVectorBare                 // bare vector<elem> (n + elems, no id)
	kindObject                     // boxed object (4-byte CRC + body)
	kindBareObject                 // bare object (body only, resolved by typeName)
)

// fieldLayout is one parameter of a constructor with its decoded wire shape.
type fieldLayout struct {
	name     string
	kind     wireKind
	isFlags  bool         // this is a `#` flags integer
	flagName string       // when conditional: which flags integer gates it
	flagBit  int          // when conditional: bit index; -1 otherwise
	elem     *fieldLayout // vector element layout
	typeName string       // object/bareObject: (qualified) referenced type name
}

func (f fieldLayout) conditional() bool { return f.flagBit >= 0 }

// ctorLayout is the decoded field layout of a single constructor.
type ctorLayout struct {
	crc    uint32
	name   string // qualified TL name, e.g. "messages.dialogs" / "message"
	result string // qualified result (abstract) type name
	fields []fieldLayout
	isFunc bool
}

// schemaModel is the parsed canonical schema indexed for the walker.
type schemaModel struct {
	byCRC    map[uint32]*ctorLayout
	byName   map[string]*ctorLayout   // qualified ctor name -> layout
	bareByT  map[string]*ctorLayout   // bare type name -> its single constructor
	ctorsOfT map[string][]*ctorLayout // abstract result type -> constructors
}

// canonical is the parsed Layer 227 model, built once at init.
var canonical = mustLoadCanonical()

func mustLoadCanonical() *schemaModel {
	m, err := parseSchemaModel(canonicalSchema)
	if err != nil {
		panic("layerwire: parse canonical schema: " + err.Error())
	}
	return m
}

func qualifyName(ns []string, name string) string {
	if len(ns) == 0 {
		return name
	}
	return strings.Join(ns, ".") + "." + name
}

func qualifyType(t tl.Type) string {
	return qualifyName(t.Namespace, t.Name)
}

func parseSchemaModel(src string) (*schemaModel, error) {
	parsed, err := tl.Parse(strings.NewReader(src))
	if err != nil {
		return nil, err
	}
	m := &schemaModel{
		byCRC:    make(map[uint32]*ctorLayout),
		byName:   make(map[string]*ctorLayout),
		bareByT:  make(map[string]*ctorLayout),
		ctorsOfT: make(map[string][]*ctorLayout),
	}
	for i := range parsed.Definitions {
		sd := parsed.Definitions[i]
		d := sd.Definition
		name := qualifyName(d.Namespace, d.Name)
		if name == "vector" {
			continue // implicit Vector pseudo-definition
		}
		cl := &ctorLayout{
			crc:    d.ID,
			name:   name,
			result: qualifyType(d.Type),
			isFunc: sd.Category == tl.CategoryFunction,
		}
		for _, p := range d.Params {
			fl, err := toFieldLayout(p)
			if err != nil {
				return nil, fmt.Errorf("%s field %q: %w", name, p.Name, err)
			}
			cl.fields = append(cl.fields, fl)
		}
		if prev, ok := m.byCRC[cl.crc]; ok && prev.name != cl.name {
			return nil, fmt.Errorf("crc collision %#08x: %s vs %s", cl.crc, prev.name, cl.name)
		}
		m.byCRC[cl.crc] = cl
		m.byName[cl.name] = cl
		if !cl.isFunc {
			m.ctorsOfT[cl.result] = append(m.ctorsOfT[cl.result], cl)
			// A bare type name is the lowercase constructor name itself.
			m.bareByT[cl.name] = cl
		}
	}
	return m, nil
}

func toFieldLayout(p tl.Parameter) (fieldLayout, error) {
	if p.Flags {
		return fieldLayout{name: p.Name, kind: kindInt, isFlags: true, flagBit: -1}, nil
	}
	fl := fieldLayout{name: p.Name, flagBit: -1}
	if p.Flag != nil {
		fl.flagName = p.Flag.Name
		fl.flagBit = p.Flag.Index
	}
	kind, typeName, elem, err := resolveType(p.Type)
	if err != nil {
		return fieldLayout{}, err
	}
	fl.kind = kind
	fl.typeName = typeName
	fl.elem = elem
	return fl, nil
}

func resolveType(t tl.Type) (kind wireKind, typeName string, elem *fieldLayout, err error) {
	if t.GenericArg != nil {
		ek, etn, eel, eerr := resolveType(*t.GenericArg)
		if eerr != nil {
			return 0, "", nil, eerr
		}
		el := &fieldLayout{kind: ek, typeName: etn, elem: eel, flagBit: -1}
		if t.Name == "vector" { // bare vector
			return kindVectorBare, "", el, nil
		}
		return kindVector, "", el, nil
	}
	switch t.Name {
	case "int":
		return kindInt, "", nil, nil
	case "long":
		return kindLong, "", nil, nil
	case "double":
		return kindDouble, "", nil, nil
	case "int128":
		return kindInt128, "", nil, nil
	case "int256":
		return kindInt256, "", nil, nil
	case "bytes":
		return kindBytes, "", nil, nil
	case "string":
		return kindString, "", nil, nil
	case "Bool":
		return kindBool, "", nil, nil
	case "true":
		return kindTrue, "", nil, nil
	}
	if t.Bare {
		return kindBareObject, qualifyType(t), nil, nil
	}
	return kindObject, qualifyType(t), nil, nil
}
