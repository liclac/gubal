package main

import (
	"github.com/liclac/gubal/cmd"

	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattes/migrate/database/postgres"
	_ "github.com/mattes/migrate/source/file"
)

//go:generate go generate ./models

func main() {
	cmd.Execute()
}
