package solana

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/idl/idltype"
)

//go:embed sourcecre.ts.tpl
var tsSolanaTpl string

//go:embed mockcontract.ts.tpl
var tsSolanaMockTpl string

var (
	tsSolanaTemplate     = template.Must(template.New("sourcecre.ts").Parse(tsSolanaTpl))
	tsSolanaMockTemplate = template.Must(template.New("mockcontract.ts").Parse(tsSolanaMockTpl))
)

// jsReservedWords are identifiers that cannot be used as TypeScript/JavaScript
// names. Field names that collide get an underscore suffix; type/class names
// that collide are rejected (they come from IDL type names, which are expected
// to be PascalCase and never collide in practice).
var jsReservedWords = map[string]bool{
	"break": true, "case": true, "catch": true, "class": true, "const": true,
	"continue": true, "debugger": true, "default": true, "delete": true,
	"do": true, "else": true, "enum": true, "export": true, "extends": true,
	"false": true, "finally": true, "for": true, "function": true, "if": true,
	"import": true, "in": true, "instanceof": true, "new": true, "null": true,
	"return": true, "super": true, "switch": true, "this": true, "throw": true,
	"true": true, "try": true, "typeof": true, "var": true, "void": true,
	"while": true, "with": true, "yield": true, "let": true, "static": true,
	"implements": true, "interface": true, "package": true, "private": true,
	"protected": true, "public": true, "await": true, "arguments": true,
	"eval": true,
}

type tsField struct {
	Name      string // camelCase TS property name
	TSType    string
	CodecExpr string
}

type tsEnumVariant struct {
	Name string
}

type tsTypeDef struct {
	Name       string // PascalCase TS type name
	CodecConst string // e.g. userDataCodec
	IsEnum     bool
	Fields     []tsField
	Variants   []tsEnumVariant
}

type tsDecoderDef struct {
	Name          string // PascalCase name from the IDL accounts/events section
	TypeName      string // TS type to decode into
	CodecConst    string
	ConstName     string // discriminator const, e.g. ACCOUNT_DATA_ACCOUNT_DISCRIMINATOR
	Discriminator string // JS array body, e.g. "85, 240, 182, ..."
}

type tsBindingData struct {
	ProgramName    string
	ClassName      string
	MockName       string
	ProgramIDConst string
	IdlConst       string
	ProgramID      string
	IdlJSON        string
	CodecImports   []string
	UsesAddress    bool
	Types          []tsTypeDef
	StructTypes    []tsTypeDef
	Accounts       []tsDecoderDef
	Events         []tsDecoderDef
}

// GenerateBindingsTS generates the TypeScript CRE binding (<Class>.ts) and its
// mock (<Class>_mock.ts) for one Anchor IDL file. It mirrors the CRE-reachable
// surface of the Go generator: per-struct writeReportFrom<Struct>(s) write
// methods plus pure account/event decoders. Native instruction builders and
// account fetchers are intentionally not generated — they are unreachable
// through the write-only Solana CRE capability.
func GenerateBindingsTS(
	pathToIdl string,
	programName string,
	outDir string,
) (className string, err error) {
	if pathToIdl == "" {
		return "", fmt.Errorf("pathToIdl is empty")
	}
	if programName == "" {
		return "", fmt.Errorf("programName is empty")
	}
	if outDir == "" {
		return "", fmt.Errorf("outDir is empty")
	}

	parsedIdl, err := idl.ParseFromFilepath(pathToIdl)
	if err != nil {
		return "", fmt.Errorf("failed to parse IDL: %w", err)
	}
	if parsedIdl == nil {
		return "", fmt.Errorf("parsedIdl is nil")
	}
	if err := parsedIdl.Validate(); err != nil {
		return "", fmt.Errorf("invalid IDL: %w", err)
	}
	rawIdl, err := os.ReadFile(pathToIdl) //nolint:gosec // G703 -- path from trusted CLI flags
	if err != nil {
		return "", fmt.Errorf("read IDL %q: %w", pathToIdl, err)
	}
	var compactIdl bytes.Buffer
	if err := json.Compact(&compactIdl, rawIdl); err != nil {
		return "", fmt.Errorf("compact IDL JSON %q: %w", pathToIdl, err)
	}

	data, err := buildTSBindingData(parsedIdl, programName, compactIdl.String())
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := renderTSTemplate(tsSolanaTemplate, data, filepath.Join(outDir, data.ClassName+".ts")); err != nil {
		return "", err
	}
	if err := renderTSTemplate(tsSolanaMockTemplate, data, filepath.Join(outDir, data.ClassName+"_mock.ts")); err != nil {
		return "", err
	}

	return data.ClassName, nil
}

