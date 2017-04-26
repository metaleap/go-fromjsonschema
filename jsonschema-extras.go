package fromjsd

import (
	"strings"

	"github.com/metaleap/go-util-misc"
	"github.com/metaleap/go-util-str"
)

func (jsd *JsonSchema) generateCtors(buf *ustr.Buffer, baseTypeNames []string, ctorcandidates map[string][]string) {
	for _, btname := range baseTypeNames {
		buf.Writeln("\nfunc Base" + btname + " (some" + btname + " interface{}) (base" + btname + " *" + btname + ") {")
		buf.Writeln("	switch me :" + "= some" + btname + ".(type) {")
		for tname, tdef := range jsd.Defs {
			if tdef.base == btname {
				buf.Writeln("	case *" + tname + ": base" + btname + " = &me." + btname)
			}
		}
		buf.Writeln("	}")
		buf.Writeln("	return")
		buf.Writeln("}")
	}
	for tname, pnames := range ctorcandidates {
		if tdef := jsd.Defs[tname]; tdef != nil {
			if len(pnames) > 0 {
				buf.Writeln("\n// Returns a new `" + tname + "` with the followings fields set: `" + ustr.Join(pnames, "`, `") + "`")
				buf.Writeln("func New" + tname + " () *" + tname + " {")
				buf.Writeln("	new" + tname + " :" + "= " + tname + "{}")
				for _, pname := range pnames {
					if pdef := tdef.Props[pname]; pdef != nil {
						buf.Writeln("	new" + tname + "." + tdef.propNameToFieldName(pname) + " = \"" + pdef.Enum[0] + "\"")
					} else {
						for bdef := jsd.Defs[tdef.base]; bdef != nil; bdef = jsd.Defs[bdef.base] {
							if pdef := bdef.Props[pname]; pdef != nil && len(pdef.Type) == 1 && pdef.Type[0] == "string" && len(pdef.Enum) == 1 {
								buf.Writeln("	new" + tname + "." + tdef.propNameToFieldName(pname) + " = \"" + pdef.Enum[0] + "\"")
							}
						}
					}
				}
				buf.Writeln("	new" + tname + ".propagateFieldsToBase()")
				buf.Writeln("	return &new" + tname)
				buf.Writeln("}")
			}
			// for bname, bdef := tdef.base, jsd.Defs[tdef.base]; bdef != nil; bname, bdef = bdef.base, jsd.Defs[bdef.base] {
			// 	buf.Writeln("func (me *" + tname + ") Base" + bname + " () *" + bname + "{")
			// 	buf.Writeln("	return &me." + bname)
			// 	buf.Writeln("}")
			// }
		}
	}
}

