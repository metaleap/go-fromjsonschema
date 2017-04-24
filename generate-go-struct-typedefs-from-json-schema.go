//	Generates Go `struct` type definitions (ready to json.Unmarshal into) from a JSON Schema definition.
//
//	Caution: contains a few strategically placed `panic`s for edge cases not-yet-considered/implemented/handled/needed. If it `panic`s for your JSON Schema, report!
//
//	- Use it like [this main.go](https://github.com/metaleap/zentient/blob/master/dbg/zentient-debug-protocol-gen/main.go) does..
//	- ..to turn a JSON Schema [like this](https://github.com/Microsoft/vscode-debugadapter-node/blob/master/debugProtocol.json)..
//	- ..into a monster `.go` package of `struct` type-defs [like this](https://github.com/metaleap/zentient/blob/master/dbg/proto/proto.go)
package fromjsd

import (
	"encoding/json"
	"strings"

	"github.com/metaleap/go-util-slice"
	"github.com/metaleap/go-util-str"
)

//	Top-level declarations of a JSON Schema
type JsonSchema struct {
	//	Something like `http://json-schema.org/draft-04/schema#`
	Schema string `json:"$schema,omitempty"`

	Title string `json:"title,omitempty"`

	Desc string `json:"description,omitempty"`

	//	Ignored, assuming `["object"]` (prior to unmarshaling a JSON Schhema, we transform all `\"type\": \"foo\"` into \"type\": [\"foo\"] for now)
	Type []string `json:"type,omitempty"`

	//	The JSON Schema's type definitions
	Defs map[string]*JsonDef `json:"definitions,omitempty"`
}

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

var (
	//	Will be appended to the resulting generated package doc-comment summary
	GoPkgDesc = "Package codegen'd via github.com/metaleap/go-fromjsonschema"

	//	Default JSON-to-Go type mappings, `number` could be tweaked to `float64` depending on the use-case at hand
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

//	Obtains from the given JSON Schema source a `*JsonSchema` that can be passed to `Generate`.
//	`err` is `nil` unless unmarshaling the specified `jsonSchemaDefSrc` failed.
func DefsFromJsonSchema(jsonSchemaDefSrc string) (*JsonSchema, error) {
	for i := ustr.Idx(jsonSchemaDefSrc, "\"type\": \""); i >= 0; i = ustr.Idx(jsonSchemaDefSrc, "\"type\": \"") {
		j := ustr.Idx(jsonSchemaDefSrc[i+9:], "\"")
		tname := jsonSchemaDefSrc[i+9:][:j]
		jsonSchemaDefSrc = jsonSchemaDefSrc[:i] + "\"type\": [\"" + tname + "\"]" + jsonSchemaDefSrc[i+9+j+1:]
	}
	var jdefs JsonSchema
	if err := json.Unmarshal([]byte(jsonSchemaDefSrc), &jdefs); err != nil {
		return nil, err
	}
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
	jdefs.Defs = topleveldefs
	return &jdefs, nil
}

//	Generate a Go package source with type-defs representing the `Defs` in the specified `jsd` (typically obtained via `DefsFromJsonSchema`).
func Generate(goPkgName string, jsd *JsonSchema) string {
	var buf ustr.Buffer
	writedesc := func(ind int, desc string) {
		writeDesc(ind, &buf, desc)
	}
	writedesc(0, jsd.Title+"\n\n"+jsd.Desc+"\n\n"+GoPkgDesc)
	buf.Writeln("package " + goPkgName)
	for tname, def := range jsd.Defs {
		buf.Writeln("\n\n")
		strEnumVals(def)
		writedesc(0, def.Desc)
		if def.Type[0] == "object" {
			buf.Writeln("type %s struct {", tname)
			if len(def.base) > 0 {
				writedesc(1, jsd.Defs[def.base].Desc)
				buf.Writeln("\t%s", def.base)
			}
			structFields(1, &buf, def)
			buf.Writeln("\n} // struct %s", tname)
		} else {
			buf.Writeln("type %s %s", tname, typeName(0, def))
		}
	}
	return buf.String()
}

func unRef(r string) string {
	const l = 14 // len("#/definitions/")
	return r[l:]
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
		if len(fdef.AllOf) > 0 {
			panic(fname)
		}
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
			d.Desc += "\n\nPOSSIBLE TYPES:"
			for _, jtn := range d.Type {
				d.Desc += "\n- `" + TypeMapping[jtn] + "` (if from JSON `" + jtn + "`)"
			}
		} else if len(d.Type) > 0 {
			switch d.Type[0] {
			case "object":
				if d.TMap != nil {
					ftname = "map[string]" + typeName(ind, d.TMap)
				} else if len(d.Props) > 0 {
					var b ustr.Buffer
					structFields(ind+1, &b, d)
					ftname = "struct {\n" + b.String() + "\n" + tabChars(ind) + "}"
				} else {
					panic(d.Desc)
				}
			case "array":
				ftname = "[]" + typeName(ind, d.TArr)
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
