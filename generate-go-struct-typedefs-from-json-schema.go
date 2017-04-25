//	Generates Go `struct` (et al) type definitions (ready to `json.Unmarshal` into) from a JSON Schema definition.
//
//	Caution: contains a few strategically placed `panic`s for edge cases not-yet-considered/implemented/handled/needed. If it `panic`s for your JSON Schema, report!
//
//	- Use it like [this main.go](https://github.com/metaleap/zentient/blob/master/zdbg-vsc-proto-gen/main.go) does..
//	- ..to turn a JSON Schema [like this](https://github.com/Microsoft/vscode-debugadapter-node/blob/master/debugProtocol.json)..
//	- ..into a monster `.go` package of `struct` type-defs [like this](https://github.com/metaleap/zentient/blob/master/zdbg-vsc/proto/proto.go)
package fromjsd

import (
	"encoding/json"
	"strings"

	"github.com/metaleap/go-util-misc"
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
//
//	`generateDecodeHelpersForBaseTypeNames` may be `nil`, or if it sounds like what you need, contain "typename":"fieldname" pairs after having grasped from source how it's used =)
func Generate(goPkgName string, jsd *JsonSchema, generateDecodeHelpersForBaseTypeNames map[string]string) string {
	var buf ustr.Buffer
	writedesc := func(ind int, desc string) {
		writeDesc(ind, &buf, desc)
	}
	writedesc(0, jsd.Title+"\n\n"+jsd.Desc+"\n\n"+GoPkgDesc)
	buf.Writeln("package " + goPkgName)
	if generateDecodeHelpersForBaseTypeNames != nil && len(generateDecodeHelpersForBaseTypeNames) > 0 {
		buf.Writeln("import \"encoding/json\"")
		buf.Writeln("import \"errors\"")
		buf.Writeln("import \"strings\"")
	}
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
			if def.Props != nil {
				structFields(1, &buf, def)
			}
			buf.Writeln("\n} // struct %s", tname)
		} else {
			buf.Writeln("type %s %s", tname, typeName(0, def))
		}
	}
	if generateDecodeHelpersForBaseTypeNames != nil {
		for gdhfbtn, pname := range generateDecodeHelpersForBaseTypeNames {
			generateDecodeHelper(jsd, &buf, gdhfbtn, pname, generateDecodeHelpersForBaseTypeNames)
		}
	}
	return buf.String()
}

func generateDecodeHelper(jsd *JsonSchema, buf *ustr.Buffer, forBaseTypeName string, byPropName string, all map[string]string) {
	tdefs := []*JsonDef{}
	pmap := map[string]string{}
	for tname, tdef := range jsd.Defs {
		if tdef.base == forBaseTypeName {
			tdefs = append(tdefs, tdef)
			if pdef, ok := tdef.Props[byPropName]; pdef == nil || !ok {
				panic(tname + ".missing:" + byPropName)
			} else if len(pdef.Type) != 1 {
				panic(tname + "." + byPropName + " has types: " + ugo.SPr(len(pdef.Type)))
			} else if pdef.Type[0] != "string" {
				panic(tname + "." + byPropName + " is " + pdef.Type[0])
			} else if len(pdef.Enum) != 1 {
				panic(tname + "." + byPropName + " has " + ugo.SPr(len(pdef.Enum)))
			} else if ustr.Has(pdef.Enum[0], "\"") {
				panic(tname + "." + byPropName + " has a quote-mark in: " + pdef.Enum[0])
			} else if _, exists := pmap[pdef.Enum[0]]; exists {
				panic(tname + "." + byPropName + " would overwrite existing: " + pdef.Enum[0])
			} else {
				pmap[pdef.Enum[0]] = tname
			}
		}
	}
	buf.Writeln("\n\n// TryUnmarshal" + forBaseTypeName + " attempts to unmarshal JSON string `js` (if it starts with a `{` and ends with a `}`) into a `struct` based on `" + forBaseTypeName + "` as follows:")
	buf.Writeln("// ")
	for pval, tname := range pmap {
		buf.Write("// If `js` contains `\"" + byPropName + "\":\"" + pval + "\"`, attempts to unmarshal ")
		if _, ok := all[tname]; ok {
			buf.Writeln("via `TryUnmarshal" + tname + "`")
		} else {
			buf.Writeln("into a new `" + tname + "`.")
		}
	}
	badjfielderrmsg := forBaseTypeName + ": encountered unknown JSON value for " + byPropName + ": "
	buf.Writeln("// Otherwise, `err`'s message will be: `" + badjfielderrmsg + "` followed by the `" + byPropName + "` value encountered.")
	buf.Writeln("// \n// In general: the `err` returned may be either `nil`, the above message, or an `encoding/json.Unmarshal()` return value.")
	buf.Writeln("// `ptr` will be a pointer to the unmarshaled `struct` value if that succeeded, else `nil`.")
	buf.Writeln("// Both `err` and `ptr` will be `nil` if `js` doesn't: start with `{` and end with `}` and contain `\"" + byPropName + "\":\"` followed by a subsequent `\"`.")
	buf.Writeln(`func TryUnmarshal` + forBaseTypeName + ` (js string) (ptr interface{}, err error) {`)
	buf.Writeln(`	if len(js)==0 || js[0]!='{' || js[len(js)-1]!='}' { return }`)
	//	it's only due to buggy syntax-highlighting that all generated := below are all split out into :` + `=
	buf.Writeln(`	i1 :` + `= strings.Index(js, "\"` + byPropName + `\":\"")  ;  if i1<1 { return }`)
	buf.Writeln(`	subjs :` + `= js[i1+4+` + ugo.SPr(len(byPropName)) + `:]`)
	buf.Writeln(`	i2 :` + `= strings.Index(subjs, "\"")  ;  if i2<1 { return }`)
	pvalvar := byPropName + `_of_` + forBaseTypeName
	buf.Writeln(`	` + pvalvar + ` :` + `= subjs[:i2]  ;  switch ` + pvalvar + ` {`)
	for pval, tname := range pmap {
		if _, ok := all[tname]; ok {
			buf.Writeln(`	case "` + pval + `":  ptr,err = TryUnmarshal` + tname + `(js)`)
		} else {
			buf.Writeln(`	case "` + pval + `":  var val ` + tname + `  ;  if err = json.Unmarshal([]byte(js), &val); err==nil { ptr = &val }`)
		}
	}
	buf.Writeln(`	default: err = errors.New("` + badjfielderrmsg + `" + ` + pvalvar + `)`)
	buf.Writeln(`	}`)
	buf.Writeln(`	return`)
	buf.Writeln(`}`)
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
				} else if d.Props != nil && len(d.Props) > 0 {
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
