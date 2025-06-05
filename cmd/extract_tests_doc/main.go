package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

type TestDoc struct {
	Name string
	File string
	Doc  string
	Tags []string
}

type TagInfo struct {
	Tests []TestDoc
}

func main() {
	// Load tag descriptions from YAML
	tagDescriptions, err := loadTagDescriptions("tags.yaml")
	if err != nil {
		fmt.Println("Failed to load tags.yaml:", err)
		return
	}

	// List all package dirs
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", "./...")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error running go list:", err)
		return
	}
	dirs := strings.Split(strings.TrimSpace(string(output)), "\n")

	tagMap := map[string]*TagInfo{}
	untaggedTests := []TestDoc{}

	// Walk through all test files
	for _, dir := range dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || !strings.HasSuffix(path, "_test.go") {
				return nil
			}

			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				fmt.Printf("Failed to parse %s: %v\n", path, err)
				return nil
			}

			for _, decl := range node.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Recv != nil {
					continue
				}

				var docLines []string
				tagSet := map[string]struct{}{}

				if fn.Doc != nil {
					for _, comment := range fn.Doc.List {
						line := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
						if strings.HasPrefix(line, "@tag:") {
							tag := strings.TrimSpace(strings.TrimPrefix(line, "@tag:"))
							if tag != "" {
								tagSet[tag] = struct{}{}
							}
						} else if !strings.HasPrefix(line, "@") {
							docLines = append(docLines, line)
						}
					}
				}

				// Convert tagSet to slice
				tags := make([]string, 0, len(tagSet))
				for tag := range tagSet {
					tags = append(tags, tag)
				}
				sort.Strings(tags)

				td := TestDoc{
					Name: fn.Name.Name,
					File: path,
					Doc:  strings.Join(docLines, "\n"),
					Tags: tags,
				}

				if len(tags) == 0 {
					untaggedTests = append(untaggedTests, td)
				} else {
					for _, tag := range tags {
						if _, exists := tagMap[tag]; !exists {
							tagMap[tag] = &TagInfo{}
						}
						tagMap[tag].Tests = append(tagMap[tag].Tests, td)
					}
				}
			}
			return nil
		})
	}

	// Start Markdown output
	fmt.Println("# OP stack test documentation and categorization\n")
	fmt.Println("## 📚 Index\n")

	// Build sorted tag list
	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		count := uniqueTestCount(tagMap[tag].Tests)
		fmt.Printf("- [%s (%d test%s)](#%s)\n", tag, count, plural(count), sanitizeAnchor(tag))
	}
	if len(untaggedTests) > 0 {
		count := len(untaggedTests)
		fmt.Printf("- [_untagged_ (%d test%s)](#untagged)\n", count, plural(count))
	}
	fmt.Println()

	// Render each tag section
	for _, tag := range tags {
		info := tagMap[tag]
		desc := tagDescriptions[tag]
		testCount := uniqueTestCount(info.Tests)

		fmt.Printf("## %s\n\n", tag)

		summary := fmt.Sprintf("<strong>%s</strong>", tag)
		if desc != "" {
			summary += fmt.Sprintf(" — %s", desc)
		}
		summary += fmt.Sprintf(" (%d test%s)", testCount, plural(testCount))

		fmt.Println("<details open>")
		fmt.Printf("<summary>%s</summary>\n\n", summary)

		seen := map[string]struct{}{}
		for _, doc := range info.Tests {
			id := doc.File + "::" + doc.Name
			if _, ok := seen[id]; ok {
				continue // avoid duplicate in case of shared tag
			}
			seen[id] = struct{}{}
			printTestMarkdown(doc)
		}

		fmt.Println("</details>\n")
	}

	// Render untagged
	if len(untaggedTests) > 0 {
		fmt.Println("## _untagged_\n")

		summary := fmt.Sprintf("<strong>_untagged_</strong> (%d test%s)", len(untaggedTests), plural(len(untaggedTests)))
		fmt.Println("<details>")
		fmt.Printf("<summary>%s</summary>\n\n", summary)
		for _, doc := range untaggedTests {
			printTestMarkdown(doc)
		}
		fmt.Println("</details>\n")
	}
}

func printTestMarkdown(doc TestDoc) {
	fmt.Printf("### `%s`\n\n", doc.Name)
	fmt.Printf("_File: `%s`_\n\n", doc.File)
	fmt.Println("```go")
	if doc.Doc != "" {
		fmt.Println(doc.Doc)
	} else {
		fmt.Println("_No documentation provided._")
	}
	fmt.Println("```\n")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func sanitizeAnchor(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
}

func loadTagDescriptions(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tagDescriptions map[string]string
	if err := yaml.Unmarshal(data, &tagDescriptions); err != nil {
		return nil, err
	}
	return tagDescriptions, nil
}

func uniqueTestCount(tests []TestDoc) int {
	seen := map[string]struct{}{}
	for _, t := range tests {
		key := t.File + "::" + t.Name
		seen[key] = struct{}{}
	}
	return len(seen)
}
