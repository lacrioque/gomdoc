// Package templates provides embedded HTML templates for rendering pages.
package templates

import (
	"html/template"
	"io"
	"strings"
)

// PageData holds data for rendering a markdown page.
type PageData struct {
	Title       string
	SiteTitle   string
	Author      string
	Status      string
	Date        string
	Tags        []string
	Category    string
	Version     string
	Reviewers   []string
	Content     template.HTML
	Path        string
	Breadcrumbs template.HTML
	TreeHTML    template.HTML
	PrevPath    string
	PrevTitle   string
	NextPath    string
	NextTitle   string
}

// JoinTags returns tags as a comma-separated string.
func (p PageData) JoinTags() string {
	return strings.Join(p.Tags, ", ")
}

// JoinReviewers returns reviewers as a comma-separated string.
func (p PageData) JoinReviewers() string {
	return strings.Join(p.Reviewers, ", ")
}

// HasMetadata returns true if any extended metadata field is set.
func (p PageData) HasMetadata() bool {
	return p.Status != "" || p.Date != "" || len(p.Tags) > 0 ||
		p.Category != "" || p.Version != "" || len(p.Reviewers) > 0
}

// IndexData holds data for rendering the index page.
type IndexData struct {
	Title     string
	SiteTitle string
	TreeHTML  template.HTML
}

// NotFoundData holds data for the custom 404 page.
type NotFoundData struct {
	SiteTitle   string
	RequestPath string
}

var pageTmpl = template.Must(template.New("page").Parse(pageTemplate))
var indexTmpl = template.Must(template.New("index").Parse(indexTemplate))
var notFoundTmpl = template.Must(template.New("notfound").Parse(notFoundTemplate))

// RenderPage renders a markdown page with navigation.
func RenderPage(w io.Writer, data PageData) error {
	return pageTmpl.Execute(w, data)
}

// RenderIndex renders the index page with the file tree.
func RenderIndex(w io.Writer, data IndexData) error {
	return indexTmpl.Execute(w, data)
}

// RenderNotFound renders the custom 404 page.
func RenderNotFound(w io.Writer, data NotFoundData) error {
	return notFoundTmpl.Execute(w, data)
}

// faviconLink is the favicon as an embedded SVG data URI.
const faviconLink = `<link rel="icon" href="data:image/svg+xml,` +
	`%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'%3E` +
	`%3Crect x='15' y='10' width='55' height='75' rx='3' fill='%230066cc'/%3E` +
	`%3Crect x='20' y='15' width='45' height='65' rx='2' fill='%23fff'/%3E` +
	`%3Crect x='30' y='25' width='25' height='3' rx='1' fill='%230066cc'/%3E` +
	`%3Crect x='30' y='33' width='25' height='2' rx='1' fill='%23ccc'/%3E` +
	`%3Crect x='30' y='39' width='20' height='2' rx='1' fill='%23ccc'/%3E` +
	`%3Crect x='30' y='45' width='25' height='2' rx='1' fill='%23ccc'/%3E` +
	`%3Crect x='30' y='51' width='18' height='2' rx='1' fill='%23ccc'/%3E` +
	`%3Crect x='30' y='57' width='25' height='2' rx='1' fill='%23ccc'/%3E` +
	`%3C/svg%3E">`

// backToTopHTML is the back-to-top button markup and behavior.
const backToTopHTML = `
    <button id="back-to-top" class="back-to-top" aria-label="Back to top" title="Back to top">&#8679;</button>
    <script>
    (function() {
        var btn = document.getElementById('back-to-top');
        window.addEventListener('scroll', function() {
            if (window.scrollY > 300) {
                btn.classList.add('visible');
            } else {
                btn.classList.remove('visible');
            }
        });
        btn.addEventListener('click', function() {
            window.scrollTo({ top: 0, behavior: 'smooth' });
        });
    })();
    </script>`

