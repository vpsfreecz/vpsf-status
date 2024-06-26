package main

import (
	"errors"
	"html/template"
	"log"
	"os"
	"time"
)

type Notice struct {
	Any       bool
	Html      template.HTML
	UpdatedAt time.Time
}

func readNoticeFile(path string) (Notice, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Notice{}, nil
		} else {
			return Notice{}, err
		}
	}

	n := Notice{
		Any:  true,
		Html: template.HTML(data),
	}

	info, err := os.Stat(path)
	if err != nil {
		log.Printf("Unable to stat notice at %v: %+v", path, err)
		return n, nil
	}

	n.UpdatedAt = info.ModTime()

	return n, nil
}

func checkNoticeFile(st *Status, path string, checkInterval time.Duration) {
	for {
		notice, err := readNoticeFile(path)

		if err == nil && notice.Any {
			st.Exporter.notice.Set(1)
		} else {
			st.Exporter.notice.Set(0)
		}

		time.Sleep(checkInterval)
	}
}
