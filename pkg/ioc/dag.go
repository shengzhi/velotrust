package ioc

import (
	"fmt"
	"reflect"
	"strings"
)

type Node struct {
	Name   string
	parent *Node
	childs []*Node
}

func NewNode(t reflect.Type) *Node {
	return &Node{
		Name: fmt.Sprintf("%s.%s", t.PkgPath(), t.Name()),
	}
}

func (n *Node) Add(child *Node) error {
	if n == nil || child == nil {
		return fmt.Errorf("dependency graph node is nil")
	}
	for current := n; current != nil; current = current.parent {
		if current.Name == child.Name {
			return fmt.Errorf("object %s is cycle reference", child.Name)
		}
	}
	child.parent = n
	n.childs = append(n.childs, child)
	return nil
}

func (n *Node) Show() string {
	sb := strings.Builder{}
	sb.WriteString(n.Name)
	sb.WriteString("\r\n")
	for _, child := range n.childs {
		fmt.Fprintf(&sb, "\t%s\r\n", child.Name)
	}
	return sb.String()
}
