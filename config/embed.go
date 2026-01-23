package config

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"strings"
)

var embeddedFS embed.FS

func LoadEmbeddedEnv() {
	data, err := embeddedFS.ReadFile(".env")
	if err != nil {
		log.Println("No embedded .env file found")
		return
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, `"'`)
			os.Setenv(key, value)
		}
	}
}

func SetEmbeddedFS(fs embed.FS) {
	embeddedFS = fs
}

func LoadEmbeddedIcon(filename string) ([]byte, error) {
	data, err := embeddedFS.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fs.ErrInvalid
	}
	return data, nil
}
