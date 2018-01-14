package fromjsd

import (
	"encoding/json"

	"github.com/metaleap/go-util/slice"
	"github.com/metaleap/go-util/str"
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
//	Arguments beyond `goPkgName` generate further code beyond the type-defs: these may all be `nil`/zeroed, or if one "sounds like what you need", check the source for how they're handled otherwise =)
func (jsd *JsonSchema) Generate(goPkgName string, generateDecodeHelpersForBaseTypeNames map[string]string, generateHandlinScaffoldsForBaseTypes map[string]string, generateCtorsForBaseTypes ...string) string {
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
	ctorcandidates := map[string][]string{}
	for tname, tdef := range jsd.Defs {
		buf.Writeln("\n\n")
		tdef.updateDescBasedOnStrEnumVals()
		writedesc(0, tdef.Desc)
		if tdef.Type[0] == "object" {
			if tdef.Props == nil || len(tdef.Props) == 0 {
				if tdef.TMap != nil {
					buf.Writeln("type %s map[string]%s", tname, tdef.TMap.genTypeName(0))
				} else {
					buf.Writeln("type %s %s", tname, TypeMapping["object"])
				}
			} else {
				buf.Writeln("type %s struct {", tname)
				if len(tdef.base) > 0 {
					writedesc(1, jsd.Defs[tdef.base].Desc)
					buf.Writeln("\t%s", tdef.base)
				}
				tdef.genStructFields(1, &buf)
				if uslice.StrHas(generateCtorsForBaseTypes, tdef.base) {
					for td := tdef; td != nil; td = jsd.Defs[td.base] {
						for pname, pdef := range td.Props {
							if len(pdef.Type) == 1 && pdef.Type[0] == "string" && len(pdef.Enum) == 1 && !uslice.StrHas(ctorcandidates[tname], pname) {
								ctorcandidates[tname] = append(ctorcandidates[tname], pname)
							}
						}
					}
				}
				buf.Writeln("\n} // struct %s\n", tname)
				if generateDecodeHelpersForBaseTypeNames != nil || generateHandlinScaffoldsForBaseTypes != nil || len(generateCtorsForBaseTypes) > 0 {
					buf.Writeln("func (me *" + tname + ") propagateFieldsToBase() {")
					if bdef, ok := jsd.Defs[tdef.base]; ok && bdef != nil {
						if bdef.Props != nil {
							for pname := range tdef.Props {
								if _, ok := bdef.Props[pname]; ok {
									buf.Writeln("	me." + tdef.base + "." + bdef.propNameToFieldName(pname) + " = me." + tdef.propNameToFieldName(pname))
								}
							}
						}
						buf.Writeln("	me." + tdef.base + ".propagateFieldsToBase()")
					}
					buf.Writeln("}")
				}
			}
		} else {
			buf.Writeln("type %s %s", tname, tdef.genTypeName(0))
		}
	}
	jsd.generateCtors(&buf, generateCtorsForBaseTypes, ctorcandidates)
	if generateDecodeHelpersForBaseTypeNames != nil {
		for gdhfbtn, pname := range generateDecodeHelpersForBaseTypeNames {
			jsd.generateDecodeHelper(&buf, gdhfbtn, pname, generateDecodeHelpersForBaseTypeNames)
		}
	}
	if generateHandlinScaffoldsForBaseTypes != nil {
		for btnamein, btnameout := range generateHandlinScaffoldsForBaseTypes {
			jsd.generateHandlingScaffold(&buf, btnamein, btnameout, ctorcandidates)
		}
	}
	return buf.String()
}
