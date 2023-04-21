package main

import "github.com/argus-labs/we-sdk/pkg"

func main() {
	cfg, err := pkg.LoadConfig("example")
	if err != nil {
		panic(err)
	}
	app := pkg.NewApplication(cfg)
	err = app.Start()
	if err != nil {
		panic(err)
	}
}