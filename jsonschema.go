package fromjsd

import (
	"encoding/json"

	"github.com/go-leap/str"
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
	for i := ustr.Pos(jsonSchemaDefSrc, "\"type\": \""); i >= 0; i = ustr.Pos(jsonSchemaDefSrc, "\"type\": \"") {
		j := ustr.Pos(jsonSchemaDefSrc[i+9:], "\"")
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
func (this *JsonSchema) Generate(goPkgName string, generateDecodeHelpersForBaseTypeNames map[string]string, generateHandlinScaffoldsForBaseTypes map[string]string, generateCtorsForBaseTypes ...string) string {
	var buf ustr.Buf
	writedesc := func(ind int, desc string) {
		writeDesc(ind, &buf, desc)
	}
	writedesc(0, this.Title+"\n\n"+this.Desc+"\n\n"+GoPkgDesc)
	buf.Writeln("package " + goPkgName)
	if generateDecodeHelpersForBaseTypeNames != nil && len(generateDecodeHelpersForBaseTypeNames) > 0 {
		buf.Writeln("import \"encoding/json\"")
		buf.Writeln("import \"errors\"")
		buf.Writeln("import \"strings\"")
	}
	ctorcandidates := map[string][]string{}
	for tname, tdef := range this.Defs {
		buf.Writeln("\n\n")
		tdef.updateDescBasedOnStrEnumVals()
		writedesc(0, tdef.Desc)
		if tdef.Type[0] == "object" {
			if tdef.Props == nil || len(tdef.Props) == 0 {
				if tdef.TMap != nil {
					buf.Writelnf("type %s map[string]%s", tname, tdef.TMap.genTypeName(0))
				} else {
					buf.Writelnf("type %s %s", tname, TypeMapping["object"])
				}
			} else {
				buf.Writelnf("type %s struct {", tname)
				if len(tdef.base) > 0 {
					writedesc(1, this.Defs[tdef.base].Desc)
					buf.Writelnf("\t%s", tdef.base)
				}
				tdef.genStructFields(1, &buf)
				if ustr.In(tdef.base, generateCtorsForBaseTypes...) {
					for td := tdef; td != nil; td = this.Defs[td.base] {
						for pname, pdef := range td.Props {
							if len(pdef.Type) == 1 && pdef.Type[0] == "string" && len(pdef.Enum) == 1 && !ustr.In(pname, ctorcandidates[tname]...) {
								ctorcandidates[tname] = append(ctorcandidates[tname], pname)
							}
						}
					}
				}
				buf.Writelnf("\n} // struct %s\n", tname)
				if generateDecodeHelpersForBaseTypeNames != nil || generateHandlinScaffoldsForBaseTypes != nil || len(generateCtorsForBaseTypes) > 0 {
					buf.Writeln("func (this *" + tname + ") propagateFieldsToBase() {")
					if bdef, okb := this.Defs[tdef.base]; okb && bdef != nil {
						if bdef.Props != nil {
							for pname := range tdef.Props {
								if _, okp := bdef.Props[pname]; okp {
									buf.Writeln("	this." + tdef.base + "." + bdef.propNameToFieldName(pname) + " = this." + tdef.propNameToFieldName(pname))
								}
							}
						}
						buf.Writeln("	this." + tdef.base + ".propagateFieldsToBase()")
					}
					buf.Writeln("}")
				}
			}
		} else {
			buf.Writelnf("type %s %s", tname, tdef.genTypeName(0))
		}
	}
	this.generateCtors(&buf, generateCtorsForBaseTypes, ctorcandidates)
	if generateDecodeHelpersForBaseTypeNames != nil {
		for gdhfbtn, pname := range generateDecodeHelpersForBaseTypeNames {
			this.generateDecodeHelper(&buf, gdhfbtn, pname, generateDecodeHelpersForBaseTypeNames)
		}
	}
	if generateHandlinScaffoldsForBaseTypes != nil {
		for btnamein, btnameout := range generateHandlinScaffoldsForBaseTypes {
			this.generateHandlingScaffold(&buf, btnamein, btnameout, ctorcandidates)
		}
	}
	return buf.String()
}
