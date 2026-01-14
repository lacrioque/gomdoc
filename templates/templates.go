// Package templates provides embedded HTML templates for rendering pages.
package templates

import (
	"html/template"
	"io"
)

// PageData holds data for rendering a markdown page.
type PageData struct {
	Title   string
	Content template.HTML
	Path    string
}

// IndexData holds data for rendering the index page.
type IndexData struct {
	Title    string
	TreeHTML template.HTML
}

var pageTmpl = template.Must(template.New("page").Parse(pageTemplate))
var indexTmpl = template.Must(template.New("index").Parse(indexTemplate))

// RenderPage renders a markdown page with navigation.
func RenderPage(w io.Writer, data PageData) error {
	return pageTmpl.Execute(w, data)
}

// RenderIndex renders the index page with the file tree.
func RenderIndex(w io.Writer, data IndexData) error {
	return indexTmpl.Execute(w, data)
}

const pageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - gomdoc</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <nav class="nav-buttons">
        <button onclick="history.back()" class="nav-btn">Back</button>
        <a href="/"><button class="nav-btn">Home</button></a>
        <span class="current-path">{{.Path}}</span>
    </nav>
    <main class="content">
        {{.Content}}
    </main>
    <script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
    <script>
        mermaid.initialize({ startOnLoad: true, theme: 'default' });
        // Find all code blocks with class "language-mermaid" and convert them
        document.querySelectorAll('pre > code.language-mermaid').forEach(function(codeBlock) {
            var pre = codeBlock.parentElement;
            var div = document.createElement('div');
            div.className = 'mermaid';
            div.textContent = codeBlock.textContent;
            pre.parentNode.replaceChild(div, pre);
        });
        mermaid.init(undefined, '.mermaid');
    </script>
</body>
</html>`

const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Index - gomdoc</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <nav class="nav-buttons">
        <span class="nav-title">gomdoc</span>
    </nav>
    <main class="content index-content">
        <h1>File Index</h1>
        {{.TreeHTML}}
    </main>
</body>
</html>`
