package fromjsd

import (
	"encoding/json"

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

//	Obtains from the given JSON Schema source a `*JsonSchema` that can be passed to `Generate`.
//	`err` is `nil` unless unmarshaling the specified `jsonSchemaDefSrc` failed.
func NewJsonSchema(jsonSchemaDefSrc string) (*JsonSchema, error) {
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

//	Generate a Go package source with type-defs representing the `Defs` in `jsd` (typically obtained via `NewJsonSchema`).
//
//	Arguments beyonds `goPkgName` generate further code beyond the type-defs: these may all be niL/zeroed, or if one "sounds like what you need", check the source for how they're handled otherwise =)
func (jsd *JsonSchema) Generate(goPkgName string, generateDecodeHelpersForBaseTypeNames map[string]string, generateHandlinScaffoldsForBaseTypes map[string]string) string {
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
		def.updateDescBasedOnStrEnumVals()
		writedesc(0, def.Desc)
		if def.Type[0] == "object" {
			buf.Writeln("type %s struct {", tname)
			if len(def.base) > 0 {
				writedesc(1, jsd.Defs[def.base].Desc)
				buf.Writeln("\t%s", def.base)
			}
			if def.Props != nil {
				def.genStructFields(1, &buf)
			}
			buf.Writeln("\n} // struct %s", tname)
		} else {
			buf.Writeln("type %s %s", tname, def.genTypeName(0))
		}
	}
	if generateDecodeHelpersForBaseTypeNames != nil {
		for gdhfbtn, pname := range generateDecodeHelpersForBaseTypeNames {
			jsd.generateDecodeHelper(&buf, gdhfbtn, pname, generateDecodeHelpersForBaseTypeNames)
		}
	}
	if generateHandlinScaffoldsForBaseTypes != nil {
		for btnamein, btnameout := range generateHandlinScaffoldsForBaseTypes {
			jsd.generateHandlingScaffold(&buf, btnamein, btnameout)
		}
	}
	return buf.String()
}
