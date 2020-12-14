package gen

import (
	"bytes"
	"errors"
	"fmt"
	"go/types"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

var ErrMultiplePackages = errors.New("Multiple packages found")
var ErrCompile = errors.New("Compile errors")

func Generate(outPath, funcname, dotTyName string, node *html.Node) error {
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName}, ".")
	if err != nil {
		return err
	}
	if len(pkgs) != 1 {
		return ErrMultiplePackages
	}

	stub := bytes.Buffer{}
	fmt.Fprintf(&stub, "package %s\n", pkgs[0].Name)
	stub.WriteString(`import "golang.org/x/net/html"`)
	fmt.Fprintf(&stub, "\nfunc %s(dot %s) (out []*html.Node) {return}\n", funcname, dotTyName)
	istub, err := imports.Process(outPath, stub.Bytes(), nil)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(outPath)
	if err != nil {
		return err
	}
	conf := packages.Config{
		Mode:    packages.NeedName | packages.NeedImports | packages.NeedTypes,
		Overlay: map[string][]byte{absPath: istub},
	}
	pkgs, err = packages.Load(&conf, ".")
	if err != nil {
		return err
	}
	if len(pkgs) != 1 {
		return ErrMultiplePackages
	}
	if packages.PrintErrors(pkgs) > 0 {
		return ErrCompile
	}

	scope := pkgs[0].Types.Scope()
	stubFunc := scope.Lookup(funcname)
	if stubFunc == nil {
		panic("Could not locate stub function")
	}
	funcTy, ok := stubFunc.Type().(*types.Signature)
	if !ok {
		panic("Stub function is not a function")
	}
	dotTy := funcTy.Params().At(0).Type().Underlying()

	gen := &generator{types: map[string][]types.Type{
		".": []types.Type{dotTy},
	}}
	gen.Printf("package %s\n", pkgs[0].Name)
	gen.WriteString(`import (
	"fmt"

	"github.com/vktec/htmpl/gen"
	"golang.org/x/net/html"
)
`)
	gen.Printf("func %s(dot %s) (out []*html.Node) {\n", funcname, dotTyName)
	if err := gen.genCode(node); err != nil {
		return err
	}
	gen.WriteString("return\n}\n")
	code, err := imports.Process(outPath, gen.Bytes(), nil)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(outPath, code, 0666)
}

type generator struct {
	bytes.Buffer
	types map[string][]types.Type
}

func (gen *generator) Printf(format string, args ...interface{}) {
	fmt.Fprintf(gen, format, args...)
}

func (gen *generator) genCode(node *html.Node) error {
	switch node.Type {
	case html.DocumentNode:
		if err := gen.genChildren(node); err != nil {
			return err
		}

	case html.ElementNode:
		switch node.Data {
		case "if", "nif":
			gen.WriteString("if ")
			if node.Data == "nif" {
				gen.WriteString("!(")
			}
			gen.genTruthy(getAttr(node, "v"))
			if node.Data == "nif" {
				gen.WriteByte(')')
			}
			gen.WriteString(" {\n")
			if err := gen.genChildren(node); err != nil {
				return err
			}
			gen.WriteString("}\n")

		case "for":
			elemTy := gen.genLoop(getAttr(node, "v"))
			if elemTy != nil {
				gen.pushTy(".", elemTy)
				if err := gen.genChildren(node); err != nil {
					return err
				}
				gen.popTy(".")
				gen.WriteString("}\n")
			}

		case "let":
			valName, valTy := gen.get(getAttr(node, "val"))
			varName := gen.name(getAttr(node, "var"))
			gen.WriteString("if true {\n")
			gen.Printf("%s := %s\n_ = %[0]s\n", varName, valName)
			gen.pushTy(varName, valTy)
			gen.genChildren(node)
			gen.popTy(varName)
			gen.WriteString("}\n")

		case "v":
			// FIXME: should handle multiple child nodes
			if node.FirstChild != nil && node.FirstChild.Type == html.TextNode {
				path := node.FirstChild.Data
				gen.WriteString("out = append(out, &html.Node{Type: html.TextNode, Data: ")
				gen.genStringify(path)
				gen.WriteString("})\n")
			}

		default:
			if err := gen.genElement(node); err != nil {
				return err
			}
		}

	default:
		gen.Printf("out = append(out, &html.Node{Type: %d, Data: %q})\n", node.Type, node.Data)
	}
	return nil
}