const themeJS = `
(function() {
    function getEffectiveTheme() {
        var stored = localStorage.getItem('gomdoc-theme');
        if (stored === 'light' || stored === 'dark') return stored;
        return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }

    function applyTheme(theme) {
        if (theme === 'light' || theme === 'dark') {
            document.documentElement.setAttribute('data-theme', theme);
        } else {
            document.documentElement.removeAttribute('data-theme');
        }
        updateToggleLabel();
    }

    function updateToggleLabel() {
        var btn = document.getElementById('theme-toggle');
        if (!btn) return;
        var effective = getEffectiveTheme();
        btn.textContent = effective === 'dark' ? '\u2600\uFE0F' : '\uD83C\uDF19';
        btn.setAttribute('aria-label', effective === 'dark' ? 'Switch to light mode' : 'Switch to dark mode');
    }

    // Apply saved preference on load
    var stored = localStorage.getItem('gomdoc-theme');
    if (stored) applyTheme(stored);

    // Listen for system preference changes
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function() {
        if (!localStorage.getItem('gomdoc-theme')) updateToggleLabel();
    });

    // Expose toggle for the button
    window.gomdocToggleTheme = function() {
        var current = getEffectiveTheme();
        var next = current === 'dark' ? 'light' : 'dark';
        localStorage.setItem('gomdoc-theme', next);
        applyTheme(next);

        // Re-initialize Mermaid with the correct theme if available
        if (typeof mermaid !== 'undefined') {
            mermaid.initialize({ startOnLoad: false, theme: next === 'dark' ? 'dark' : 'default' });
            document.querySelectorAll('.mermaid[data-processed]').forEach(function(el) {
                el.removeAttribute('data-processed');
            });
            mermaid.init(undefined, '.mermaid');
        }
    };
})();
`

const codeBlockJS = `
(function() {
    document.querySelectorAll('pre > code').forEach(function(codeEl) {
        var pre = codeEl.parentElement;

        // Skip mermaid blocks (they get replaced by the mermaid script)
        if (codeEl.classList.contains('language-mermaid')) {
            return;
        }

        // Wrap pre in a container for positioning the copy button
        var wrapper = document.createElement('div');
        wrapper.className = 'code-block-wrapper';
        pre.parentNode.insertBefore(wrapper, pre);
        wrapper.appendChild(pre);

        // Add copy button
        var btn = document.createElement('button');
        btn.className = 'copy-btn';
        btn.textContent = 'Copy';
        btn.setAttribute('aria-label', 'Copy code to clipboard');
        btn.addEventListener('click', function() {
            var text = codeEl.textContent;
            navigator.clipboard.writeText(text).then(function() {
                btn.textContent = 'Copied!';
                btn.classList.add('copied');
                setTimeout(function() {
                    btn.textContent = 'Copy';
                    btn.classList.remove('copied');
                }, 2000);
            });
        });
        wrapper.appendChild(btn);

        // Add line numbers: wrap each line in a span
        var lines = codeEl.innerHTML.split('\n');
        // Remove trailing empty line (common in code blocks)
        if (lines.length > 0 && lines[lines.length - 1].trim() === '') {
            lines.pop();
        }
        if (lines.length > 1) {
            pre.classList.add('line-numbers');
            codeEl.innerHTML = lines.map(function(line) {
                return '<span class="line">' + line + '</span>';
            }).join('\n');
        }
    });
})();
`

const searchJS = `
(function() {
    var input = document.getElementById('search-input');
    var resultsDiv = document.getElementById('search-results');
    var debounceTimer;

    input.addEventListener('input', function() {
        clearTimeout(debounceTimer);
        var query = input.value.trim();
        if (query.length < 2) {
            resultsDiv.innerHTML = '';
            resultsDiv.style.display = 'none';
            return;
        }
        debounceTimer = setTimeout(function() {
            fetch('/api/search?q=' + encodeURIComponent(query))
                .then(function(r) { return r.json(); })
                .then(function(results) {
                    if (results.length === 0) {
                        resultsDiv.innerHTML = '<div class="search-no-results">No results found</div>';
                        resultsDiv.style.display = 'block';
                        return;
                    }
                    var html = '';
                    results.forEach(function(r) {
                        html += '<a class="search-result" href="' + r.path + '">';
                        html += '<div class="search-result-title">' + escapeHtml(r.title) + '</div>';
                        html += '<div class="search-result-snippet">' + escapeHtml(r.snippet) + '</div>';
                        html += '</a>';
                    });
                    resultsDiv.innerHTML = html;
                    resultsDiv.style.display = 'block';
                });
        }, 200);
    });

    document.addEventListener('click', function(e) {
        if (!e.target.closest('.search-box')) {
            resultsDiv.style.display = 'none';
        }
    });

    input.addEventListener('focus', function() {
        if (resultsDiv.innerHTML) resultsDiv.style.display = 'block';
    });

    function escapeHtml(text) {
        var div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
})();
`

