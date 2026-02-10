package main

import (
	"html/template"
	"log"
	"woll-find/database"
	"woll-find/handlers"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/template/html/v2"
)

func main() {
	// Inicializa Banco de Dados
	database.Connect()
	handlers.InitSession()

	// Inicializa Motor de Templates
	engine := html.New("./views", ".html")
	engine.AddFunc("safeHTML", func(s string) template.HTML {
		return template.HTML(s)
	})

	// Inicializa Fiber
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Middleware de Compressão (Gzip/Brotli/Deflate)
	// Nível de compressão padrão equilibra velocidade e tamanho
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // Prioriza velocidade para resposta instantânea
	}))

	// Arquivos Estáticos
	app.Static("/public", "./public")

	// Rotas
	app.Get("/", handlers.Home)
	
	// Autenticação
	app.Get("/login", handlers.LoginPage)
	app.Post("/login", handlers.Login)
	app.Get("/register", handlers.RegisterPage)
	app.Post("/register", handlers.Register)
	app.Get("/logout", handlers.Logout)
	
	// Aplicação Principal
	app.Get("/dashboard", handlers.Dashboard)
	app.Post("/folder", handlers.CreateFolder)
	app.Post("/folder/:id/delete", handlers.DeleteFolder) // Rota adicionada para deletar pasta
	app.Post("/upload", handlers.UploadFile)
	app.Get("/folder/:id", handlers.ViewFolder)
	
	// Chat e Busca
	app.Get("/chat", handlers.ChatPage)
	app.Post("/chat/search", handlers.ChatSearch)

	log.Fatal(app.Listen(":3000"))
}
