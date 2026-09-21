package codeindex

import (
	"strings"

	"github.com/msuozzo/bonsai"
	bonsaijava "github.com/msuozzo/bonsai/bonsai-java"
)

type javaExtractor struct{}

func init() {
	register("java", []string{".java"}, bonsaijava.NewParser, &javaExtractor{})
}

func (e *javaExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, child := range root.Children {
		switch child.Kind {
		case bonsaijava.KindPackageDeclaration:
			name := strings.TrimSuffix(strings.TrimPrefix(compactText(child, src), "package "), ";")
			entries = append(entries, Entry{
				Section:   SectionPackage,
				Name:      name,
				Detail:    name,
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		case bonsaijava.KindImportDeclaration:
			entries = append(entries, Entry{
				Section:   SectionImport,
				Detail:    compactText(child, src),
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		case bonsaijava.KindClassDeclaration:
			entries = append(entries, e.extractClass(child, src, "class"))
		case bonsaijava.KindInterfaceDeclaration:
			entries = append(entries, e.extractInterface(child, src))
		case bonsaijava.KindEnumDeclaration:
			entries = append(entries, e.extractEnum(child, src))
		case bonsaijava.KindRecordDeclaration:
			entries = append(entries, e.extractRecord(child, src))
		case bonsaijava.KindAnnotationTypeDeclaration:
			name := textByField(child, src, bonsaijava.FieldName)
			entries = append(entries, Entry{
				Section:   SectionType,
				Name:      name,
				Detail:    name,
				StartLine: int(child.StartPoint.Row) + 1,
				EndLine:   int(child.EndPoint.Row) + 1,
			})
		}
	}
	return entries
}

func (e *javaExtractor) extractClass(node *bonsai.Node, src []byte, kind string) Entry {
	name := textByField(node, src, bonsaijava.FieldName)
	detail := name
	if sc := node.ChildByField(bonsaijava.FieldSuperclass); sc != nil {
		detail += " extends " + compactText(sc, src)
	}
	if ifs := node.ChildByField(bonsaijava.FieldInterfaces); ifs != nil {
		detail += " implements " + compactText(ifs, src)
	}
	return Entry{
		Section:   SectionClass,
		Name:      name,
		Detail:    detail,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    e.extractBodyMembers(node, src),
	}
}

func (e *javaExtractor) extractInterface(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaijava.FieldName)
	return Entry{
		Section:   SectionTrait,
		Name:      name,
		Detail:    name,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    e.extractBodyMembers(node, src),
	}
}

func (e *javaExtractor) extractEnum(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaijava.FieldName)
	var constants []string
	for body := range node.Find(bonsaijava.KindEnumBody) {
		for ec := range body.Find(bonsaijava.KindEnumConstant) {
			constants = append(constants, compactText(ec, src))
		}
	}
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    name + " enum",
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    constants,
	}
}

func (e *javaExtractor) extractRecord(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaijava.FieldName)
	params := textByField(node, src, bonsaijava.FieldParameters)
	return Entry{
		Section:   SectionClass,
		Name:      name,
		Detail:    name + params,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *javaExtractor) extractBodyMembers(node *bonsai.Node, src []byte) []string {
	var members []string
	for _, c := range node.Children {
		if c.Kind == bonsaijava.KindClassBody || c.Kind == bonsaijava.KindInterfaceBody {
			for _, bc := range c.Children {
				switch bc.Kind {
				case bonsaijava.KindMethodDeclaration:
					name := textByField(bc, src, bonsaijava.FieldName)
					params := textByField(bc, src, bonsaijava.FieldParameters)
					retType := textByField(bc, src, bonsaijava.FieldType)
					sig := ""
					if retType != "" {
						sig += retType + " "
					}
					sig += name + params
					members = append(members, sig)
				case bonsaijava.KindConstructorDeclaration:
					name := textByField(bc, src, bonsaijava.FieldName)
					params := textByField(bc, src, bonsaijava.FieldParameters)
					members = append(members, name+params)
				case bonsaijava.KindFieldDeclaration:
					members = append(members, strings.TrimSuffix(compactText(bc, src), ";"))
				}
			}
		}
	}
	return members
}