const tocJS = `
(function() {
    var sidebar = document.getElementById('toc-sidebar');
    var tocList = document.getElementById('toc-list');
    var headings = document.querySelectorAll('.content h1, .content h2, .content h3');

    if (headings.length < 2) {
        sidebar.style.display = 'none';
        return;
    }

    headings.forEach(function(heading, index) {
        if (!heading.id) {
            heading.id = 'heading-' + index;
        }
        var li = document.createElement('li');
        li.className = 'toc-item toc-' + heading.tagName.toLowerCase();
        var a = document.createElement('a');
        a.href = '#' + heading.id;
        a.textContent = heading.textContent;
        a.addEventListener('click', function(e) {
            e.preventDefault();
            heading.scrollIntoView({ behavior: 'smooth' });
            history.replaceState(null, '', '#' + heading.id);
        });
        li.appendChild(a);
        tocList.appendChild(li);
    });

    var tocLinks = tocList.querySelectorAll('a');
    var observer = new IntersectionObserver(function(entries) {
        entries.forEach(function(entry) {
            if (!entry.isIntersecting) {
                return;
            }
            tocLinks.forEach(function(link) {
                link.classList.remove('toc-active');
            });
            var activeLink = tocList.querySelector('a[href="#' + entry.target.id + '"]');
            if (activeLink) {
                activeLink.classList.add('toc-active');
            }
        });
    }, {
        rootMargin: '0px 0px -70% 0px',
        threshold: 0
    });

    headings.forEach(function(heading) {
        observer.observe(heading);
    });
})();
`

const pageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - {{.SiteTitle}}</title>
    ` + faviconLink + `
    <link rel="stylesheet" href="/static/style.css">
</head>
<body class="has-sidebar">
    <header class="print-header">
        <h1 class="print-title">{{.Title}}</h1>
        {{if .Author}}<p class="print-author">{{.Author}}</p>{{end}}
    </header>
    <nav class="nav-buttons">
        <a href="/"><button class="nav-btn">Home</button></a>
        <div class="search-box">
            <input type="text" id="search-input" placeholder="Search..." autocomplete="off">
            <div id="search-results" class="search-results"></div>
        </div>
        <button onclick="window.print()" class="nav-btn print-btn">Print</button>
        <button id="theme-toggle" class="theme-toggle" onclick="gomdocToggleTheme()" aria-label="Toggle dark mode">🌙</button>
    </nav>
    {{.Breadcrumbs}}
    <div class="page-layout">
        <aside class="sidebar">{{.TreeHTML}}</aside>
        <div class="page-main">
            {{if .HasMetadata}}<div class="doc-metadata">
                {{if .Status}}<span class="meta-item meta-status meta-status-{{.Status}}">{{.Status}}</span>{{end}}
                {{if .Category}}<span class="meta-item meta-category">{{.Category}}</span>{{end}}
                {{if .Version}}<span class="meta-item meta-version">v{{.Version}}</span>{{end}}
                {{if .Date}}<span class="meta-item meta-date">{{.Date}}</span>{{end}}
                {{if .Tags}}<span class="meta-item meta-tags">{{.JoinTags}}</span>{{end}}
                {{if .Reviewers}}<span class="meta-item meta-reviewers">Reviewers: {{.JoinReviewers}}</span>{{end}}
            </div>{{end}}
            <main class="content">
                {{.Content}}
            </main>
            <nav class="prev-next-nav">
                {{if .PrevPath}}<a href="{{.PrevPath}}" class="prev-next-btn prev-btn">&larr; {{.PrevTitle}}</a>{{end}}
                <span class="prev-next-spacer"></span>
                {{if .NextPath}}<a href="{{.NextPath}}" class="prev-next-btn next-btn">{{.NextTitle}} &rarr;</a>{{end}}
            </nav>
        </div>
        <aside id="toc-sidebar" class="toc-sidebar">
            <nav class="toc-nav">
                <h3 class="toc-title">On this page</h3>
                <ul id="toc-list" class="toc-list"></ul>
            </nav>
        </aside>
    </div>
    <footer class="site-footer">
        Documentation created by gomdoc: <a href="https://github.com/lacrioque/gomdoc/">https://github.com/lacrioque/gomdoc/</a>
    </footer>
    <script>` + themeJS + `</script>
    <script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
    <script>
        (function() {
            var isDark = document.documentElement.getAttribute('data-theme') === 'dark' ||
                (!document.documentElement.getAttribute('data-theme') &&
                 window.matchMedia('(prefers-color-scheme: dark)').matches);
            mermaid.initialize({ startOnLoad: true, theme: isDark ? 'dark' : 'default' });
        })();
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
    <script>` + codeBlockJS + `</script>
    <script>` + searchJS + `</script>
    <script>` + tocJS + `</script>` + backToTopHTML + `
