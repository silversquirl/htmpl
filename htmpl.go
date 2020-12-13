package htmpl

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func Evaluate(node *html.Node, dot interface{}) []*html.Node {
	eval := evaluator{make(map[string][]reflect.Value)}
	vdot := reflect.ValueOf(dot)
	eval.push(".", vdot)
	eval.push("$", vdot)
	return eval.eval(node)
}

type evaluator struct {
	vars map[string][]reflect.Value
}

func (eval *evaluator) eval(node *html.Node) []*html.Node {
	switch node.Type {
	case html.DocumentNode:
		return eval.children(node)

	case html.ElementNode:
		switch node.Data {
		case "if":
			if isTruthy(eval.v(node)) {
				return eval.children(node)
			} else {
				return nil
			}

		case "nif":
			if !isTruthy(eval.v(node)) {
				return eval.children(node)
			} else {
				return nil
			}

		case "for":
			return eval.iterate(node, eval.v(node))

		case "let":
			varName := getAttr(node, "var")
			valPath := getAttr(node, "val")
			eval.push(varName, eval.get(valPath))
			nodes := eval.children(node)
			eval.pop(varName)
			return nodes

		case "v":
			// FIXME: should handle multiple child nodes
			if node.FirstChild != nil && node.FirstChild.Type == html.TextNode {
				path := node.FirstChild.Data
				v := stringify(eval.get(path))
				return []*html.Node{&html.Node{Type: html.TextNode, Data: v}}
			} else {
				return nil
			}

		default:
			ret := shallowClone(node)
			for _, child := range eval.children(node) {
				ret.AppendChild(child)
			}
			return []*html.Node{ret}
		}

	default:
		return []*html.Node{shallowClone(node)}
	}
}
func shallowClone(node *html.Node) *html.Node {
	return &html.Node{
		Type:      node.Type,
		DataAtom:  node.DataAtom,
		Data:      node.Data,
		Namespace: node.Namespace,
	}
}

// evalChildren evaluates all children of a given node
func (eval *evaluator) children(node *html.Node) (nodes []*html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		nodes = append(nodes, eval.eval(child)...)
	}
	return
}

// iterate evaluates all children of a given node, once for each item in the specified collection
func (eval *evaluator) iterate(node *html.Node, v reflect.Value) (nodes []*html.Node) {
	switch v.Kind() {
	case reflect.Invalid:
		nodes = nil
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			eval.push(".", v.Index(i))
			nodes = append(nodes, eval.children(node)...)
			eval.pop(".")
		}
	case reflect.Chan:
		x, ok := v.Recv()
		for ok {
			eval.push(".", x)
			nodes = append(nodes, eval.children(node)...)
			x, ok = v.Recv()
			eval.pop(".")
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			eval.push(".", it.Key())
			nodes = append(nodes, eval.children(node)...)
			eval.pop(".")
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			eval.push(".", v.Field(i))
			nodes = append(nodes, eval.children(node)...)
			eval.pop(".")
		}
	default:
		nodes = eval.children(node)
	}
	return
}

func (eval *evaluator) v(node *html.Node) reflect.Value {
	return eval.get(getAttr(node, "v"))
}
func getAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func (eval *evaluator) get(pathString string) reflect.Value {
	path := strings.Split(pathString, ".")
	for i, part := range path {
		path[i] = strings.Trim(part, " \t\r\n")
	}

	if path[0] == "" {
		if len(path) == 1 {
			return reflect.Value{}
		}

		path[0] = "."
		if path[1] == "" {
			path = path[:1]
		}
	}

	// TODO: bracketed keys

	vals := eval.vars[path[0]]
	if len(vals) == 0 {
		return reflect.Value{}
	}
	v := vals[len(vals)-1]

	for _, part := range path[1:] {
		v = index(v, part)
		if !v.IsValid() {
			break
		}
	}
	return v
}

func (eval *evaluator) push(varName string, v reflect.Value) {
	eval.vars[varName] = append(eval.vars[varName], v)
}
func (eval *evaluator) pop(varName string) {
	v := eval.vars[varName]
	eval.vars[varName] = v[:len(v)-1]
}

func isTruthy(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Invalid:
		return false
	case reflect.Array, reflect.Slice:
		return v.Len() > 0
	case reflect.Map, reflect.Struct:
		return true
	default:
		return !v.IsZero()
	}
}

func index(v reflect.Value, key string) reflect.Value {
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		i, err := strconv.Atoi(key)
		if err != nil || i < 0 || i > v.Len() {
			return reflect.Value{}
		}
		v = v.Index(i)
	case reflect.Map:
		v = v.MapIndex(reflect.ValueOf(key))
	case reflect.Struct:
		v = v.FieldByName(key)
	default:
		return reflect.Value{}
	}
	return unwrap(v)
}
func unwrap(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func stringify(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}
	return fmt.Sprint(v) // TODO: improve this
}
