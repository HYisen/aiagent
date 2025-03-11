package main

import (
	"aiagent/clients/model"
	"gorm.io/gen"
)

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath:      "./clients/query",
		ModelPkgPath: "",
		Mode:         gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface,
	})
	g.ApplyBasic(model.Session{})

	g.ApplyBasic(model.Chat{}, model.Result{})
	g.Execute()
}