</body>
</html>`

const folderToggleJS = `
(function() {
    var STORAGE_KEY = 'gomdoc-folder-state';

    function loadState() {
        try {
            var raw = localStorage.getItem(STORAGE_KEY);
            if (!raw) return null;
            return JSON.parse(raw);
        } catch(e) {
            return null;
        }
    }

    function saveState() {
        var state = {};
        document.querySelectorAll('.folder-details').forEach(function(d) {
            var key = d.getAttribute('data-folder');
            if (key) {
                state[key] = d.open;
            }
        });
        try {
            localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
        } catch(e) {}
    }

    // Restore saved state on load
    var saved = loadState();
    if (saved) {
        document.querySelectorAll('.folder-details').forEach(function(d) {
            var key = d.getAttribute('data-folder');
            if (key && saved.hasOwnProperty(key)) {
                d.open = saved[key];
            }
        });
    }

    // Persist state on toggle
    document.querySelectorAll('.folder-details').forEach(function(d) {
        d.addEventListener('toggle', saveState);
    });
})();
`

const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Index - {{.SiteTitle}}</title>
    ` + faviconLink + `
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <nav class="nav-buttons">
        <span class="nav-title">{{.SiteTitle}}</span>
        <div class="search-box">
            <input type="text" id="search-input" placeholder="Search..." autocomplete="off">
            <div id="search-results" class="search-results"></div>
        </div>
        <button id="theme-toggle" class="theme-toggle" onclick="gomdocToggleTheme()" aria-label="Toggle dark mode">🌙</button>
    </nav>
    <main class="content index-content">
        <h1>File Index</h1>
        {{.TreeHTML}}
    </main>
    <footer class="site-footer">
        Documentation created by gomdoc: <a href="https://github.com/lacrioque/gomdoc/">https://github.com/lacrioque/gomdoc/</a>
    </footer>
    <script>` + themeJS + `</script>
    <script>` + searchJS + `</script>
    <script>` + folderToggleJS + `</script>` + backToTopHTML + `
</body>
</html>`

const notFoundTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Page Not Found - {{.SiteTitle}}</title>
    ` + faviconLink + `
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <nav class="nav-buttons">
        <button onclick="history.back()" class="nav-btn">Back</button>
        <a href="/"><button class="nav-btn">Home</button></a>
        <div class="search-box">
            <input type="text" id="search-input" placeholder="Search..." autocomplete="off">
            <div id="search-results" class="search-results"></div>
        </div>
    </nav>
    <main class="content not-found-content">
        <h1>404 - Page Not Found</h1>
        <p>The page <code>{{.RequestPath}}</code> could not be found.</p>
        <p>Try searching for what you need, or go back to the <a href="/">home page</a>.</p>
    </main>
    <footer class="site-footer">
        Documentation created by gomdoc: <a href="https://github.com/lacrioque/gomdoc/">https://github.com/lacrioque/gomdoc/</a>
    </footer>
    <script>` + searchJS + `</script>
</body>
</html>`
