# fromjsd

    import "github.com/metaleap/go-fromjsonschema"

Generates Go `struct` (et al) type definitions (ready to `json.Unmarshal` into) from a
JSON Schema definition.

Caution: contains a few strategically placed `panic`s for edge cases
not-yet-considered/implemented/handled/needed. If it `panic`s for your JSON
Schema, report!

- Use it like [this main.go](https://github.com/metaleap/zentient/blob/master/zdbg-vsc-proto-gen/main.go) does..
- ..to turn a JSON Schema [like this](https://github.com/Microsoft/vscode-debugadapter-node/blob/master/debugProtocol.json)..
- ..into a monster `.go` package of `struct` type-defs [like this](https://github.com/metaleap/zentient/blob/master/zdbg-vsc/proto/proto.go)

## Usage

```go
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
```

#### func  Generate

```go
func Generate(goPkgName string, jsd *JsonSchema, generateDecodeHelpersForBaseTypeNames map[string]string) string
```
Generate a Go package source with type-defs representing the `Defs` in the
specified `jsd` (typically obtained via `DefsFromJsonSchema`).

`generateDecodeHelpersForBaseTypeNames` may be `nil` --- or if it "sounds like what you need", may contain "typename":"fieldname" pairs after having grasped from source how it's used =)
#### type JsonDef

```go
type JsonDef struct {
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
```

Represents a top-level type definition, or a property definition, a
type-reference, an embedded anonymous `struct`/`object` type definition, or an
`array`/`map` element type definition..

#### type JsonSchema

```go
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
```

Top-level declarations of a JSON Schema

#### func  DefsFromJsonSchema

```go
func DefsFromJsonSchema(jsonSchemaDefSrc string) (*JsonSchema, error)
```
Obtains from the given JSON Schema source a `*JsonSchema` that can be passed to
`Generate`. `err` is `nil` unless unmarshaling the specified `jsonSchemaDefSrc`
failed.
