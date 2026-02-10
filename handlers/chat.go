package handlers

import (
	"strings"
	"woll-find/database"

	"github.com/gofiber/fiber/v2"
)

// ChatPage renderiza a interface principal de chat com a sidebar de pastas
func ChatPage(c *fiber.Ctx) error {
	sess, _ := Store.Get(c)
	userID := sess.Get("user_id")
	if userID == nil {
		return c.Redirect("/login")
	}

	// Carrega pastas e arquivos para a barra lateral
	var folders []database.Folder
	database.DB.Preload("Files").Where("user_id = ?", userID).Find(&folders)

	return c.Render("chat", fiber.Map{
		"Folders": folders,
	})
}

// ChatSearch executa a busca Full Text Search (FTS5) e retorna fragmentos HTML
func ChatSearch(c *fiber.Ctx) error {
	sess, _ := Store.Get(c)
	userID := sess.Get("user_id")
	if userID == nil {
		return c.Status(401).SendString("Não autorizado")
	}

	query := c.FormValue("query")
	if query == "" {
		return c.SendString("")
	}

	// Otimização para busca "as-you-type" (como solicitado: MATCH 'termo*')
	// Se a query não termina com espaço ou *, adicionamos * para buscar prefixo
	// Isso permite que "fina" encontre "financeiro"
	trimmedQuery := strings.TrimSpace(query)
	if len(trimmedQuery) > 0 && !strings.HasSuffix(trimmedQuery, "*") {
		// Se for composto (ex: "relatorio fin"), adicionamos * ao final: "relatorio fin*"
		// Mas para FTS5 seguro, é bom garantir que " aspas não quebrem a query.
		// Vamos fazer uma higienização básica e adicionar o *
		
		// Remove caracteres perigosos para FTS simples
		safeQuery := strings.ReplaceAll(trimmedQuery, "\"", "")
		safeQuery = strings.ReplaceAll(safeQuery, "'", "")
		
		// Adiciona * ao final da última palavra
		query = safeQuery + "*"
	}
	
	type Result struct {
		FileName   string
		Snippet    string
		RawContent string
	}

	var results []Result

	// Usando função snippet() do SQLite para destacar os termos encontrados
	// snippet(nome_tabela, indice_coluna, tag_inicio, tag_fim, elipsis, max_tokens)
	sql := `
        SELECT 
            files.file_name,
            snippet(sheet_fts, 0, '<mark class="bg-yellow-200 text-blue-800 rounded px-1">', '</mark>', '...', 64) as snippet,
            sheet_data.raw_content
        FROM sheet_fts
        JOIN sheet_data ON sheet_data.id = sheet_fts.rowid
        JOIN files ON files.id = sheet_data.file_id
        JOIN folders ON folders.id = files.folder_id
        WHERE folders.user_id = ? AND sheet_fts MATCH ?
        ORDER BY rank
        LIMIT 50
    `

	if err := database.DB.Raw(sql, userID, query).Scan(&results).Error; err != nil {
		return c.SendString(`<div class="p-4 text-red-500">Erro na busca (tente termos mais simples): ` + err.Error() + `</div>`)
	}

	if len(results) == 0 {
		return c.SendString(`<div class="p-4 text-gray-500 text-center">Nenhum resultado encontrado para "` + query + `"</div>`)
	}

	return c.Render("partials/search_results", fiber.Map{
		"Results": results,
	})
}
