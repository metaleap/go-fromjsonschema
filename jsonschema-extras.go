package fromjsd

import (
	"github.com/metaleap/go-util-misc"
	"github.com/metaleap/go-util-str"
)

func (jsd *JsonSchema) generateDecodeHelper(buf *ustr.Buffer, forBaseTypeName string, byPropName string, all map[string]string) {
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

func (me *JsonSchema) generateHandlingScaffold(buf *ustr.Buffer, baseTypeNameIn string, baseTypeNameOut string) {
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
		buf.Writeln("\n// Called by `Handle" + baseTypeNameIn + "` when unmarshaling `" + tni + "` to populate the given `" + tno + "` that it'll then return a pointer to (or discard and return `nil` in case this handler returns an `error`).")
		buf.Writeln("var On" + tni + " func(*" + tni + ", *" + tno + ")error")
	}
	buf.Writeln("\n// If a type-switch on `in" + baseTypeNameIn + "` succeeds, `out" + baseTypeNameOut + "` points to a `" + baseTypeNameOut + "`-based `struct` value containing the `" + baseTypeNameOut + "` returned by the specified `makeNew" + baseTypeNameOut + "` constructor and further populated by the `OnFoo" + baseTypeNameIn + "` handler corresponding to the concrete type of `in" + baseTypeNameIn + "` (if any). The only `err` returned, if any, is that returned by the specialized `OnFoo" + baseTypeNameIn + "` handler.")
	buf.Writeln("func Handle" + baseTypeNameIn + " (in" + baseTypeNameIn + " interface{}, makeNew" + baseTypeNameOut + " func(*" + baseTypeNameIn + ")" + baseTypeNameOut + ") (out" + baseTypeNameOut + " interface{}, base" + baseTypeNameOut + " *" + baseTypeNameOut + ", err error) {")
	buf.Writeln("	switch input :" + "= in" + baseTypeNameIn + ".(type) {")
	for tni, tno := range inouts {
		buf.Writeln("	case *" + tni + ": output :" + "= " + tno + "{ " + baseTypeNameOut + ": makeNew" + baseTypeNameOut + "(&input." + baseTypeNameIn + ") }; base" + baseTypeNameOut + " = &output." + baseTypeNameOut + "; if On" + tni + "!=nil { err = On" + tni + "(input, &output) }; out" + baseTypeNameOut + " = &output")
	}
	buf.Writeln("	}")
	buf.Writeln("	return")
	buf.Writeln("}")
}