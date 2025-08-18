package main

import (
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load("../env/.env")
}