func (gen *generator) genChildren(node *html.Node) error {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if err := gen.genCode(child); err != nil {
			return err
		}
	}
	return nil
}

func (gen *generator) genTruthy(name string) {
	name, ty := gen.get(name)
	if ty == nil {
		gen.WriteString("false")
		return
	}
	switch ty := ty.(type) {
	case *types.Array, *types.Slice:
		gen.Printf("len(%s) > 0", name)
	case *types.Basic:
		info := ty.Info()
		if info&types.IsBoolean != 0 {
			gen.WriteString(name)
		} else if info&types.IsNumeric != 0 {
			gen.Printf("%s != 0", name)
		} else if info&types.IsString != 0 {
			gen.Printf(`%s != ""`, name)
		}
		panic("Unknown basic type " + ty.String())
	case *types.Chan:
		fmt.Printf("%s != nil", name)
	case *types.Map, *types.Struct:
		fmt.Printf("true")
	default:
		panic("Unknown type " + ty.String())
	}
}

func (gen *generator) genLoop(name string) (elemTy types.Type) {
	name, ty := gen.get(name)
	if ty == nil {
		return nil
	}
	switch ty := ty.(type) {
	case *types.Array:
		gen.Printf("for _, dot := range %s {_=dot\n", name)
		return ty.Elem()
	case *types.Slice:
		gen.Printf("for _, dot := range %s {_=dot\n", name)
		return ty.Elem()
	case *types.Basic:
		gen.Printf("if true {\ndot := %s\n_=dot\n", name)
		return ty
	case *types.Chan:
		gen.Printf("for dot := range %s {_=dot\n", name)
		return ty.Elem()
	case *types.Map:
		gen.Printf("for dot := range %s {_=dot\n", name)
		return ty.Key()
	case *types.Struct:
		gen.WriteString("for _, dot := range []string{")
		for i := 0; i < ty.NumFields(); i++ {
			// TODO: handle embedded fields
			gen.Printf("%q,", ty.Field(i).Name())
		}
		gen.WriteString("} {_=dot\n")
		return types.Typ[types.String]
	default:
		panic("Unknown type " + ty.String())
	}
}

func (gen *generator) genStringify(name string) {
	name, ty := gen.get(name)
	if ty == nil {
		gen.WriteString(`""`)
		return
	}
	switch ty := ty.(type) {
	case *types.Array, *types.Slice, *types.Chan, *types.Map, *types.Struct:
		gen.Printf("fmt.Sprint(%s)", name)
	case *types.Basic:
		if ty.Info()&types.IsString != 0 {
			gen.WriteString(name)
		} else {
			gen.Printf("fmt.Sprint(%s)", name)
		}
	default:
		panic("Unknown type " + ty.String())
	}
}

func (gen *generator) genElement(node *html.Node) error {
	gen.WriteString("out = append(out, func() *html.Node {\n")
	gen.WriteString("var out []*html.Node\n")
	if err := gen.genChildren(node); err != nil {
		return err
	}

	gen.Printf("outNode := &html.Node{Type: html.ElementNode, DataAtom: %d, Data: %q, ", node.DataAtom, node.Data)
	if len(node.Attr) != 0 {
		gen.WriteString("Attr: []html.Attribute{")
		for _, attr := range node.Attr {
			gen.Printf("{Namespace: %q, Key: %q, Val: %q},", attr.Namespace, attr.Key, attr.Val)
		}
		gen.WriteString("}")
	}
	gen.WriteString("}\n")

	gen.WriteString("for _, child := range out {\n")
	gen.WriteString("outNode.AppendChild(child)\n")
	gen.WriteString("}\n")

	gen.WriteString("return outNode\n")
	gen.WriteString("}())\n")
	return nil
}

