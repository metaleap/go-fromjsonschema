package fromjsd

import (
	"github.com/go-leap/str"
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

func (this *JsonDef) EnsureProps(propNamesAndTypes map[string]string) {
	if this.Props == nil {
		// this.Props = map[string]*JsonDef{}
		panic("EnsureProps: Props was nil and so this likely isn't supposed to have any")
	}
	for pname, ptype := range propNamesAndTypes {
		if pdef, ok := this.Props[pname]; pdef == nil || !ok {
			pdef = &JsonDef{Type: []string{ptype}, Desc: pname}
			this.Props[pname] = pdef
		} else {
			panic("EnsureProps: property `" + pname + "` exists in the original schema now, remove it from this call.")
		}
	}
}

func (this *JsonDef) genStructFields(ind int, b *ustr.Buf) {
	tabchars := tabChars(ind)
	for pname, pdef := range this.Props {
		if len(pdef.AllOf) > 0 {
			panic(pname)
		}
		ftname := pdef.genTypeName(ind)
		gtname := this.propNameToFieldName(pname)
		b.Writeln("")
		pdef.updateDescBasedOnStrEnumVals()
		writeDesc(ind, b, pdef.Desc)
		omitempty := ",omitempty"
		if ustr.In(pname, this.Req...) {
			omitempty = ""
		}
		b.Writelnf("%s%s %s `json:\"%s%s\"`", tabchars, gtname, ftname, pname, omitempty)
	}
}

func (this *JsonDef) genTypeName(ind int) (ftname string) {
	ftname = "interface{}"
	if this != nil {
		if len(this.Ref) > 0 {
			ftname = unRef(this.Ref)
		} else if len(this.Type) > 1 {
			this.Desc += "\n\nPOSSIBLE TYPES:"
			for _, jtn := range this.Type {
				this.Desc += "\n- `" + TypeMapping[jtn] + "` (for JSON `" + jtn + "`s)"
			}
		} else if len(this.Type) > 0 {
			switch this.Type[0] {
			case "object":
				if this.Props != nil && len(this.Props) > 0 {
					var b ustr.Buf
					this.genStructFields(ind+1, &b)
					ftname = "struct {\n" + b.String() + "\n" + tabChars(ind) + "}"
				} else if this.TMap != nil {
					ftname = "map[string]" + this.TMap.genTypeName(ind)
				} else {
					ftname = TypeMapping["object"]
				}
			case "array":
				ftname = "[]" + this.TArr.genTypeName(ind)
			default:
				if tn, ok := TypeMapping[this.Type[0]]; ok {
					ftname = tn
				} else {
					panic(this.Type[0])
				}
			}
		}
	}
	return
}

func (this *JsonDef) propNameToFieldName(pname string) (fname string) {
	for ustr.Pref(pname, "_") {
		pname = pname[1:] + "_"
	}
	if fname = ustr.Case(pname, 0, true); fname == this.base {
		fname += "_"
	}
	return
}

func (this *JsonDef) updateDescBasedOnStrEnumVals() {
	if len(this.Type) > 0 && this.Type[0] == "string" {
		en := this.Enum_
		if len(en) == 0 {
			en = this.Enum
		}
		if len(en) > 0 {
			this.Desc += "\n\nPOSSIBLE VALUES: `" + ustr.Join(en, "`, `") + "`"
		}
	}
}