func renderTSTemplate(t *template.Template, data *tsBindingData, outPath string) error {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("render %q: %w", outPath, err)
	}
	if err := os.WriteFile(outPath, buf.Bytes(), 0o600); err != nil { //nolint:gosec // G703 -- derived from trusted CLI path
		return fmt.Errorf("write %q: %w", outPath, err)
	}
	return nil
}

func buildTSBindingData(parsedIdl *idl.Idl, programName, idlJSON string) (*tsBindingData, error) {
	mapper := &tsTypeMapper{
		defs:    map[string]*idl.IdlTypeDef{},
		imports: map[string]bool{},
	}
	for i := range parsedIdl.Types {
		def := &parsedIdl.Types[i]
		if _, exists := mapper.defs[def.Name]; exists {
			return nil, fmt.Errorf("duplicate type %q in IDL", def.Name)
		}
		mapper.defs[def.Name] = def
	}

	ordered, err := topoSortTypes(parsedIdl.Types)
	if err != nil {
		return nil, err
	}

	className := toPascalCase(programName)
	if jsReservedWords[className] || !isValidJSIdentifier(className) {
		return nil, fmt.Errorf("program name %q maps to invalid TypeScript class name %q", programName, className)
	}

	programID := ""
	if parsedIdl.Address != nil && !parsedIdl.Address.IsZero() {
		programID = parsedIdl.Address.String()
	}
	data := &tsBindingData{
		ProgramName:    programName,
		ClassName:      className,
		MockName:       className + "Mock",
		ProgramIDConst: toUpperSnake(programName) + "_PROGRAM_ID",
		IdlConst:       toUpperSnake(programName) + "_IDL",
		ProgramID:      programID,
		IdlJSON:        idlJSON,
	}

	typeNames := map[string]string{} // TS name -> IDL name (collision detection)
	codecNames := map[string]bool{}
	for _, def := range ordered {
		tsName := toPascalCase(def.Name)
		if prev, exists := typeNames[tsName]; exists {
			return nil, fmt.Errorf("type name collision: IDL types %q and %q both map to TypeScript name %q", prev, def.Name, tsName)
		}
		typeNames[tsName] = def.Name

		tsDef, err := mapper.mapTypeDef(def, tsName)
		if err != nil {
			return nil, err
		}
		if codecNames[tsDef.CodecConst] {
			return nil, fmt.Errorf("codec name collision: %q generated twice", tsDef.CodecConst)
		}
		codecNames[tsDef.CodecConst] = true

		data.Types = append(data.Types, *tsDef)
		if !tsDef.IsEnum {
			data.StructTypes = append(data.StructTypes, *tsDef)
		}
	}

	// Per-struct write method names must not collide (e.g. types Foo and Foos
	// would both produce writeReportFromFoos).
	writeMethods := map[string]string{}
	for _, st := range data.StructTypes {
		for _, method := range []string{"writeReportFrom" + st.Name, "writeReportFrom" + st.Name + "s"} {
			if prev, exists := writeMethods[method]; exists {
				return nil, fmt.Errorf("write method name collision: types %q and %q both produce %q", prev, st.Name, method)
			}
			writeMethods[method] = st.Name
		}
	}

	typesByName := make(map[string]tsTypeDef, len(data.Types))
	for _, td := range data.Types {
		typesByName[td.Name] = td
	}

	for _, account := range parsedIdl.Accounts {
		decoder, err := buildDecoderDef("account", account.Name, account.Discriminator[:], typesByName)
		if err != nil {
			return nil, err
		}
		data.Accounts = append(data.Accounts, *decoder)
	}
	for _, event := range parsedIdl.Events {
		decoder, err := buildDecoderDef("event", event.Name, event.Discriminator[:], typesByName)
		if err != nil {
			return nil, err
		}
		data.Events = append(data.Events, *decoder)
	}

	data.CodecImports = mapper.sortedImports()
	data.UsesAddress = mapper.usesAddress

	return data, nil
}

