package codeindex

import (
	"strings"

	"github.com/msuozzo/bonsai"
	bonsaijs "github.com/msuozzo/bonsai/bonsai-javascript"
	bonsaits "github.com/msuozzo/bonsai/bonsai-tsx"         // parser for .tsx files
	bonsaitsx "github.com/msuozzo/bonsai/bonsai-typescript" // kind/field constants (shared with tsx)
)

// ---- JavaScript ----

type jsExtractor struct{}

func init() {
	register("javascript", []string{".js", ".jsx", ".mjs", ".cjs"}, bonsaijs.NewParser, &jsExtractor{})
}

func (e *jsExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	return extractJSLike(root, src, jsKinds{})
}

// ---- TypeScript ----

type tsExtractor struct{}

func init() {
	register("typescript", []string{".ts"}, bonsaitsx.NewParser, &tsExtractor{})
}

func (e *tsExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	return extractTSLike(root, src)
}

// ---- TSX ----

type tsxExtractor struct{}

func init() {
	register("tsx", []string{".tsx"}, bonsaits.NewParser, &tsxExtractor{})
}

func (e *tsxExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	return extractTSLike(root, src)
}

// jsKinds holds the JavaScript-specific kind constants.
type jsKinds struct{}

func (jsKinds) importStmt() string  { return bonsaijs.KindImportStatement }
func (jsKinds) exportStmt() string  { return bonsaijs.KindExportStatement }
func (jsKinds) classDecl() string   { return bonsaijs.KindClassDeclaration }
func (jsKinds) funcDecl() string    { return bonsaijs.KindFunctionDeclaration }
func (jsKinds) genFuncDecl() string { return bonsaijs.KindGeneratorFunctionDeclaration }
func (jsKinds) lexicalDecl() string { return bonsaijs.KindLexicalDeclaration }
func (jsKinds) varDecl() string     { return bonsaijs.KindVariableDeclaration }
func (jsKinds) classBody() string   { return bonsaijs.KindClassBody }
func (jsKinds) methodDef() string   { return bonsaijs.KindMethodDefinition }
func (jsKinds) fieldDef() string    { return bonsaijs.KindFieldDefinition }
func (jsKinds) name() string        { return bonsaijs.FieldName }
func (jsKinds) params() string      { return bonsaijs.FieldParameters }
func (jsKinds) returnType() string  { return "" } // JS has no return types

