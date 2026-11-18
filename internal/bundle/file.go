package bundle

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
)

func readFile(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrRead
	}
	if info.Size() >= MaxArchiveBytes {
		return nil, ErrBounds
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, ErrRead
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrRead
	}
	if info.Size() >= MaxArchiveBytes {
		return nil, ErrBounds
	}
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, input: file}, MaxArchiveBytes))
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if err != nil {
		return nil, ErrRead
	}
	if len(data) >= MaxArchiveBytes {
		return nil, ErrBounds
	}
	return data, nil
}

func Load(ctx context.Context, path string) (Capture, error) {
	data, err := readFile(ctx, path)
	if err != nil {
		return Capture{}, err
	}
	return Read(ctx, data)
}

func regularDestination(path string) bool {
	info, err := os.Lstat(path)
	return os.IsNotExist(err) || err == nil && info.Mode().IsRegular()
}

// Publish stages/validates one private file beside the destination. Only the
// final rename changes the destination. Every earlier failure/cancellation
// keeps the previous regular file and removes only this invocation's temp.
func Publish(ctx context.Context, path string, data []byte) error {
	return publish(ctx, path, data, publicationSteps{
		protect: privateFile,
		write:   func(file *os.File, data []byte) (int, error) { return file.Write(data) },
		sync:    func(file *os.File) error { return file.Sync() },
		close:   func(file *os.File) error { return file.Close() },
		read:    readFile, replace: replaceFile,
	})
}

type publicationSteps struct {
	protect func(*os.File) error
	write   func(*os.File, []byte) (int, error)
	sync    func(*os.File) error
	close   func(*os.File) error
	read    func(context.Context, string) ([]byte, error)
	replace func(string, string) error
}

func publish(ctx context.Context, path string, data []byte, steps publicationSteps) error {
	if _, err := Read(ctx, data); err != nil {
		return err
	}
	if path == "" || !regularDestination(path) {
		return ErrWrite
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".envdoctor-bundle-*.tmp")
	if err != nil {
		return ErrWrite
	}
	temp := file.Name()
	defer func() { _ = file.Close(); _ = os.Remove(temp) }()
	if err := steps.protect(file); err != nil {
		return ErrWrite
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if n, err := steps.write(file, data); err != nil || n != len(data) {
		return ErrWrite
	}
	if err := steps.sync(file); err != nil {
		return ErrWrite
	}
	if err := steps.close(file); err != nil {
		return ErrWrite
	}
	staged, err := steps.read(ctx, temp)
	if err != nil {
		return err
	}
	if !bytes.Equal(staged, data) {
		return ErrIntegrity
	}
	if _, err := Read(ctx, staged); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !regularDestination(path) {
		return ErrWrite
	}
	if err := steps.replace(temp, path); err != nil {
		return ErrWrite
	}
	return nil
}
