package utils

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
)

func HasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			return slices.Contains(strings.Fields(attr.Val), class)
		}
	}
	return false
}
