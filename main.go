package main

import "github.com/gofiber/fiber/v2"

func main() {
	app := fiber.New(fiber.Config{
		ReadBufferSize:  8092,
		WriteBufferSize: 8092,
		AppName:         "istio-client",
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON("app: test")

	})

	app.Listen("localhost:18086")

}
