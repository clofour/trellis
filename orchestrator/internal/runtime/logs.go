package runtime

import (
	"bufio"
	"context"
	"io"
	"os"
	"time"
)

func newLogReader(ctx context.Context, file *os.File, follow bool, tail int) (io.ReadCloser, error) {
	if tail > 0 {
		start, err := tailOffset(file, tail)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		if _, err := file.Seek(start, io.SeekStart); err != nil {
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
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
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
			case <-ticker.C:
			}
		}
	}()
	return reader, nil
}

func tailOffset(file *os.File, tail int) (int64, error) {
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	position := info.Size()
	lines := 0
	buffer := make([]byte, 32*1024)
	for position > 0 {
		start := max(int64(0), position-int64(len(buffer)))
		n, err := file.ReadAt(buffer[:position-start], start)
		if err != nil && err != io.EOF {
			return 0, err
		}
		for i := n - 1; i >= 0; i-- {
			if buffer[i] == '\n' && start+int64(i) < info.Size()-1 {
				lines++
				if lines == tail {
					return start + int64(i) + 1, nil
				}
			}
		}
		position = start
	}
	return 0, nil
}
