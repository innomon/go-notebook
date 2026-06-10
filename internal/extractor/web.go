package extractor

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

// ExtractTextFromURL downloads a web page and converts its HTML content to clean Markdown
func ExtractTextFromURL(urlStr string) (string, string, error) {
	resp, err := http.Get(urlStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP status error: %d %s", resp.StatusCode, resp.Status)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// 1. Extract title
	title := ""
	var findTitle func(*html.Node)
	findTitle = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			if n.FirstChild != nil {
				title = strings.TrimSpace(n.FirstChild.Data)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTitle(c)
		}
	}
	findTitle(doc)

	// Fallback title to domain/URL if empty
	if title == "" {
		title = urlStr
	}

	// 2. Convert body HTML to Markdown
	var buf bytes.Buffer
	var convertNode func(*html.Node)
	convertNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Skip scripts, styles, navs, footers, headers
			name := strings.ToLower(n.Data)
			if name == "script" || name == "style" || name == "nav" || name == "footer" || name == "header" || name == "head" || name == "aside" || name == "form" || name == "iframe" {
				return
			}

			// Format headers
			switch name {
			case "h1":
				buf.WriteString("\n\n# ")
			case "h2":
				buf.WriteString("\n\n## ")
			case "h3":
				buf.WriteString("\n\n### ")
			case "h4":
				buf.WriteString("\n\n#### ")
			case "h5":
				buf.WriteString("\n\n##### ")
			case "h6":
				buf.WriteString("\n\n###### ")
			case "p":
				buf.WriteString("\n\n")
			case "br":
				buf.WriteString("\n")
			case "li":
				buf.WriteString("\n- ")
			case "strong", "b":
				buf.WriteString("**")
			case "em", "i":
				buf.WriteString("*")
			case "a":
				// Find href
				href := ""
				for _, attr := range n.Attr {
					if strings.ToLower(attr.Key) == "href" {
						href = attr.Val
						break
					}
				}
				if href != "" {
					buf.WriteString(fmt.Sprintf(" [%s](%s) ", getInnerText(n), href))
					return // Skip children since we got the inner text
				}
			}
		} else if n.Type == html.TextNode {
			txt := n.Data
			// Minimize spaces
			txt = strings.ReplaceAll(txt, "\t", " ")
			txt = strings.ReplaceAll(txt, "\n", " ")
			buf.WriteString(txt)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			convertNode(c)
		}

		// Close inline elements
		if n.Type == html.ElementNode {
			name := strings.ToLower(n.Data)
			switch name {
			case "strong", "b":
				buf.WriteString("**")
			case "em", "i":
				buf.WriteString("*")
			case "h1", "h2", "h3", "h4", "h5", "h6", "p":
				buf.WriteString("\n\n")
			}
		}
	}

	convertNode(doc)

	// Clean up whitespace
	content := buf.String()
	// Replace double newlines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return title, strings.TrimSpace(content), nil
}

func getInnerText(n *html.Node) string {
	var buf bytes.Buffer
	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(n)
	return strings.TrimSpace(buf.String())
}
