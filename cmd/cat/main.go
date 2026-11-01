package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

type copyFunc func(io.Writer, io.Reader) error

type lineNumberer struct {
	lineNumber      int
	inLine          bool
	countEmptyLines bool
}

func main() {
	os.Exit(run(os.Args[0], os.Args[1:], os.Stdout, os.Stdin, os.Stderr))
}

func run(name string, args []string, out io.Writer, in io.Reader, outErr io.Writer) int {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(outErr)
	useNumbers := flags.Bool("n", false, "number all output lines")
	dontNumberEmptyLines := flags.Bool("b", false, "number non-empty output lines, overrides -n")
	flags.Usage = func() {
		fmt.Fprintf(outErr, "Usage: %s [flags] FILE...\n\n", filepath.Base(name))
		fmt.Fprintln(outErr, "If FILE is omitted or is -, read from standard input.")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	files := flags.Args()

	if len(files) == 0 {
		files = append(files, "-")
	}

	var copyFn copyFunc
	if *useNumbers || *dontNumberEmptyLines {
		numberer := lineNumberer{
			countEmptyLines: !*dontNumberEmptyLines,
		}
		copyFn = numberer.copy
	} else {
		copyFn = func(w io.Writer, r io.Reader) error {
			_, err := io.Copy(w, r)
			return err
		}
	}
	if err := catFiles(files, out, in, outErr, copyFn); err != nil {
		return 1
	}
	return 0
}

func catFiles(filenames []string, out io.Writer, in io.Reader, outErr io.Writer, copyFn copyFunc) error {
	var result error
	for _, filename := range filenames {
		var err error

		if filename == "-" {
			err = copyFn(out, in)
		} else {
			err = catFile(filename, out, copyFn)
		}

		if err != nil {
			fmt.Fprintln(outErr, err)
			result = err
		}
	}
	return result
}

func catFile(filename string, w io.Writer, copyFn copyFunc) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", filename, err)
	}

	if info.IsDir() {
		return fmt.Errorf("%s: is a directory", filename)
	}

	if err := copyFn(w, f); err != nil {
		return fmt.Errorf("copying %s: %w", filename, err)
	}

	return nil
}

func (n *lineNumberer) copy(w io.Writer, r io.Reader) error {
	buf := make([]byte, 1024)
	for {
		bytesRead, err := r.Read(buf)

		for s := buf[:bytesRead]; len(s) > 0; {
			if !n.inLine && (n.countEmptyLines || s[0] != '\n') {
				n.lineNumber++
				if _, err := fmt.Fprintf(w, "%6d\t", n.lineNumber); err != nil {
					return err
				}
			}

			toWrite := len(s)
			if newline := slices.Index(s, '\n'); newline != -1 {
				toWrite = newline + 1
				n.inLine = false
			} else {
				n.inLine = true
			}

			written, err := w.Write(s[:toWrite])
			if err != nil {
				return err
			}
			s = s[written:]
		}

		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}
	}
}
