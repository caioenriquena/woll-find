package database

import (
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

type User struct {
	gorm.Model
	Email        string `gorm:"uniqueIndex"`
	PasswordHash string
	Folders      []Folder
}

type Folder struct {
	gorm.Model
	UserID uint
	Name   string
	Files  []File
}

type File struct {
	gorm.Model
	FolderID uint
	FileName string
	FilePath string
	Status   string // "indexed" (indexado), "processing" (processando), "error" (erro)
}

type SheetData struct {
	gorm.Model
	FileID     uint
	RawContent string // Representação JSON da linha
}

func Connect() {
	var err error
	
	// Define o caminho do banco de dados (padrão ou via ENV)
	dbName := "wollfind.db"
	dbPath := os.Getenv("DB_PATH") // Ex: /app/database/wollfind.db
	
	if dbPath == "" {
		dbPath = dbName
	} else {
		// Garante que o diretório existe
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatal("Falha ao criar diretório do banco:", err)
		}
	}

	// Habilita chaves estrangeiras e otimizações de performance
	// _journal_mode=WAL: Permite leitura e escrita simultâneas
	// _synchronous=NORMAL: Reduz operações de fsync (mais rápido, seguro em WAL)
	// _busy_timeout=5000: Espera até 5s se o banco estiver bloqueado
	dsn := dbPath + "?_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"
	
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("Falha ao conectar ao banco de dados:", err)
	}

	log.Println("Conectado ao banco de dados")

	// Garante as configurações via PRAGMA também
	DB.Exec("PRAGMA journal_mode = WAL;")
	DB.Exec("PRAGMA synchronous = NORMAL;")
	DB.Exec("PRAGMA temp_store = MEMORY;") // Armazena tabelas temporárias na RAM
	DB.Exec("PRAGMA mmap_size = 30000000000;") // Usa Memory-Mapped I/O para leitura rápida

	// Migração Automática das Tabelas Padrão
	err = DB.AutoMigrate(&User{}, &Folder{}, &File{}, &SheetData{})
	if err != nil {
		log.Fatal("Falha ao migrar banco de dados:", err)
	}

	// Cria Tabela Virtual FTS5 se não existir
	// Usamos 'content' para armazenar o texto pesquisável
	// Vinculamos ao SheetData via rowid (docid)
	
	// Verifica se a tabela existe
	if !DB.Migrator().HasTable("sheet_fts") {
		err := DB.Exec("CREATE VIRTUAL TABLE sheet_fts USING fts5(content, file_id UNINDEXED)").Error
		if err != nil {
			log.Fatal("Falha ao criar tabela FTS5:", err)
		}
		log.Println("Tabela FTS5 criada")
	}
}
