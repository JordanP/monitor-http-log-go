package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

func readlines(ctx context.Context, filepath string, lines chan<- string) error {
	defer close(lines)

	file, err := os.Open(filepath)
	if err != nil {
		return errors.Wrap(err, "failed to follow file")
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()

	if err := watcher.Add(file.Name()); err != nil {
		return err
	}

	r := bufio.NewReader(file)
	for {
		// We have reached EOF, wait for a write event
		if err := waitForWriteEvent(ctx, watcher); err != nil {
			return err
		}

		by, err := r.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return err
		}
		// From now on, err is either nil or is io.EOF
		lines <- strings.TrimRight(string(by), "\n")
	}
}

func waitForWriteEvent(ctx context.Context, w *fsnotify.Watcher) error {
	for {
		select {
		case err := <-w.Errors:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case event := <-w.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				return nil
			}

		}
	}
}
