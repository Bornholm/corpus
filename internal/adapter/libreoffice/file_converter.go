package libreoffice

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bornholm/corpus/internal/adapter/pandoc"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

type FileConverter struct {
	pandoc *pandoc.FileConverter
}

// Convert implements port.FileConverter.
func (f *FileConverter) Convert(ctx context.Context, filename string, r io.Reader) (io.ReadCloser, error) {
	if filepath.Ext(filename) != ".doc" {
		return f.pandoc.Convert(ctx, filename, r)
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "corpus-*")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer os.RemoveAll(tempDir)

	source := filepath.Join(tempDir, "file.doc")
	target := filepath.Join(tempDir, "file.docx")

	copy, err := os.Create(source)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if _, err := io.Copy(copy, r); err != nil {
		return nil, errors.WithStack(err)
	}

	cmd := exec.Command("libreoffice", "--headless", "--convert-to", "docx", source, "--outdir", tempDir)

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return nil, errors.WithStack(err)
	}

	docx, err := os.Open(target)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer docx.Close()

	filename = strings.TrimSuffix(filename, ".doc")

	return f.pandoc.Convert(ctx, filename+".docx", docx)
}

// SupportedExtensions implements port.FileConverter.
func (f *FileConverter) SupportedExtensions() []string {
	return append(f.pandoc.SupportedExtensions(), ".doc")
}

func NewFileConverter(pandoc *pandoc.FileConverter) *FileConverter {
	return &FileConverter{
		pandoc: pandoc,
	}
}

var _ port.FileConverter = &FileConverter{}
