package fromjsd

import (
	"encoding/json"
	"strings"

	"github.com/metaleap/go-util-slice"
	"github.com/metaleap/go-util-str"
)

type JsonDefs struct {
	Schema string              `json:"$schema,omitempty"`
	Title  string              `json:"title,omitempty"`
	Desc   string              `json:"description,omitempty"`
	Type   []string            `json:"type,omitempty"`
	Defs   map[string]*JsonDef `json:"definitions,omitempty"`
}

type JsonDef struct {
	base  string
	Type  []string            `json:"type,omitempty"`                 // top-level defs
	Desc  string              `json:"description,omitempty"`          // tld
	AllOf []*JsonDef          `json:"allOf,omitempty"`                // tld
	Props map[string]*JsonDef `json:"properties,omitempty"`           // tld
	Req   []string            `json:"required,omitempty"`             // tld
	Enum  []string            `json:"enum,omitempty"`                 // tld
	Map   *JsonDef            `json:"additionalProperties,omitempty"` // prop defs
	Items *JsonDef            `json:"items,omitempty"`                // pd
	Enum_ []string            `json:"_enum,omitempty"`                // pd
	Ref   string              `json:"$ref,omitempty"`                 // pd or base from allof[0]
}

var (
	GoPkgDesc   = "Package codegen'd via github.com/metaleap/go-fromjsonschema"
	TypeMapping = map[string]string{
		"boolean": "bool",
		"number":  "int64",
		"integer": "int",
		"string":  "string",
		"null":    "interface{/*nil*/}",
		"array":   "[]interface{}",
		"object":  "map[string]interface{}",
	}
)

func DefsFromJsonSchema(jsonSchemaDefSrc string) (*JsonDefs, error) {
	for i := ustr.Idx(jsonSchemaDefSrc, "\"type\": \""); i >= 0; i = ustr.Idx(jsonSchemaDefSrc, "\"type\": \"") {
		j := ustr.Idx(jsonSchemaDefSrc[i+9:], "\"")
		tname := jsonSchemaDefSrc[i+9:][:j]
		jsonSchemaDefSrc = jsonSchemaDefSrc[:i] + "\"type\": [\"" + tname + "\"]" + jsonSchemaDefSrc[i+9+j+1:]
	}
	var jdefs JsonDefs
	if err := json.Unmarshal([]byte(jsonSchemaDefSrc), &jdefs); err != nil {
		return nil, err
	}
	return &jdefs, nil
}

func Generate(goPkgName string, jdefs *JsonDefs) string {
	topleveldefs := map[string]*JsonDef{}
	for tname, jdef := range jdefs.Defs {
		if len(jdef.AllOf) == 0 {
			topleveldefs[tname] = jdef
		} else if len(jdef.AllOf) == 2 {
			jdef.AllOf[1].base = unRef(jdef.AllOf[0].Ref)
			jdef = jdef.AllOf[1]
		} else {
			panic(tname)
		}
		if len(jdef.Type) != 1 {
			panic(tname)
		}
		topleveldefs[tname] = jdef
	}

	var buf ustr.Buffer
	writedesc := func(ind int, desc string) {
		writeDesc(ind, &buf, desc)
	}
	writedesc(0, jdefs.Title+"\n\n"+jdefs.Desc+"\n\n"+GoPkgDesc)
	buf.Writeln("package " + goPkgName)
	for tname, def := range topleveldefs {
		buf.Writeln("\n\n")
		strEnumVals(def)
		writedesc(0, def.Desc)
		switch def.Type[0] {
		case "string":
			buf.Writeln("type %s string ", tname)
			continue
		case "object":
			buf.Writeln("type %s struct {", tname)
			if len(def.base) > 0 {
				writedesc(1, topleveldefs[def.base].Desc)
				buf.Writeln("\t%s", def.base)
			}
			structFields(1, &buf, def)
			buf.Writeln("} // struct %s", tname)
		default:
			panic(def.Type[0])
		}
	}
	return buf.String()
}

func unRef(r string) string {
	return r[len("#/definitions/"):]
}

func strEnumVals(d *JsonDef) {
	if len(d.Type) > 0 && d.Type[0] == "string" {
		en := d.Enum_
		if len(en) == 0 {
			en = d.Enum
		}
		if len(en) > 0 {
			d.Desc += "\n\nPOSSIBLE VALUES: `" + ustr.Join(en, "`, `") + "`"
		}
	}
}

func structFields(ind int, b *ustr.Buffer, def *JsonDef) {
	tabchars := tabChars(ind)
	for fname, fdef := range def.Props {
		ftname := typeName(ind, fdef)
		gtname := strings.Title(fname)
		if gtname == def.base {
			gtname += "_"
		}
		b.Writeln("")
		strEnumVals(fdef)
		writeDesc(ind, b, fdef.Desc)
		omitempty := ",omitempty"
		if uslice.StrHas(def.Req, fname) {
			omitempty = ""
		}
		b.Writeln("%s%s %s `json:\"%s%s\"`", tabchars, gtname, ftname, fname, omitempty)
	}
}

func tabChars(n int) string {
	return strings.Repeat("\t", n)
}

func typeName(ind int, d *JsonDef) (ftname string) {
	ftname = "interface{}"
	if d != nil {
		if len(d.Ref) > 0 {
			ftname = unRef(d.Ref)
		} else if len(d.Type) > 1 {
			d.Desc += "\n\nPOSSIBLE TYPES: `" + ustr.Join(uslice.StrMap(d.Type, func(jtn string) string { return TypeMapping[jtn] }), "`, `") + "`"
		} else if len(d.Type) > 0 {
			switch d.Type[0] {
			case "object":
				if d.Map != nil {
					ftname = "map[string]" + typeName(ind, d.Map)
				} else if len(d.Props) > 0 {
					var b ustr.Buffer
					structFields(ind+1, &b, d)
					ftname = "struct{\n" + b.String() + "\n" + tabChars(ind) + "}"
				} else {
					panic(d.Desc)
				}
			case "array":
				ftname = "[]" + typeName(ind, d.Items)
			default:
				if tn, ok := TypeMapping[d.Type[0]]; ok {
					ftname = tn
				} else {
					panic(d.Type[0])
				}
			}
		}
	}
	return
}

func writeDesc(ind int, b *ustr.Buffer, desc string) {
	tabchars := tabChars(ind)
	if desclns := ustr.Split(ustr.Trim(desc), "\n"); len(desclns) > 0 {
		for _, dln := range desclns {
			b.Writeln("%s// %s", tabchars, dln)
		}
	}
}
