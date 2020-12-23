package htmpl

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var nltabRemover = strings.NewReplacer("\n", "", "\t", "")

func testFrag(t *testing.T, dot interface{}, input, output string) {
	t.Helper()

	input = nltabRemover.Replace(input)
	output = nltabRemover.Replace(output)

	root := &html.Node{Type: html.ElementNode}
	nodes, err := html.ParseFragment(strings.NewReader(input), root)
	if err != nil {
		t.Error(err)
		return
	}

	root.Type = html.DocumentNode
	for _, child := range nodes {
		root.AppendChild(child)
	}
	nodes = Evaluate(root, dot)

	newRoot := &html.Node{Type: html.DocumentNode}
	for _, child := range nodes {
		newRoot.AppendChild(child)
	}

	b := strings.Builder{}
	if err := html.Render(&b, newRoot); err != nil {
		t.Error(err)
		return
	}
	if b.String() != output {
		t.Errorf("Expected and actual output do not match:\n\tExpected: %q\n\tReceived: %q", output, b.String())
	}
}

// Static HTML should not be modified
func TestStatic(t *testing.T) {
	testFrag(t, nil, ``, ``)
	testFrag(t, nil, `
		<div>
			<script>
				<v>abc</v>
				<if v="foo">
				</if>
			</script>
		</div>
	`, `
		<div>
			<script>
				<v>abc</v>
				<if v="foo">
				</if>
			</script>
		</div>
	`)
}

// <v> should substitute values
func TestV(t *testing.T) {
	testFrag(t, map[string]string{"foo": "bar", "baz": "quux"}, `
		<v>.foo</v>
		<v>.baz</v>
	`, `
		bar
		quux
	`)
	type testData struct {
		Foo  string
		Bar  int
		Baz  map[string]interface{}
		Quux []string
	}
	testFrag(t, testData{
		Foo: "I am foo",
		Bar: -42,
		Baz: map[string]interface{}{
			"hello": 7,
			"world": -76.3,
		},
		Quux: []string{"a", "b", "c", "d"},
	}, `
		<v>.Foo</v>
		<v>.Bar</v>
		<v>.Baz.hello</v>
		<v>.Baz.world</v>
		<v>.Quux.2</v>
	`, `
		I am foo
		-42
		7
		-76.3
		c
	`)
}

// <if> should render its contents iff the condition is truthy
func TestIf(t *testing.T) {
	testFrag(t, map[string]bool{"t": true, "f": false}, `
		<if v=".t">
			Hi there!
		</if>
		<if v=".f">
			I'm not displayed :(
		</if>
	`, `
		Hi there!
	`)
	testFrag(t, map[string]interface{}{
		"empty":      "",
		"hello":      "world",
		"slice":      []string{"foo", "bar"},
		"emptySlice": []string{},
	}, `
		<if v=".empty">
			No
		</if>
		<if v=".hello">
			Hello, <v>.hello</v>!
		</if>
		<if v=".slice">
			There are items!
		</if>
		<if v=".emptySlice">
			What, why does <em>this</em> have items?!
		</if>
	`, `
		Hello, world!
		There are items!
	`)
}

// <nif> should render its contents iff the condition is falsey
func TestNif(t *testing.T) {
	testFrag(t, map[string]bool{"t": true, "f": false}, `
		<nif v=".t">
			Hi there!
		</nif>
		<nif v=".f">
			Yay I'm displayed this time!
		</nif>
	`, `
		Yay I&#39;m displayed this time!
	`)
	testFrag(t, map[string]interface{}{
		"empty":      "",
		"hello":      "world",
		"slice":      []string{"foo", "bar"},
		"emptySlice": []string{},
	}, `
		<nif v=".empty">
			Yes
		</nif>
		<nif v=".hello">
			Hello, <v>.hello</v>!
		</nif>
		<nif v=".slice">
			There are items!
		</nif>
		<nif v=".emptySlice">
			No items here
		</nif>
	`, `
		Yes
		No items here
	`)
}

// <for> should render its contents once for each item in a collection
func TestFor(t *testing.T) {
	testFrag(t, []string{}, `
		<for v=".">
			No
		</for>
		<for v="bad">
			Noo
		</for>
	`, ``)
	testFrag(t, "Hello, world!", `
		<for v=".">
			<v>.</v>
		</for>
	`, `
		Hello, world!
	`)
	testFrag(t, []string{"Bob", "Jim", "Fred"}, `
		<for v=".">
			<v>.</v>
		</for>
	`, `
		Bob
		Jim
		Fred
	`)
	testFrag(t, map[string]int{"apples": 3, "bananas": 7}, `
		<for v=".">
			<v>.</v>: <v>$.map[.]</v>
		</for>
	`, `
		apples: 3
		bananas: 7
	`)
}

// <let> should bind variables within its body
func TestLet(t *testing.T) {
	testFrag(t, map[string]string{"foo": "bar", "baz": "faz"}, `
		<v>.foo</v>
		<v>foo</v>
		{
		<let var="foo", val=".foo">
			<v>.foo</v>
			<v>foo</v>
		</let>
		}
		<v>.foo</v>
		<v>foo</v>
		{
		<let var=".", val=".baz">
			<v>.foo</v>
			<v>.</v>
		</let>
		}
	`, `
		bar
		{
			bar
			bar
		}
		bar
		{
			faz
		}
	`)
}

// Node values should be stringified to HTML
func TestStringifyNode(t *testing.T) {
	testFrag(t, html.Node{Type: html.ElementNode, DataAtom: atom.H1, Data: "h1"}, `
		<v>.</v>
	`, `
		&lt;h1&gt;&lt;/h1&gt;
	`)
	testFrag(t, []*html.Node{
		&html.Node{Type: html.ElementNode, DataAtom: atom.H1, Data: "h1"},
		&html.Node{Type: html.ElementNode, DataAtom: atom.H2, Data: "h2"},
		&html.Node{Type: html.ElementNode, DataAtom: atom.H3, Data: "h3"},
	}, `
		<v>.</v>
	`, `
		&lt;h1&gt;&lt;/h1&gt;
		&lt;h2&gt;&lt;/h2&gt;
		&lt;h3&gt;&lt;/h3&gt;
	`)
}
