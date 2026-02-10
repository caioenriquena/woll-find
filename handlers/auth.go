package handlers

import (
	"woll-find/database"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"golang.org/x/crypto/bcrypt"
)

var Store *session.Store

func InitSession() {
	Store = session.New()
}

func Home(c *fiber.Ctx) error {
	return c.Render("index", fiber.Map{})
}

func LoginPage(c *fiber.Ctx) error {
	return c.Render("login", fiber.Map{})
}

func RegisterPage(c *fiber.Ctx) error {
	return c.Render("register", fiber.Map{})
}

func Register(c *fiber.Ctx) error {
	type RegisterInput struct {
		Email    string `form:"email"`
		Password string `form:"password"`
	}
	input := new(RegisterInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).SendString("Bad Request")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).SendString("Error hashing password")
	}

	user := database.User{
		Email:        input.Email,
		PasswordHash: string(hash),
	}

	if result := database.DB.Create(&user); result.Error != nil {
		return c.Status(500).SendString("Error creating user")
	}

	return c.Redirect("/login")
}

func Login(c *fiber.Ctx) error {
	type LoginInput struct {
		Email    string `form:"email"`
		Password string `form:"password"`
	}
	input := new(LoginInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).SendString("Bad Request")
	}

	var user database.User
	if result := database.DB.Where("email = ?", input.Email).First(&user); result.Error != nil {
		return c.Status(401).SendString("Invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return c.Status(401).SendString("Invalid credentials")
	}

	sess, err := Store.Get(c)
	if err != nil {
		return err
	}
	sess.Set("user_id", user.ID)
	if err := sess.Save(); err != nil {
		return err
	}

	return c.Redirect("/dashboard")
}

func Logout(c *fiber.Ctx) error {
	sess, err := Store.Get(c)
	if err != nil {
		return err
	}
	if err := sess.Destroy(); err != nil {
		return err
	}
	return c.Redirect("/")
}