func (gen *generator) get(path string) (string, types.Type) {
	goName, ty, _ := gen.get_(path, false)
	return goName, ty
}
func (gen *generator) get_(path string, nested bool) (goName string, ty types.Type, rest string) {
	if path == "" {
		return "", nil, ""
	}
	idx := strings.IndexAny(path, ".[]")
	head := ""
	if idx < 0 {
		head, path = path, ""
	} else {
		head, path = path[:idx], path[idx:]
	}
	if head == "" {
		head = "."
		if path == "." {
			path = ""
		}
	}
	goName, ty = unwrap(gen.name(head), gen.ty(head))

	for path != "" && ty != nil {
		sep := path[0]
		path = path[1:]
		switch sep {
		case '.':
			idx := strings.IndexAny(path, ".[]")
			var part string
			if idx < 0 {
				part, path = path, ""
			} else {
				part, path = path[:idx], path[idx:]
			}
			goName, ty = index(goName, part, ty)

		case '[':
			part, partTy, rest := gen.get_(path, true)
			path = rest
			goName, ty = indexVal(goName, part, ty, partTy)

		case ']':
			if nested {
				return goName, ty, path[1:]
			} else {
				return "", nil, ""
			}

		default:
			panic("Invalid byte")
		}
	}

	if nested {
		// Unmatched '['
		return "", nil, ""
	} else {
		return goName, ty, ""
	}
}

func (gen *generator) name(varName string) (goName string) {
	if varName == "." {
		return "dot"
	} else if varName == "$" {
		return "dollar"
	} else {
		return "var_" + varName
	}
}

func (gen *generator) ty(name string) types.Type {
	tys := gen.types[name]
	if len(tys) == 0 {
		return nil
	} else {
		return tys[len(tys)-1]
	}
}
func (gen *generator) pushTy(name string, ty types.Type) {
	gen.types[name] = append(gen.types[name], ty)
}
func (gen *generator) popTy(name string) {
	tys := gen.types[name]
	gen.types[name] = tys[:len(tys)-1]
}

func getAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func index(goName, key string, ty types.Type) (string, types.Type) {
	switch cty := ty.(type) {
	case *types.Array:
		i, err := strconv.ParseInt(key, 0, 0)
		if err != nil || i > cty.Len() {
			return "", nil
		}
		goName += "[" + key + "]"
		ty = cty.Elem()

	case *types.Slice:
		_, err := strconv.ParseInt(key, 0, 0)
		if err != nil {
			return "", nil
		}
		// TODO: don't panic
		goName += "[" + key + "]"
		ty = cty.Elem()

	case *types.Basic, *types.Chan:
		return "", nil

	case *types.Map:
		goName += fmt.Sprintf("[%q]", key)
		ty = cty.Elem()

	case *types.Struct:
		f := fieldByName(cty, key)
		if f == nil {
			return "", nil
		}
		goName += "." + key
		ty = f.Type()

	default:
		panic("Unknown type " + cty.String())
	}

	return unwrap(goName, ty)
}

func indexVal(goName, keyName string, ty, keyTy types.Type) (string, types.Type) {
	panic("TODO")
}

func unwrap(goName string, ty types.Type) (string, types.Type) {
	if ty == nil {
		return goName, ty
	}
	ty = ty.Underlying()
	pty, ok := ty.(*types.Pointer)
	if !ok {
		return goName, ty
	}
	for ok {
		goName = "*" + goName
		ty = pty.Elem().Underlying()
		pty, ok = ty.(*types.Pointer)
	}
	return "(" + goName + ")", ty
}

func fieldByName(s *types.Struct, name string) *types.Var {
	// TODO: support embedding
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if f.Name() == name {
			return f
		}
	}
	return nil
}
