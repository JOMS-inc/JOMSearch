package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
	"github.com/flosch/pongo2/v7"
)

const (
	dbPath      = "../db/whoknows.db"
	templateDir = "internal/templates"
	staticDir   = "internal/static"

	descriptionMaxLen = 200
)

type SearchResult struct {
	Title       string
	Description string
	URL         string
}

func RegisterSearchRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /search", search)

	mux.Handle(
		"GET /static/",
		http.StripPrefix(
			"/static/",
			http.FileServer(http.Dir(staticDir)),
		),
	)
}

func search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	language := r.URL.Query().Get("language")

	if language == "" {
		language = "en"
	}

	searchResults := []SearchResult{}

	if q != "" {
		conn, err := db.Open(dbPath)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		rows, err := conn.Query(`
			SELECT
				title,
				content AS description,
				url
			FROM pages
			WHERE language = ?
			  AND content LIKE ?
		`, language, "%"+q+"%")

		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var result SearchResult

			if err := rows.Scan(
				&result.Title,
				&result.Description,
				&result.URL,
			); err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			result.Description = truncate(result.Description, descriptionMaxLen)

			searchResults = append(searchResults, result)
		}

		if err := rows.Err(); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	templateResults := make([]pongo2.Context, 0, len(searchResults))

	for _, result := range searchResults {
		templateResults = append(templateResults, pongo2.Context{
			"title":       result.Title,
			"description": result.Description,
			"url":         result.URL,
		})
	}

	tempDir, err := os.MkdirTemp("", "jomsearch-templates-*")
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	if err := prepareTemplate(
		filepath.Join(templateDir, "layout.html"),
		filepath.Join(tempDir, "layout.html"),
	); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := prepareTemplate(
		filepath.Join(templateDir, "search.html"),
		filepath.Join(tempDir, "search.html"),
	); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	loader, err := pongo2.NewLocalFileSystemLoader(tempDir)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	templateSet := pongo2.NewSet("search", loader)

	tmpl, err := templateSet.FromFile("search.html")
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	ctx := pongo2.Context{
		"query":          q,
		"search_results": templateResults,
		"g": pongo2.Context{
			"user": nil,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := tmpl.ExecuteWriter(ctx, w); err != nil {
		return
	}
}

// truncate shortens s to at most maxLen characters (runes), trimming
// trailing whitespace and appending an ellipsis if truncation occurred.
func truncate(s string, maxLen int) string {
	runes := []rune(s)

	if len(runes) <= maxLen {
		return s
	}

	truncated := string(runes[:maxLen])
	return strings.TrimRight(truncated, " \t\n\r") + "…"
}

func prepareTemplate(sourcePath, destinationPath string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	template := convertFlaskTemplate(string(data))

	return os.WriteFile(
		destinationPath,
		[]byte(template),
		0644,
	)
}

func convertFlaskTemplate(template string) string {
	template = strings.ReplaceAll(
		template,
		"{{ url_for('static', filename='style.css') }}",
		"/static/style.css",
	)

	template = strings.ReplaceAll(
		template,
		`{{ url_for("static", filename="style.css") }}`,
		"/static/style.css",
	)

	template = strings.ReplaceAll(
		template,
		"{{ url_for('search') }}",
		"/search",
	)

	template = strings.ReplaceAll(
		template,
		"{{ url_for('login') }}",
		"/login",
	)

	template = strings.ReplaceAll(
		template,
		"{{ url_for('logout') }}",
		"/logout",
	)

	template = strings.ReplaceAll(
		template,
		"{{ url_for('register') }}",
		"/register",
	)

	template = strings.ReplaceAll(
		template,
		"{{ url_for('about') }}",
		"/about",
	)

	template = strings.ReplaceAll(
		template,
		"{{ request.args.get('q', '') }}",
		"{{ query }}",
	)

	template = strings.ReplaceAll(
		template,
		`{{ request.args.get("q", "") }}`,
		"{{ query }}",
	)

	for {
		start := strings.Index(
			template,
			"{% with flashes = get_flashed_messages() %}",
		)

		if start == -1 {
			break
		}

		endRelative := strings.Index(
			template[start:],
			"{% endwith %}",
		)

		if endRelative == -1 {
			break
		}

		end := start + endRelative + len("{% endwith %}")

		template = template[:start] + template[end:]
	}

	return template
}