package main

import (
	"fmt"
	"os"

	"github.com/YuarenArt/marketgo/internal/app_cmd"

	"log"

	"github.com/YuarenArt/marketgo/internal/config"
	"github.com/YuarenArt/marketgo/pkg/logging"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading .env")
	}
	cfg := config.NewConfig()
	appLogger := logging.NewLogger(cfg)

	if err := app_cmd.NewApp(appLogger, cfg).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
}
