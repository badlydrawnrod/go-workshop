package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	filename := filepath.Join(dir, name)
	if err := os.WriteFile(filename, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestCatFile(t *testing.T) {
	dir := t.TempDir()
	want := "hello"
	filename := writeTestFile(t, dir, "hello", want)

	var got bytes.Buffer
	err := catFile(filename, &got, func(w io.Writer, r io.Reader) error {
		_, err := io.Copy(w, r)
		return err
	})
	if err != nil {
		t.Fatalf("catFile(%q): %v", filename, err)
	}

	if got.String() != want {
		t.Errorf("unexpected output: got %q, want %q", got.String(), want)
	}
}

func TestCatFileCopyError(t *testing.T) {
	filename := writeTestFile(t, t.TempDir(), "file", "hello")
	wantErr := errors.New("copy failed")
	err := catFile(filename, io.Discard, func(io.Writer, io.Reader) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("catFile() error = %v; want %v", err, wantErr)
	}
	if err != nil && !strings.Contains(err.Error(), filename) {
		t.Errorf("catFile() error = %q; want it to mention %q", err, filename)
	}
}

func TestCatFiles(t *testing.T) {
	dir := t.TempDir()
	fileA := writeTestFile(t, dir, "file_a", "hello")
	fileB := writeTestFile(t, dir, "file_b", "\nthis is a test\n")

	var got bytes.Buffer
	var outErr bytes.Buffer
	files := []string{fileA, "-", fileB}
	err := catFiles(files, &got, strings.NewReader(" from stdin"), &outErr, func(w io.Writer, r io.Reader) error {
		_, err := io.Copy(w, r)
		return err
	})
	if err != nil {
		t.Fatalf("catFiles(%q): %v", files, err)
	}

	want := "hello from stdin\nthis is a test\n"
	if got.String() != want {
		t.Errorf("output = %q; want %q", got.String(), want)
	}
	if outErr.Len() != 0 {
		t.Errorf("stderr = %q; want empty", outErr.String())
	}
}

func TestCatFilesWithMissingFile(t *testing.T) {
	dir := t.TempDir()
	fileA := writeTestFile(t, dir, "file_a", "hello")
	missing := filepath.Join(dir, "missing")
	fileB := writeTestFile(t, dir, "file_b", "\nthis is a test\n")

	var got bytes.Buffer
	var outErr bytes.Buffer
	files := []string{fileA, missing, fileB}
	err := catFiles(files, &got, strings.NewReader(""), &outErr, func(w io.Writer, r io.Reader) error {
		_, err := io.Copy(w, r)
		return err
	})
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("catFiles() error = %v; want an error matching %v", err, os.ErrNotExist)
	}

	want := "hello\nthis is a test\n"
	if got.String() != want {
		t.Errorf("output = %q; want %q", got.String(), want)
	}
	if !strings.Contains(outErr.String(), missing) {
		t.Errorf("stderr = %q; want it to mention %q", outErr.String(), missing)
	}
}

func TestCatFilesWithDirectory(t *testing.T) {
	dir := t.TempDir()
	fileA := writeTestFile(t, dir, "file_a", "hello")
	directory := filepath.Join(dir, "directory")
	fileB := writeTestFile(t, dir, "file_b", "\nthis is a test\n")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Fatal(err)
	}

	var got bytes.Buffer
	var outErr bytes.Buffer
	files := []string{fileA, directory, fileB}
	err := catFiles(files, &got, strings.NewReader(""), &outErr, func(w io.Writer, r io.Reader) error {
		_, err := io.Copy(w, r)
		return err
	})
	if err == nil {
		t.Error("catFiles() error = nil; want a directory error")
	} else if !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("catFiles() error = %q; want a directory error", err)
	}

	want := "hello\nthis is a test\n"
	if got.String() != want {
		t.Errorf("output = %q; want %q", got.String(), want)
	}
	if !strings.Contains(outErr.String(), directory) {
		t.Errorf("stderr = %q; want it to mention %q", outErr.String(), directory)
	}
}

