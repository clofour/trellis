package runtime

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"time"
)

func newLogReader(ctx context.Context, file *os.File, follow bool, tail int) (io.ReadCloser, error) {
	if tail > 0 {
		data, err := io.ReadAll(file)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		lines := bytes.Split(data, []byte("\n"))
		start := len(lines) - tail - 1
		if start < 0 {
			start = 0
		}
		if _, err := file.Seek(int64(len(data)-len(bytes.Join(lines[start:], []byte("\n")))), io.SeekStart); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	if !follow {
		return file, nil
	}
	reader, writer := io.Pipe()
	go func() {
		defer file.Close()
		defer writer.Close()
		buf := bufio.NewReader(file)
		for {
			chunk, err := buf.ReadBytes('\n')
			if len(chunk) > 0 {
				if _, werr := writer.Write(chunk); werr != nil {
					return
				}
			}
			if err == nil {
				continue
			}
			if err != io.EOF {
				_ = writer.CloseWithError(err)
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(200 * time.Millisecond):
			}
		}
	}()
	return reader, nil
}
