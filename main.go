package main

import (
	"encoding/json"
	"fmt"
	"istio-client/client"
	"istio-client/istio"
	"os"

	"github.com/gofiber/fiber/v2"
)

var cs = client.GetIstioClient()

func main() {

	test()

	// os exit
	fmt.Printf("\nThis is for dev, exit main.go\n")
	os.Exit(0)
	// go fiber run belew
	app := fiber.New(fiber.Config{
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
		AppName:         "istio-client",
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON("app: test")

	})

	app.Listen("localhost:18086")

}

func test() {

	var vs = &istio.Vs{
		Name:      "nginx-vs",
		Namespace: "shencq",
	}

	v := vs.GetVs(cs)
	vCopy := v.DeepCopy()
	vCopy.ObjectMeta.Annotations = nil
	vCopy.ObjectMeta.ManagedFields = nil
	vCopy.ObjectMeta.SelfLink = ""
	vCopy.ObjectMeta.UID = ""
	vCopy.ObjectMeta.ResourceVersion = ""

	vb, _ := json.Marshal(vCopy)
	fmt.Println(string(vb))

}
