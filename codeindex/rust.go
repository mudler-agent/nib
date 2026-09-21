package codeindex

import (
	"strings"

	"github.com/msuozzo/bonsai"
	bonsairust "github.com/msuozzo/bonsai/bonsai-rust"
)

type rustExtractor struct{}

func init() {
	register("rust", []string{".rs"}, bonsairust.NewParser, &rustExtractor{})
}

func (e *rustExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, child := range root.Children {
		switch child.Kind {
		case bonsairust.KindUseDeclaration:
			entries = append(entries, Entry{
				Section:   SectionImport,
				Detail:    compactText(child, src),
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		case bonsairust.KindExternCrateDeclaration:
			entries = append(entries, Entry{
				Section:   SectionImport,
				Detail:    compactText(child, src),
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		case bonsairust.KindFunctionItem, bonsairust.KindFunctionSignatureItem:
			entries = append(entries, e.extractFunction(child, src))
		case bonsairust.KindStructItem:
			entries = append(entries, e.extractStruct(child, src))
		case bonsairust.KindEnumItem:
			entries = append(entries, e.extractEnum(child, src))
		case bonsairust.KindUnionItem:
			entries = append(entries, e.extractType(child, src, "union"))
		case bonsairust.KindTraitItem:
			entries = append(entries, e.extractTrait(child, src))
		case bonsairust.KindImplItem:
			entries = append(entries, e.extractImpl(child, src))
		case bonsairust.KindModItem:
			entries = append(entries, Entry{
				Section:   SectionModule,
				Name:      textByField(child, src, bonsairust.FieldName),
				Detail:    textByField(child, src, bonsairust.FieldName),
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		case bonsairust.KindConstItem:
			entries = append(entries, Entry{
				Section:   SectionConst,
				Detail:    strings.TrimSuffix(strings.TrimPrefix(compactText(child, src), "const "), ";"),
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		case bonsairust.KindStaticItem:
			entries = append(entries, Entry{
				Section:   SectionVar,
				Detail:    strings.TrimSuffix(strings.TrimPrefix(compactText(child, src), "static "), ";"),
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		case bonsairust.KindTypeItem:
			entries = append(entries, e.extractType(child, src, "type"))
		case bonsairust.KindMacroDefinition:
			entries = append(entries, Entry{
				Section:   SectionMacro,
				Detail:    compactText(child, src),
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		}
	}
	return entries
}

func (e *rustExtractor) extractFunction(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsairust.FieldName)
	params := textByField(node, src, bonsairust.FieldParameters)
	retType := textByField(node, src, bonsairust.FieldReturnType)
	sig := name + params
	if retType != "" {
		sig += " -> " + retType
	}
	return Entry{
		Section:   SectionFunc,
		Name:      name,
		Detail:    sig,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *rustExtractor) extractStruct(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsairust.FieldName)
	var fields []string
	for fl := range node.Find(bonsairust.KindFieldDeclarationList) {
		for fd := range fl.Find(bonsairust.KindFieldDeclaration) {
			fields = append(fields, compactText(fd, src))
		}
	}
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    name + " struct",
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    fields,
	}
}

func (e *rustExtractor) extractEnum(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsairust.FieldName)
	var variants []string
	for vl := range node.Find(bonsairust.KindEnumVariantList) {
		for v := range vl.Find(bonsairust.KindEnumVariant) {
			variants = append(variants, compactText(v, src))
		}
	}
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    name + " enum",
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    variants,
	}
}

func (e *rustExtractor) extractTrait(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsairust.FieldName)
	var methods []string
	for _, c := range node.Children {
		if c.Kind == bonsairust.KindDeclarationList {
			for _, dc := range c.Children {
				if dc.Kind == bonsairust.KindFunctionSignatureItem {
					mName := textByField(dc, src, bonsairust.FieldName)
					params := textByField(dc, src, bonsairust.FieldParameters)
					retType := textByField(dc, src, bonsairust.FieldReturnType)
					sig := mName + params
					if retType != "" {
						sig += " -> " + retType
					}
					methods = append(methods, sig)
				}
			}
		}
	}
	return Entry{
		Section:   SectionTrait,
		Name:      name,
		Detail:    name,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    methods,
	}
}

func (e *rustExtractor) extractImpl(node *bonsai.Node, src []byte) Entry {
	trait := textByField(node, src, bonsairust.FieldTrait)
	typ := textByField(node, src, bonsairust.FieldType)
	detail := typ
	if trait != "" {
		detail = trait + " for " + typ
	}

	var methods []string
	for _, c := range node.Children {
		if c.Kind == bonsairust.KindDeclarationList {
			for _, dc := range c.Children {
				if dc.Kind == bonsairust.KindFunctionItem {
					mName := textByField(dc, src, bonsairust.FieldName)
					params := textByField(dc, src, bonsairust.FieldParameters)
					retType := textByField(dc, src, bonsairust.FieldReturnType)
					sig := mName + params
					if retType != "" {
						sig += " -> " + retType
					}
					methods = append(methods, sig)
				}
			}
		}
	}
	return Entry{
		Section:   SectionImpl,
		Detail:    detail,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    methods,
	}
}

func (e *rustExtractor) extractType(node *bonsai.Node, src []byte, kind string) Entry {
	name := textByField(node, src, bonsairust.FieldName)
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    strings.TrimSuffix(strings.TrimPrefix(compactText(node, src), "type "), ";"),
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}
