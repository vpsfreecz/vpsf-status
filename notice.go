package main

import (
	"errors"
	"os"
)

func readNoticeFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		} else {
			return "", err
		}
	}

	return string(data), nil
}
