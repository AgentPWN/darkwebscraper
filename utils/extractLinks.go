package utils

import (
	"strings"

	"golang.org/x/net/html"
)

func ExtractPostLinks(body string, prefix string) []string {
	var links []string

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return links
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && strings.HasPrefix(attr.Val, prefix) {
					links = append(links, attr.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)
	return links
}
