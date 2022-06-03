package main

import (
	"errors"
	"os"
	"path/filepath"
)

func readNoticeFile(stateDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, "notice.html"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		} else {
			return "", err
		}
	}

	return string(data), nil
}
