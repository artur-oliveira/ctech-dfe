package documents

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type xmlNode struct {
	name     string
	text     string
	attrs    map[string]string
	children []*xmlNode
}

func parseXML(data []byte) (*xmlNode, error) {
	if len(data) == 0 || len(data) > maxSourceXMLBytes {
		return nil, fmt.Errorf("XML size must be between 1 and %d bytes", maxSourceXMLBytes)
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	var root *xmlNode
	var stack []*xmlNode
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse fiscal XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &xmlNode{name: t.Name.Local, attrs: make(map[string]string, len(t.Attr))}
			for _, attr := range t.Attr {
				n.attrs[attr.Name.Local] = attr.Value
			}
			if len(stack) == 0 {
				root = n
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			}
			stack = append(stack, n)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(t)
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("fiscal XML has no root element")
	}
	return root, nil
}

func (n *xmlNode) child(name string) *xmlNode {
	if n == nil {
		return nil
	}
	for _, child := range n.children {
		if child.name == name {
			return child
		}
	}
	return nil
}

func (n *xmlNode) childrenNamed(name string) []*xmlNode {
	if n == nil {
		return nil
	}
	out := make([]*xmlNode, 0)
	for _, child := range n.children {
		if child.name == name {
			out = append(out, child)
		}
	}
	return out
}

func (n *xmlNode) firstDeep(name string) *xmlNode {
	if n == nil {
		return nil
	}
	if n.name == name {
		return n
	}
	for _, child := range n.children {
		if found := child.firstDeep(name); found != nil {
			return found
		}
	}
	return nil
}

func (n *xmlNode) value(name string) string {
	child := n.child(name)
	if child == nil {
		return ""
	}
	return strings.TrimSpace(child.text)
}

func (n *xmlNode) attr(name string) string {
	if n == nil {
		return ""
	}
	return n.attrs[name]
}
