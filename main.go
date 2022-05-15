package main

import (
	"encoding/json"
	"fmt"
	"istio-client/client"
	"istio-client/istio"
	tools "istio-client/utils"
	"log"
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
		AppName:   "nginx",
		Version:   "canary-v0.0.1",
	}

	// get
	// vs.Http.Name = "test"

	// m := vs.GetVsRule(cs)

	// mStr, _ := json.Marshal(m)
	// fmt.Printf("vs http index =%d, and match = %s\n", vs.Http.Index, string(mStr))

	vs.Version = "canary-v0.0.1"
	rName := vs.AppName + "-" + tools.ReplaceVersion(vs.Version)
	vs.VirtualService = vs.GetVs(cs)

	// get match
	// j := vs.VirtualService.Spec.Http[0].Match
	// str, _ := json.Marshal(j)
	// fmt.Println(string(str))
	// os.Exit(0)

	// del
	// vs.VertualService = vs.GetVs(cs)
	// vsNew, err := vs.DelVsRule(cs)

	// if err == nil {
	// 	for _, n := range vsNew.Spec.Http {
	// 		fmt.Printf("routerName = %s\t", n.Name)
	// 	}
	// }

	// update

	updateJson := `
	[
		{
        "headers": {
            "x-weike-forward": {
                "exact": "canary-v0.0.1"
                }
            },
		"uri": {
			"prefix": "/canary-v1"
			}
        },
		{
	     	"headers": {
				"user-id": {
					"regex": "^(10323.*|10324.*)$"
				}
			}
	    }
	]`

	err := json.Unmarshal([]byte(updateJson), &vs.HttpMatch)
	if err != nil {
		log.Printf("Unmarshal updateJson err: %v", err)
		os.Exit(1)
	}
	fmt.Printf("%v", vs.HttpMatch)

	vs.CanaryWeight = 82
	vsTarg := vs.UpdateVsRule(cs, rName)
	if vsTarg == nil {
		log.Panicln("update vs failed, please check")
	}
}
