package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/vktec/htmlparse"
	"github.com/vktec/htmpl"
	"golang.org/x/net/html"
)

func main() {
	tmplFile := flag.String("t", "", "template `file`name")
	dataStr := flag.String("d", "", "`JSON` data to render the template with. If not specified, htmpl will read from stdin")
	flag.Parse()
	log.SetFlags(0)

	if *tmplFile == "" {
		log.Fatal("-t must be provided")
	}
	tmpl, err := ioutil.ReadFile(*tmplFile)
	if err != nil {
		log.Fatal(err)
	}
	node := &html.Node{Type: html.DocumentNode}
	if err := htmlparse.Parse(node, tmpl); err != nil {
		log.Fatal(err)
	}

	var data map[string]interface{}
	if *dataStr != "" {
		err = json.Unmarshal([]byte(*dataStr), &data)
	} else {
		err = json.NewDecoder(os.Stdin).Decode(&data)
	}
	if err != nil {
		log.Fatal(err)
	}

	nodes := htmpl.Evaluate(node, data)
	result := &html.Node{Type: html.DocumentNode}
	for _, child := range nodes {
		result.AppendChild(child)
	}
	if err := html.Render(os.Stdout, result); err != nil {
		log.Fatal(err)
	}
}
