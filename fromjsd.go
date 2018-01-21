// Generates Go `struct` (et al) type definitions (ready to `json.Unmarshal` into) from a JSON Schema definition.
//
// Caution: contains a few strategically placed `panic`s for edge cases not-yet-considered/implemented/handled/needed. If it `panic`s for your JSON Schema, report!
//
// - Use it like [this main.go](https://github.com/metaleap/zentient/blob/master/cmd/zentient-dbg-vsc-genprotocol/main.go) does..
//
// - ..to turn a JSON Schema [like this](https://github.com/Microsoft/vscode-debugadapter-node/blob/master/debugProtocol.json)..
//
// - ..into a monster `.go` package of `struct` (et al) type-defs [like this](https://github.com/metaleap/zentient/blob/master/dbg/vsc/protocol/protocol.go)
package fromjsd

import (
	"strings"

	"github.com/go-leap/str"
)

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

func unRef(r string) string {
	const l = 14 // len("#/definitions/")
	return r[l:]
}

func tabChars(n int) string {
	return strings.Repeat("\t", n)
}

func writeDesc(ind int, b *ustr.Buf, desc string) {
	tabchars := tabChars(ind)
	if desclns := ustr.Split(strings.TrimSpace(desc), "\n"); len(desclns) > 0 {
		for _, dln := range desclns {
			b.Writelnf("%s// %s", tabchars, dln)
		}
	}
}