func (jsd *JsonSchema) generateDecodeHelper(buf *ustr.Buffer, forBaseTypeName string, byPropName string, all map[string]string) {
	tdefs := []*JsonDef{}
	pmap := map[string]string{}
	for tname, tdef := range jsd.Defs {
		if tdef.base == forBaseTypeName {
			tdefs = append(tdefs, tdef)
			if pdef, ok := tdef.Props[byPropName]; ok && pdef != nil {
				if len(pdef.Type) != 1 {
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
	}
	buf.Writeln("\n\n// TryUnmarshal" + forBaseTypeName + " attempts to unmarshal JSON string `js` (if it starts with a `{` and ends with a `}`) into a `struct` based on `" + forBaseTypeName + "` as follows:")
	for pval, tname := range pmap {
		buf.Write("// \n// If `js` contains `\"" + byPropName + "\":\"" + pval + "\"`, attempts to unmarshal ")
		if _, ok := all[tname]; ok {
			buf.Writeln("via `TryUnmarshal" + tname + "`")
		} else {
			buf.Writeln("into a new `" + tname + "`.")
		}
	}
	badjfielderrmsg := forBaseTypeName + ": encountered unknown JSON value for " + byPropName + ": "
	buf.Writeln("// \n// Otherwise, `err`'s message will be: `" + badjfielderrmsg + "` followed by the `" + byPropName + "` value encountered.")
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
			buf.Writeln(`	case "` + pval + `":  var val ` + tname + `  ;  if err = json.Unmarshal([]byte(js), &val); err==nil { val.propagateFieldsToBase()  ;  ptr = &val }`)
		}
	}
	buf.Writeln(`	default: err = errors.New("` + badjfielderrmsg + `" + ` + pvalvar + `)`)
	buf.Writeln(`	}`)
	buf.Writeln(`	return`)
	buf.Writeln(`}`)
}

func (me *JsonSchema) generateHandlingScaffold(buf *ustr.Buffer, baseTypeNameIn string, baseTypeNameOut string, ctorcandidates map[string][]string) {
	inouts := map[string]string{}
	for tnameout, tdefout := range me.Defs {
		if tdefout.base == baseTypeNameOut {
			tnamein := tnameout[0:len(tnameout)-len(baseTypeNameOut)] + baseTypeNameIn
			if _, ok := me.Defs[tnamein]; ok {
				inouts[tnamein] = tnameout
			}
		}
	}
	for tni, tno := range inouts {
		buf.Writeln("\n// Called by `Handle" + baseTypeNameIn + "` (after it unmarshaled the given `" + tni + "`) to further populate the given `" + tno + "` before returning it to its caller (in addition to this handler's returned `error`).")
		buf.Writeln("var On" + tni + " func(*" + tni + ", *" + tno + ")error")
	}
	buf.Writeln("\n// If a type-switch on `in" + baseTypeNameIn + "` succeeds, `out" + baseTypeNameOut + "` points to a `" + baseTypeNameOut + "`-based `struct` value containing the `" + baseTypeNameOut + "` initialized by the specified `initNew" + baseTypeNameOut + "` and further populated by the `OnFoo" + baseTypeNameIn + "` handler corresponding to the concrete type of `in" + baseTypeNameIn + "` (if any). The only `err` returned, if any, is that returned by the specialized `OnFoo" + baseTypeNameIn + "` handler.")
	buf.Writeln("func Handle" + baseTypeNameIn + " (in" + baseTypeNameIn + " interface{}, initNew" + baseTypeNameOut + " func(*" + baseTypeNameIn + ", *" + baseTypeNameOut + ")) (out" + baseTypeNameOut + " interface{}, base" + baseTypeNameOut + " *" + baseTypeNameOut + ", err error) {")
	buf.Writeln("	switch input :" + "= in" + baseTypeNameIn + ".(type) {")
	for tni, tno := range inouts {
		_, isptr := ctorcandidates[tno]
		buf.Writeln("	case *" + tni + ":")
		if isptr {
			buf.Writeln("		o :" + "= New" + tno + "()")
		} else {
			buf.Writeln("		o :" + "= &" + tno + "{}")
		}
		buf.Writeln("		if initNew" + baseTypeNameOut + "!=nil { initNew" + baseTypeNameOut + "(&input." + baseTypeNameIn + ", &o." + baseTypeNameOut + ")  ;  o.propagateFieldsToBase() }")
		buf.Writeln("		if On" + tni + "!=nil { err = On" + tni + "(input, o)  ;  o.propagateFieldsToBase() }")
		buf.Writeln("		out" + baseTypeNameOut + " , base" + baseTypeNameOut + " = o , &o." + baseTypeNameOut + "")
	}
	buf.Writeln("	}")
	buf.Writeln("	return")
	buf.Writeln("}")
}

func (me *JsonSchema) ForceCopyProps(fromBaseTypeName string, toBaseTypeName string, pnames ...string) {
	for _, pname := range pnames {
		for tname, tdef := range me.Defs {
			if pdef := tdef.Props[pname]; pdef != nil && tdef.base == fromBaseTypeName {
				tnalt := strings.TrimSuffix(tname, fromBaseTypeName) + toBaseTypeName
				if tdalt := me.Defs[tnalt]; tdalt != nil {
					pcopy := *pdef
					if tdalt.Props != nil {
						tdalt.Props[pname] = &pcopy
					} else {
						tdalt.Props = map[string]*JsonDef{pname: &pcopy}
					}
				}
			}
		}
	}
}