func TestLineNumberer(t *testing.T) {
	tests := []struct {
		name            string
		inputs          []string
		countEmptyLines bool
		want            string
	}{
		{
			name:            "number all lines",
			inputs:          []string{"one\n\ntwo\n"},
			countEmptyLines: true,
			want:            "     1\tone\n     2\t\n     3\ttwo\n",
		},
		{
			name:   "skip empty lines",
			inputs: []string{"one\n\ntwo\n"},
			want:   "     1\tone\n\n     2\ttwo\n",
		},
		{
			name:   "final line without newline",
			inputs: []string{"one\ntwo"},
			want:   "     1\tone\n     2\ttwo",
		},
		{
			name:   "single line without newline",
			inputs: []string{"one"},
			want:   "     1\tone",
		},
		{
			name:   "empty input",
			inputs: []string{""},
			want:   "",
		},
		{
			name:   "join inputs",
			inputs: []string{"one", "\ntwo\n"},
			want:   "     1\tone\n     2\ttwo\n",
		},
		{
			name:   "skip empty line after joined input",
			inputs: []string{"one", "\n\ntwo\n"},
			want:   "     1\tone\n\n     2\ttwo\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := lineNumberer{
				countEmptyLines: tt.countEmptyLines,
			}
			var got bytes.Buffer

			for _, input := range tt.inputs {
				if err := n.copy(&got, strings.NewReader(input)); err != nil {
					t.Fatalf("copy(%q): %v", input, err)
				}
			}
			if got.String() != tt.want {
				t.Errorf("copy output = %q; want %q", got.String(), tt.want)
			}
		})
	}
}

type errorReader struct {
	data string
	err  error
}

func (r *errorReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) > 0 {
		return n, nil
	}
	return n, r.err
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestLineNumbererReadError(t *testing.T) {
	wantErr := errors.New("read failed")
	n := lineNumberer{}
	var got bytes.Buffer
	err := n.copy(&got, &errorReader{data: "partial", err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Errorf("copy() error = %v; want %v", err, wantErr)
	}
	if want := "     1\tpartial"; got.String() != want {
		t.Errorf("copy output = %q; want %q", got.String(), want)
	}
}

func TestLineNumbererDataWithReadError(t *testing.T) {
	wantErr := errors.New("read failed")
	tests := []struct {
		name    string
		readErr error
		wantErr error
	}{
		{name: "EOF", readErr: io.EOF},
		{name: "read error", readErr: wantErr, wantErr: wantErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := lineNumberer{}
			var got bytes.Buffer
			err := n.copy(&got, &errorReader{data: "one\ntwo\nthree", err: tt.readErr})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("copy() error = %v; want %v", err, tt.wantErr)
			}
			if want := "     1\tone\n     2\ttwo\n     3\tthree"; got.String() != want {
				t.Errorf("copy output = %q; want %q", got.String(), want)
			}
		})
	}
}

func TestLineNumbererWriteError(t *testing.T) {
	wantErr := errors.New("write failed")
	n := lineNumberer{}
	if err := n.copy(errorWriter{wantErr}, strings.NewReader("one\n")); !errors.Is(err, wantErr) {
		t.Errorf("copy() error = %v; want %v", err, wantErr)
	}
}

func TestRun(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantOutput string
		wantStatus int
		wantStderr string
	}{
		{name: "default stdin", stdin: "hello\n", wantOutput: "hello\n"},
		{name: "b overrides n", args: []string{"-n", "-b"}, stdin: "one\n\ntwo\n", wantOutput: "     1\tone\n\n     2\ttwo\n"},
		{name: "missing file", args: []string{missing}, wantStatus: 1, wantStderr: missing},
		{name: "help", args: []string{"-h"}, wantStderr: "Usage: cat"},
		{name: "invalid flag", args: []string{"-unknown"}, wantStatus: 2, wantStderr: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := run("cat", tt.args, &stdout, strings.NewReader(tt.stdin), &stderr)
			if status != tt.wantStatus {
				t.Errorf("exit status = %d; want %d (stderr: %q)", status, tt.wantStatus, stderr.String())
			}
			if stdout.String() != tt.wantOutput {
				t.Errorf("stdout = %q; want %q", stdout.String(), tt.wantOutput)
			}
			if tt.wantStderr == "" && stderr.Len() != 0 || tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q; want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
