package config

import (
	"github.com/joho/godotenv"
	"os"
	"regexp"
)

const projectDirName = "alfred-bot"

func BootstrapEnv(testing bool) {
	var envFileName string
	if testing {
		projectName := regexp.MustCompile(`^(.*` + projectDirName + `)`)
		currentWorkDirectory, _ := os.Getwd()
		rootPath := projectName.Find([]byte(currentWorkDirectory))
		envFileName = string(rootPath) + "/.env.test"
	} else {
		envFileName = ".env"
	}

	err := godotenv.Load(envFileName)
	if err != nil {
		panic(err)
	}
}