func extractJSLike(root *bonsai.Node, src []byte, k jsKinds) []Entry {
	var entries []Entry
	for _, child := range root.Children {
		switch child.Kind {
		case k.importStmt():
			entries = append(entries, Entry{
				Section:   SectionImport,
				Detail:    compactText(child, src),
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		case k.exportStmt():
			// export may wrap a function/class/const declaration
			entries = append(entries, extractExport(child, src, k)...)
		case k.classDecl():
			entries = append(entries, extractJSClass(child, src, k))
		case k.funcDecl(), k.genFuncDecl():
			entries = append(entries, extractJSFunc(child, src, k))
		case k.lexicalDecl():
			entries = append(entries, extractJSConst(child, src, k)...)
		}
	}
	return entries
}

func extractExport(node *bonsai.Node, src []byte, k jsKinds) []Entry {
	var entries []Entry
	for _, c := range node.Children {
		switch c.Kind {
		case k.classDecl():
			entries = append(entries, extractJSClass(c, src, k))
		case k.funcDecl(), k.genFuncDecl():
			entries = append(entries, extractJSFunc(c, src, k))
		case k.lexicalDecl():
			entries = append(entries, extractJSConst(c, src, k)...)
		}
	}
	return entries
}

func extractJSClass(node *bonsai.Node, src []byte, k jsKinds) Entry {
	name := textByField(node, src, k.name())
	var members []string
	if body := node.ChildByField("body"); body != nil {
		for _, c := range body.Children {
			switch c.Kind {
			case k.methodDef():
				mName := textByField(c, src, k.name())
				params := textByField(c, src, k.params())
				members = append(members, mName+params)
			case k.fieldDef():
				members = append(members, compactText(c, src))
			}
		}
	}
	return Entry{
		Section:   SectionClass,
		Name:      name,
		Detail:    name,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    members,
	}
}

func extractJSFunc(node *bonsai.Node, src []byte, k jsKinds) Entry {
	name := textByField(node, src, k.name())
	params := textByField(node, src, k.params())
	return Entry{
		Section:   SectionFunc,
		Name:      name,
		Detail:    name + params,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func extractJSConst(node *bonsai.Node, src []byte, k jsKinds) []Entry {
	var entries []Entry
	for _, c := range node.Children {
		if c.Kind == bonsaijs.KindVariableDeclarator {
			name := textByField(c, src, k.name())
			entries = append(entries, Entry{
				Section:   SectionConst,
				Name:      name,
				Detail:    compactText(c, src),
				StartLine: int(node.StartPoint.Row) + 1,
				EndLine:   int(node.EndPoint.Row) + 1,
			})
		}
	}
	return entries
}

// ---- TypeScript / TSX ----

func extractTSLike(root *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, child := range root.Children {
		switch child.Kind {
		case bonsaitsx.KindImportStatement:
			entries = append(entries, Entry{
				Section:   SectionImport,
				Detail:    compactText(child, src),
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		case bonsaitsx.KindExportStatement:
			entries = append(entries, extractTSExport(child, src)...)
		case bonsaitsx.KindClassDeclaration, bonsaitsx.KindAbstractClassDeclaration:
			entries = append(entries, extractTSClass(child, src))
		case bonsaitsx.KindFunctionDeclaration, bonsaitsx.KindGeneratorFunctionDeclaration:
			entries = append(entries, extractTSFunc(child, src))
		case bonsaitsx.KindInterfaceDeclaration:
			entries = append(entries, extractTSInterface(child, src))
		case bonsaitsx.KindTypeAliasDeclaration:
			entries = append(entries, extractTSTypeAlias(child, src))
		case bonsaitsx.KindEnumDeclaration:
			entries = append(entries, extractTSEnum(child, src))
		case bonsaitsx.KindLexicalDeclaration:
			entries = append(entries, extractTSConst(child, src)...)
		}
	}
	return entries
}

func extractTSExport(node *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, c := range node.Children {
		switch c.Kind {
		case bonsaitsx.KindClassDeclaration, bonsaitsx.KindAbstractClassDeclaration:
			entries = append(entries, extractTSClass(c, src))
		case bonsaitsx.KindFunctionDeclaration, bonsaitsx.KindGeneratorFunctionDeclaration:
			entries = append(entries, extractTSFunc(c, src))
		case bonsaitsx.KindInterfaceDeclaration:
			entries = append(entries, extractTSInterface(c, src))
		case bonsaitsx.KindTypeAliasDeclaration:
			entries = append(entries, extractTSTypeAlias(c, src))
		case bonsaitsx.KindEnumDeclaration:
			entries = append(entries, extractTSEnum(c, src))
		case bonsaitsx.KindLexicalDeclaration:
			entries = append(entries, extractTSConst(c, src)...)
		}
	}
	return entries
}

func extractTSClass(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaitsx.FieldName)
	var members []string
	if body := node.ChildByField("body"); body != nil {
		for _, c := range body.Children {
			switch c.Kind {
			case bonsaitsx.KindMethodDefinition:
				mName := textByField(c, src, bonsaitsx.FieldName)
				params := textByField(c, src, bonsaitsx.FieldParameters)
				retType := textByField(c, src, bonsaitsx.FieldReturnType)
				sig := mName + params
				if retType != "" {
					sig += retType
				}
				members = append(members, sig)
			case bonsaitsx.KindPublicFieldDefinition:
				members = append(members, compactText(c, src))
			}
		}
	}
	prefix := ""
	if node.Kind == bonsaitsx.KindAbstractClassDeclaration {
		prefix = "abstract "
	}
	return Entry{
		Section:   SectionClass,
		Name:      name,
		Detail:    prefix + name,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    members,
	}
}

func extractTSFunc(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaitsx.FieldName)
	params := textByField(node, src, bonsaitsx.FieldParameters)
	retType := textByField(node, src, bonsaitsx.FieldReturnType)
	sig := name + params
	if retType != "" {
		sig += retType
	}
	return Entry{
		Section:   SectionFunc,
		Name:      name,
		Detail:    sig,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func extractTSInterface(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaitsx.FieldName)
	var members []string
	if body := node.ChildByField("body"); body != nil {
		for _, c := range body.Children {
			switch c.Kind {
			case bonsaitsx.KindMethodSignature:
				mName := textByField(c, src, bonsaitsx.FieldName)
				params := textByField(c, src, bonsaitsx.FieldParameters)
				retType := textByField(c, src, bonsaitsx.FieldReturnType)
				sig := mName + params
				if retType != "" {
					sig += retType
				}
				members = append(members, sig)
			case bonsaitsx.KindPropertySignature:
				members = append(members, compactText(c, src))
			}
		}
	}
	return Entry{
		Section:   SectionTrait,
		Name:      name,
		Detail:    name,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    members,
	}
}

func extractTSTypeAlias(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaitsx.FieldName)
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    strings.TrimSuffix(strings.TrimPrefix(compactText(node, src), "type "), ";"),
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func extractTSEnum(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaitsx.FieldName)
	var members []string
	if body := node.ChildByField("body"); body != nil {
		for _, c := range body.Children {
			if c.Kind == bonsaitsx.KindEnumAssignment {
				members = append(members, compactText(c, src))
			}
		}
	}
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    name + " enum",
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    members,
	}
}

func extractTSConst(node *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, c := range node.Children {
		if c.Kind == bonsaitsx.KindVariableDeclarator {
			name := textByField(c, src, bonsaitsx.FieldName)
			entries = append(entries, Entry{
				Section:   SectionConst,
				Name:      name,
				Detail:    compactText(c, src),
				StartLine: int(node.StartPoint.Row) + 1,
				EndLine:   int(node.EndPoint.Row) + 1,
			})
		}
	}
	return entries
}
