package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"woll-find/database"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

// Dashboard renderiza a página principal com as pastas do usuário
func Dashboard(c *fiber.Ctx) error {
	sess, _ := Store.Get(c)
	userID := sess.Get("user_id")
	if userID == nil {
		return c.Redirect("/login")
	}

	var folders []database.Folder
	database.DB.Where("user_id = ?", userID).Find(&folders)

	return c.Render("dashboard", fiber.Map{
		"Folders": folders,
	})
}

// CreateFolder cria uma nova pasta para o usuário
func CreateFolder(c *fiber.Ctx) error {
	sess, _ := Store.Get(c)
	userID := sess.Get("user_id")
	if userID == nil {
		return c.Status(401).SendString("Não autorizado")
	}

	name := c.FormValue("name")
	folder := database.Folder{
		UserID: userID.(uint),
		Name:   name,
	}
	database.DB.Create(&folder)
	return c.Redirect("/dashboard")
}

// DeleteFolder remove uma pasta e seus arquivos associados
func DeleteFolder(c *fiber.Ctx) error {
	sess, _ := Store.Get(c)
	userID := sess.Get("user_id")
	if userID == nil {
		return c.Status(401).SendString("Não autorizado")
	}

	folderID := c.Params("id")
	var folder database.Folder
	
	// Verifica se a pasta pertence ao usuário antes de deletar
	if err := database.DB.Where("id = ? AND user_id = ?", folderID, userID).First(&folder).Error; err != nil {
		return c.Status(404).SendString("Pasta não encontrada")
	}

	// Deletar arquivos físicos e registros associados seria o ideal aqui.
	// Por simplicidade, deletamos o registro da pasta. 
	// Em produção, usaríamos Transações e deleção em cascata (GORM ou SQL).
	database.DB.Delete(&folder)

	return c.Redirect("/dashboard")
}

// ViewFolder exibe o conteúdo de uma pasta específica
func ViewFolder(c *fiber.Ctx) error {
	sess, _ := Store.Get(c)
	userID := sess.Get("user_id")
	if userID == nil {
		return c.Redirect("/login")
	}

	folderID := c.Params("id")
	var folder database.Folder
	if err := database.DB.Preload("Files").First(&folder, folderID).Error; err != nil {
		return c.Redirect("/dashboard")
	}

	// Verifica propriedade
	if folder.UserID != userID.(uint) {
		return c.Status(403).SendString("Proibido")
	}

	return c.Render("folder", fiber.Map{
		"Folder": folder,
	})
}

// UploadFile lida com o upload de arquivos Excel
func UploadFile(c *fiber.Ctx) error {
	folderID, _ := strconv.Atoi(c.FormValue("folder_id"))

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).SendString("Erro no upload")
	}

	// Salva o arquivo
	// Garante que o diretório existe
	if _, err := os.Stat("./public/uploads"); os.IsNotExist(err) {
		os.MkdirAll("./public/uploads", 0755)
	}

	filename := fmt.Sprintf("./public/uploads/%s", file.Filename)
	if err := c.SaveFile(file, filename); err != nil {
		return c.Status(500).SendString("Erro ao salvar arquivo")
	}

	// Registro no Banco de Dados
	dbFile := database.File{
		FolderID: uint(folderID),
		FileName: file.Filename,
		FilePath: filename,
		Status:   "processing",
	}
	database.DB.Create(&dbFile)

	// Processamento Assíncrono (Goroutine)
	go processExcel(dbFile.ID, filename)

	return c.Redirect(fmt.Sprintf("/folder/%d", folderID))
}

// processExcel lê o arquivo Excel e indexa os dados
func processExcel(fileID uint, path string) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		log.Println("Erro ao abrir arquivo:", err)
		database.DB.Model(&database.File{}).Where("id = ?", fileID).Update("status", "error")
		return
	}
	defer f.Close()

	const batchSize = 500
	var batchData []database.SheetData
	var batchSearchable []string

	// Função auxiliar para inserir o lote atual
	flushBatch := func() {
		if len(batchData) == 0 {
			return
		}

		tx := database.DB.Begin()
		if tx.Error != nil {
			log.Println("Erro ao iniciar transação:", tx.Error)
			return
		}

		// 1. Insere em SheetData (GORM preenche os IDs automaticamente)
		if err := tx.Create(&batchData).Error; err != nil {
			tx.Rollback()
			log.Println("Erro ao inserir lote em SheetData:", err)
			return
		}

		// 2. Insere no FTS
		for i, data := range batchData {
			// rowid DEVE corresponder ao sheetData.ID
			// Armazenamos file_id como UNINDEXED para filtragem rápida
			err = tx.Exec("INSERT INTO sheet_fts(rowid, content, file_id) VALUES(?, ?, ?)", data.ID, batchSearchable[i], fileID).Error
			if err != nil {
				log.Println("Erro ao indexar linha no FTS:", err)
				// Não damos rollback aqui para não perder os dados já salvos, 
				// mas idealmente deveria ser atômico.
				// Em caso de erro crítico no FTS, a busca ficará incompleta.
			}
		}

		if err := tx.Commit().Error; err != nil {
			log.Println("Erro ao commitar transação:", err)
		}

		// Limpa os slices mantendo a capacidade
		batchData = batchData[:0]
		batchSearchable = batchSearchable[:0]
	}

	// Itera sobre todas as planilhas (sheets)
	for _, sheet := range f.GetSheetList() {
		// Substituído GetRows por Rows (iterador/streaming) para economizar memória
		rows, err := f.Rows(sheet)
		if err != nil {
			log.Println("Erro ao obter linhas da planilha:", err)
			continue
		}

		for rows.Next() {
			row, err := rows.Columns()
			if err != nil {
				log.Println("Erro ao ler colunas:", err)
				continue
			}

			// Ignora linhas vazias
			if len(row) == 0 {
				continue
			}
			
			// Verifica se a linha tem algum conteúdo
			hasContent := false
			for _, cell := range row {
				if strings.TrimSpace(cell) != "" {
					hasContent = true
					break
				}
			}
			if !hasContent {
				continue
			}

			// Concatena conteúdo da linha para busca
			searchable := strings.Join(row, " ")
			jsonContent, _ := json.Marshal(row)

			// Adiciona ao lote
			batchData = append(batchData, database.SheetData{
				FileID:     fileID,
				RawContent: string(jsonContent),
			})
			batchSearchable = append(batchSearchable, searchable)

			// Se atingiu o tamanho do lote, insere no banco
			if len(batchData) >= batchSize {
				flushBatch()
			}
		}
		if err = rows.Close(); err != nil {
			log.Println("Erro ao fechar iterador de linhas:", err)
		}
	}

	// Insere qualquer dado restante
	flushBatch()

	database.DB.Model(&database.File{}).Where("id = ?", fileID).Update("status", "indexed")
}
