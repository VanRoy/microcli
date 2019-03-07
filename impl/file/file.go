package file

import (
	"os"
	"path/filepath"
)

func Exist(path string) bool {

	localPath := Rel(path)

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return false
	} else if err != nil {
		return false
	} else {
		return true
	}
}

func Rel(path string) string {

	currentDir, err := os.Getwd()
	if err != nil {
		//MESSAGE
		return ""
	}

	return filepath.Join(currentDir, path)
}
