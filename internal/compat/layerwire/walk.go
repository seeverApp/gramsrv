package layerwire

import (
	"fmt"

	"github.com/gotd/td/bin"
)

// skipObject advances b past one boxed object (CRC + body), resolving the
// constructor from the canonical schema.
func (m *schemaModel) skipObject(b *bin.Buffer) error {
	id, err := b.PeekID()
	if err != nil {
		return err
	}
	cl, ok := m.byCRC[id]
	if !ok {
		return fmt.Errorf("layerwire: unknown constructor %#08x", id)
	}
	if err := b.ConsumeID(id); err != nil {
		return err
	}
	return m.skipCtorBody(b, cl)
}

// skipCtorBody advances b past a constructor body (no leading CRC), evaluating
// flag integers so conditional fields are read iff present.
func (m *schemaModel) skipCtorBody(b *bin.Buffer, cl *ctorLayout) error {
	var flags map[string]uint32
	for i := range cl.fields {
		f := &cl.fields[i]
		if f.isFlags {
			v, err := b.Uint32()
			if err != nil {
				return fmt.Errorf("%s.%s: %w", cl.name, f.name, err)
			}
			if flags == nil {
				flags = make(map[string]uint32, 2)
			}
			flags[f.name] = v
			continue
		}
		if f.conditional() && flags[f.flagName]&(1<<uint(f.flagBit)) == 0 {
			continue
		}
		if err := m.skipValue(b, f); err != nil {
			return fmt.Errorf("%s.%s: %w", cl.name, f.name, err)
		}
	}
	return nil
}

// skipValue advances b past one (already known-present) field value.
func (m *schemaModel) skipValue(b *bin.Buffer, f *fieldLayout) error {
	switch f.kind {
	case kindInt:
		_, err := b.Int()
		return err
	case kindLong:
		_, err := b.Long()
		return err
	case kindDouble:
		_, err := b.Double()
		return err
	case kindInt128:
		_, err := b.Int128()
		return err
	case kindInt256:
		_, err := b.Int256()
		return err
	case kindBytes:
		_, err := b.Bytes()
		return err
	case kindString:
		_, err := b.String()
		return err
	case kindBool:
		_, err := b.Bool()
		return err
	case kindTrue:
		return nil
	case kindVector, kindVectorBare:
		if f.kind == kindVector {
			id, err := b.Uint32()
			if err != nil {
				return err
			}
			if id != vectorTypeID {
				return fmt.Errorf("expected vector id, got %#08x", id)
			}
		}
		n, err := b.Int()
		if err != nil {
			return err
		}
		if n < 0 {
			return fmt.Errorf("negative vector length %d", n)
		}
		for i := 0; i < n; i++ {
			if err := m.skipValue(b, f.elem); err != nil {
				return err
			}
		}
		return nil
	case kindObject:
		return m.skipObject(b)
	case kindBareObject:
		cl, ok := m.bareByT[f.typeName]
		if !ok {
			return fmt.Errorf("unknown bare type %q", f.typeName)
		}
		return m.skipCtorBody(b, cl)
	default:
		return fmt.Errorf("bad wire kind %d", f.kind)
	}
}
