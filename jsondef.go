package fromjsd

import (
	"strings"

	"github.com/metaleap/go-util-slice"
	"github.com/metaleap/go-util-str"
)

//	Represents a top-level type definition, or a property definition, a type-reference, an embedded anonymous `struct`/`object` type definition, or an `array`/`map` element type definition..
type JsonDef struct {
	base  string
	Desc  string              `json:"description,omitempty"`          // top-level defs
	AllOf []*JsonDef          `json:"allOf,omitempty"`                // tld
	Props map[string]*JsonDef `json:"properties,omitempty"`           // tld
	Type  []string            `json:"type,omitempty"`                 // tld
	Req   []string            `json:"required,omitempty"`             // tld
	Enum  []string            `json:"enum,omitempty"`                 // tld
	Enum_ []string            `json:"_enum,omitempty"`                // prop defs
	TMap  *JsonDef            `json:"additionalProperties,omitempty"` // pd
	TArr  *JsonDef            `json:"items,omitempty"`                // pd
	Ref   string              `json:"$ref,omitempty"`                 // pd or base from allof[0]
}

func (me *JsonDef) EnsureProps(propNamesAndTypes map[string]string) {
	if me.Props == nil {
		// me.Props = map[string]*JsonDef{}
		panic("EnsureProps: Props was nil and so this likely isn't supposed to have any")
	}
	for pname, ptype := range propNamesAndTypes {
		if pdef, ok := me.Props[pname]; pdef == nil || !ok {
			pdef = &JsonDef{Type: []string{ptype}, Desc: pname}
			me.Props[pname] = pdef
		} else {
			panic("EnsureProps: property `" + pname + "` exists in the original schema now, remove it from this call.")
		}
	}
}

func (me *JsonDef) genStructFields(ind int, b *ustr.Buffer) {
	tabchars := tabChars(ind)
	for fname, fdef := range me.Props {
		if len(fdef.AllOf) > 0 {
			panic(fname)
		}
		ftname := fdef.genTypeName(ind)
		gtname := strings.Title(fname)
		if gtname == me.base {
			gtname += "_"
		}
		b.Writeln("")
		fdef.updateDescBasedOnStrEnumVals()
		writeDesc(ind, b, fdef.Desc)
		omitempty := ",omitempty"
		if uslice.StrHas(me.Req, fname) {
			omitempty = ""
		}
		b.Writeln("%s%s %s `json:\"%s%s\"`", tabchars, gtname, ftname, fname, omitempty)
	}
}

func (me *JsonDef) genTypeName(ind int) (ftname string) {
	ftname = "interface{}"
	if me != nil {
		if len(me.Ref) > 0 {
			ftname = unRef(me.Ref)
		} else if len(me.Type) > 1 {
			me.Desc += "\n\nPOSSIBLE TYPES:"
			for _, jtn := range me.Type {
				me.Desc += "\n- `" + TypeMapping[jtn] + "` (if from JSON `" + jtn + "`)"
			}
		} else if len(me.Type) > 0 {
			switch me.Type[0] {
			case "object":
				if me.TMap != nil {
					ftname = "map[string]" + me.TMap.genTypeName(ind)
				} else if me.Props != nil && len(me.Props) > 0 {
					var b ustr.Buffer
					me.genStructFields(ind+1, &b)
					ftname = "struct {\n" + b.String() + "\n" + tabChars(ind) + "}"
				} else {
					panic(me.Desc)
				}
			case "array":
				ftname = "[]" + me.TArr.genTypeName(ind)
			default:
				if tn, ok := TypeMapping[me.Type[0]]; ok {
					ftname = tn
				} else {
					panic(me.Type[0])
				}
			}
		}
	}
	return
}

func (me *JsonDef) updateDescBasedOnStrEnumVals() {
	if len(me.Type) > 0 && me.Type[0] == "string" {
		en := me.Enum_
		if len(en) == 0 {
			en = me.Enum
		}
		if len(en) > 0 {
			me.Desc += "\n\nPOSSIBLE VALUES: `" + ustr.Join(en, "`, `") + "`"
		}
	}
}