func buildDecoderDef(kind, name string, discriminator []byte, typesByName map[string]tsTypeDef) (*tsDecoderDef, error) {
	tsName := toPascalCase(name)
	def, exists := typesByName[tsName]
	if !exists {
		return nil, fmt.Errorf("%s %q has no matching type definition in the IDL types section", kind, name)
	}
	if def.IsEnum {
		return nil, fmt.Errorf("%s %q refers to enum type %q; only struct %ss are supported", kind, name, tsName, kind)
	}
	parts := make([]string, len(discriminator))
	for i, b := range discriminator {
		parts[i] = fmt.Sprintf("%d", b)
	}
	return &tsDecoderDef{
		Name:          tsName,
		TypeName:      tsName,
		CodecConst:    def.CodecConst,
		ConstName:     strings.ToUpper(kind) + "_" + toUpperSnake(name) + "_DISCRIMINATOR",
		Discriminator: strings.Join(parts, ", "),
	}, nil
}

// topoSortTypes orders type definitions so that every `defined` reference
// points to an already-emitted codec const. Cyclic type references cannot be
// expressed with plain codec consts and fail loudly.
func topoSortTypes(types idl.IdTypeDef_slice) ([]*idl.IdlTypeDef, error) {
	defsByName := make(map[string]*idl.IdlTypeDef, len(types))
	deps := map[string][]string{}
	for i := range types {
		def := &types[i]
		defsByName[def.Name] = def
		refs, err := definedRefsOfTypeDef(def)
		if err != nil {
			return nil, err
		}
		deps[def.Name] = refs
	}

	var ordered []*idl.IdlTypeDef
	state := map[string]int{} // 0 unvisited, 1 visiting, 2 done
	var visit func(name string, path []string) error
	visit = func(name string, path []string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("cyclic type reference in IDL: %s", strings.Join(append(path, name), " -> "))
		case 2:
			return nil
		}
		state[name] = 1
		for _, dep := range deps[name] {
			if _, known := deps[dep]; !known {
				return fmt.Errorf("type %q references undefined type %q", name, dep)
			}
			if err := visit(dep, append(path, name)); err != nil {
				return err
			}
		}
		state[name] = 2
		ordered = append(ordered, defsByName[name])
		return nil
	}

	for i := range types {
		if err := visit(types[i].Name, nil); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func definedRefsOfTypeDef(def *idl.IdlTypeDef) ([]string, error) {
	var refs []string
	var walk func(t idltype.IdlType)
	walk = func(t idltype.IdlType) {
		switch typed := t.(type) {
		case *idltype.Option:
			walk(typed.Option)
		case *idltype.COption:
			walk(typed.COption)
		case *idltype.Vec:
			walk(typed.Vec)
		case *idltype.Array:
			walk(typed.Type)
		case *idltype.Defined:
			refs = append(refs, typed.Name)
		}
	}

	switch ty := def.Ty.(type) {
	case *idl.IdlTypeDefTyStruct:
		switch fields := ty.Fields.(type) {
		case nil:
			// empty struct
		case idl.IdlDefinedFieldsNamed:
			for _, field := range fields {
				walk(field.Ty)
			}
		default:
			return nil, fmt.Errorf("type %q: tuple struct fields are not supported by the TypeScript generator yet", def.Name)
		}
	case *idl.IdlTypeDefTyEnum:
		if !ty.IsAllSimple() {
			return nil, fmt.Errorf("type %q: data-carrying enums are not supported by the TypeScript generator yet (only scalar enums)", def.Name)
		}
	default:
		return nil, fmt.Errorf("type %q: unsupported type definition kind %T", def.Name, def.Ty)
	}
	return refs, nil
}

type tsTypeMapper struct {
	defs        map[string]*idl.IdlTypeDef
	imports     map[string]bool
	usesAddress bool
}

func (m *tsTypeMapper) sortedImports() []string {
	out := make([]string, 0, len(m.imports))
	for name := range m.imports {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (m *tsTypeMapper) mapTypeDef(def *idl.IdlTypeDef, tsName string) (*tsTypeDef, error) {
	codecConst := toCamelCase(def.Name) + "Codec"

	switch ty := def.Ty.(type) {
	case *idl.IdlTypeDefTyEnum:
		// definedRefsOfTypeDef already rejected data-carrying enums.
		m.imports["getEnumCodec"] = true
		out := &tsTypeDef{Name: tsName, CodecConst: codecConst, IsEnum: true}
		seen := map[string]string{}
		for _, variant := range ty.Variants {
			variantName := toPascalCase(variant.Name)
			if prev, exists := seen[variantName]; exists {
				return nil, fmt.Errorf("type %q: enum variants %q and %q both map to %q", def.Name, prev, variant.Name, variantName)
			}
			seen[variantName] = variant.Name
			out.Variants = append(out.Variants, tsEnumVariant{Name: variantName})
		}
		return out, nil

	case *idl.IdlTypeDefTyStruct:
		m.imports["getStructCodec"] = true
		out := &tsTypeDef{Name: tsName, CodecConst: codecConst}
		var named idl.IdlDefinedFieldsNamed
		if ty.Fields != nil {
			var ok bool
			named, ok = ty.Fields.(idl.IdlDefinedFieldsNamed)
			if !ok {
				return nil, fmt.Errorf("type %q: tuple struct fields are not supported by the TypeScript generator yet", def.Name)
			}
		}
		seen := map[string]string{}
		for _, field := range named {
			fieldName := toCamelCase(field.Name)
			if jsReservedWords[fieldName] {
				fieldName += "_"
			}
			if !isValidJSIdentifier(fieldName) {
				return nil, fmt.Errorf("type %q: field %q maps to invalid TypeScript identifier %q", def.Name, field.Name, fieldName)
			}
			if prev, exists := seen[fieldName]; exists {
				return nil, fmt.Errorf("type %q: fields %q and %q both map to %q", def.Name, prev, field.Name, fieldName)
			}
			seen[fieldName] = field.Name

			tsType, codecExpr, err := m.mapType(field.Ty, fmt.Sprintf("%s.%s", def.Name, field.Name))
			if err != nil {
				return nil, err
			}
			out.Fields = append(out.Fields, tsField{Name: fieldName, TSType: tsType, CodecExpr: codecExpr})
		}
		return out, nil

	default:
		return nil, fmt.Errorf("type %q: unsupported type definition kind %T", def.Name, def.Ty)
	}
}

// mapType maps an Anchor IDL type to its TypeScript type and the
// @solana/codecs expression that encodes/decodes it (the v1 coverage matrix).
// Unsupported types fail loudly instead of silently mis-encoding.
func (m *tsTypeMapper) mapType(t idltype.IdlType, owner string) (tsType, codecExpr string, err error) {
	simple := func(ts, codecFn string) (string, string, error) {
		m.imports[codecFn] = true
		return ts, codecFn + "()", nil
	}

	switch typed := t.(type) {
	case *idltype.Bool:
		return simple("boolean", "getBooleanCodec")
	case *idltype.U8:
		return simple("number", "getU8Codec")
	case *idltype.I8:
		return simple("number", "getI8Codec")
	case *idltype.U16:
		return simple("number", "getU16Codec")
	case *idltype.I16:
		return simple("number", "getI16Codec")
	case *idltype.U32:
		return simple("number", "getU32Codec")
	case *idltype.I32:
		return simple("number", "getI32Codec")
	case *idltype.U64:
		return simple("bigint", "getU64Codec")
	case *idltype.I64:
		return simple("bigint", "getI64Codec")
	case *idltype.U128:
		return simple("bigint", "getU128Codec")
	case *idltype.I128:
		return simple("bigint", "getI128Codec")
	case *idltype.F32:
		return simple("number", "getF32Codec")
	case *idltype.F64:
		return simple("number", "getF64Codec")
	case *idltype.U256, *idltype.I256:
		return "", "", fmt.Errorf("%s: %s is not supported by the TypeScript generator yet (no @solana/codecs codec; needs a hand-written 32-byte little-endian codec)", owner, t.String())
	case *idltype.String:
		m.imports["addCodecSizePrefix"] = true
		m.imports["getUtf8Codec"] = true
		m.imports["getU32Codec"] = true
		return "string", "addCodecSizePrefix(getUtf8Codec(), getU32Codec())", nil
	case *idltype.Bytes:
		m.imports["addCodecSizePrefix"] = true
		m.imports["getBytesCodec"] = true
		m.imports["getU32Codec"] = true
		return "Uint8Array", "addCodecSizePrefix(getBytesCodec(), getU32Codec())", nil
	case *idltype.Pubkey:
		m.usesAddress = true
		return "Address", "getAddressCodec()", nil
	case *idltype.Option:
		innerTS, innerCodec, err := m.mapType(typed.Option, owner)
		if err != nil {
			return "", "", err
		}
		m.imports["getNullableCodec"] = true
		return fmt.Sprintf("%s | null", innerTS), fmt.Sprintf("getNullableCodec(%s)", innerCodec), nil
	case *idltype.COption:
		return "", "", fmt.Errorf("%s: COption is not supported by the TypeScript generator yet", owner)
	case *idltype.Vec:
		innerTS, innerCodec, err := m.mapType(typed.Vec, owner)
		if err != nil {
			return "", "", err
		}
		m.imports["getArrayCodec"] = true
		m.imports["getU32Codec"] = true
		return arrayTSType(innerTS), fmt.Sprintf("getArrayCodec(%s, { size: getU32Codec() })", innerCodec), nil
	case *idltype.Array:
		size, ok := typed.Size.(*idltype.IdlArrayLenValue)
		if !ok {
			return "", "", fmt.Errorf("%s: generic array lengths are not supported by the TypeScript generator yet", owner)
		}
		innerTS, innerCodec, err := m.mapType(typed.Type, owner)
		if err != nil {
			return "", "", err
		}
		m.imports["getArrayCodec"] = true
		return arrayTSType(innerTS), fmt.Sprintf("getArrayCodec(%s, { size: %d })", innerCodec, size.Value), nil
	case *idltype.Defined:
		if len(typed.Generics) > 0 {
			return "", "", fmt.Errorf("%s: generic type %q is not supported by the TypeScript generator yet", owner, typed.Name)
		}
		def, exists := m.defs[typed.Name]
		if !exists {
			return "", "", fmt.Errorf("%s: references undefined type %q", owner, typed.Name)
		}
		return toPascalCase(def.Name), toCamelCase(def.Name) + "Codec", nil
	case *idltype.Generic:
		return "", "", fmt.Errorf("%s: generic types are not supported by the TypeScript generator yet", owner)
	default:
		return "", "", fmt.Errorf("%s: unsupported IDL type %T", owner, t)
	}
}

// arrayTSType wraps union element types in parentheses so `T | null` becomes
// `(T | null)[]` instead of the wrong `T | null[]`.
func arrayTSType(inner string) string {
	if strings.Contains(inner, "|") {
		return "(" + inner + ")[]"
	}
	return inner + "[]"
}

func splitNameWords(name string) []string {
	var words []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			words = append(words, string(current))
			current = nil
		}
	}
	runes := []rune(name)
	for i, r := range runes {
		switch {
		case r == '_' || r == '-' || r == ' ':
			flush()
		case unicode.IsUpper(r):
			// Split on lower→Upper and on the last upper of an acronym
			// followed by lower (e.g. HTTPServer -> HTTP, Server).
			if i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])) {
				flush()
			} else if i > 0 && unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				flush()
			}
			current = append(current, r)
		default:
			current = append(current, r)
		}
	}
	flush()
	return words
}

func toPascalCase(name string) string {
	var b strings.Builder
	for _, word := range splitNameWords(name) {
		runes := []rune(strings.ToLower(word))
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

func toCamelCase(name string) string {
	pascal := toPascalCase(name)
	if pascal == "" {
		return ""
	}
	runes := []rune(pascal)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func toUpperSnake(name string) string {
	words := splitNameWords(name)
	for i, word := range words {
		words[i] = strings.ToUpper(word)
	}
	return strings.Join(words, "_")
}

func isValidJSIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' && r != '$' {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$' {
			return false
		}
	}
	return true
}
