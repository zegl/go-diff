package diff

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

type PrintFileDiffOptions struct {
	quoteNames bool
}

type PrintFileDiffOption func(*PrintFileDiffOptions)

func WithQuotedNames() PrintFileDiffOption {
	return func(opts *PrintFileDiffOptions) {
		opts.quoteNames = true
	}
}

func getOptions(opts ...PrintFileDiffOption) *PrintFileDiffOptions {
	options := &PrintFileDiffOptions{}
	for _, applyOption := range opts {
		applyOption(options)
	}
	return options
}

// PrintMultiFileDiff prints a multi-file diff in unified diff format.
func PrintMultiFileDiff(ds []*FileDiff, options ...PrintFileDiffOption) ([]byte, error) {
	var buf bytes.Buffer
	for _, d := range ds {
		diff, err := PrintFileDiff(d, options...)
		if err != nil {
			return nil, err
		}
		if _, err := buf.Write(diff); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// PrintFileDiff prints a FileDiff in unified diff format.
//
// TODO(sqs): handle escaping whitespace/etc. chars in filenames
func PrintFileDiff(d *FileDiff, options ...PrintFileDiffOption) ([]byte, error) {
	opts := getOptions(options...)
	var buf bytes.Buffer

	for _, xheader := range d.Extended {
		if opts.quoteNames {
			if err := printQuotedXheader(&buf, d, xheader); err != nil {
				return nil, err
			}
			continue
		}

		if _, err := fmt.Fprintln(&buf, xheader); err != nil {
			return nil, err
		}
	}

	// FileDiff is added/deleted file
	// No further hunks printing needed
	if d.NewName == "" {
		_, err := fmt.Fprintf(&buf, onlyInMessage, filepath.Dir(d.OrigName), filepath.Base(d.OrigName))
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	if d.Hunks == nil {
		return buf.Bytes(), nil
	}

	if err := printFileHeader(&buf, "--- ", d.OrigName, d.OrigTime, opts.quoteNames); err != nil {
		return nil, err
	}
	if err := printFileHeader(&buf, "+++ ", d.NewName, d.NewTime, opts.quoteNames); err != nil {
		return nil, err
	}

	ph, err := PrintHunks(d.Hunks)
	if err != nil {
		return nil, err
	}

	if _, err := buf.Write(ph); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func printQuotedXheader(buf io.Writer, d *FileDiff, xheader string) error {
	// Print quoted "diff --git" lines
	if strings.HasPrefix(xheader, "diff --git") {
		if _, err := fmt.Fprintf(buf, "diff --git %s %s\n", quote(d.OrigName), quote(d.NewName)); err != nil {
			return err
		}
		return nil
	}

	// rename from
	if strings.HasPrefix(xheader, "rename from ") {
		rem := xheader[len("rename from "):]
		if _, err := fmt.Fprintf(buf, "rename from %s\n", quote(rem)); err != nil {
			return err
		}
		return nil
	}

	// rename to
	if strings.HasPrefix(xheader, "rename to ") {
		rem := xheader[len("rename to "):]
		if _, err := fmt.Fprintf(buf, "rename to %s\n", quote(rem)); err != nil {
			return err
		}
		return nil
	}

	// Print as-is
	if _, err := fmt.Fprintln(buf, xheader); err != nil {
		return err
	}

	// TODO: "Binary files a/XXX and b/YYY differ"

	return nil
}

func quote(in string) string {
	if in == "/dev/null" {
		return in
	}

	if strings.HasPrefix(in, "\"") && strings.HasPrefix(in, "\"") {
		return in
	}

	return fmt.Sprintf("%q", in)
}

func printFileHeader(w io.Writer, prefix string, filename string, timestamp *time.Time, quoteName bool) error {
	if quoteName {
		filename = quote(filename)
	}

	if _, err := fmt.Fprint(w, prefix, filename); err != nil {
		return err
	}
	if timestamp != nil {
		if _, err := fmt.Fprint(w, "\t", timestamp.Format(diffTimeFormatLayout)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return nil
}

// PrintHunks prints diff hunks in unified diff format.
func PrintHunks(hunks []*Hunk) ([]byte, error) {
	var buf bytes.Buffer
	for _, hunk := range hunks {
		_, err := fmt.Fprintf(&buf,
			"@@ -%d,%d +%d,%d @@", hunk.OrigStartLine, hunk.OrigLines, hunk.NewStartLine, hunk.NewLines,
		)
		if err != nil {
			return nil, err
		}
		if hunk.Section != "" {
			_, err := fmt.Fprint(&buf, " ", hunk.Section)
			if err != nil {
				return nil, err
			}
		}
		if _, err := fmt.Fprintln(&buf); err != nil {
			return nil, err
		}

		if hunk.OrigNoNewlineAt == 0 {
			if _, err := buf.Write(hunk.Body); err != nil {
				return nil, err
			}
		} else {
			if _, err := buf.Write(hunk.Body[:hunk.OrigNoNewlineAt]); err != nil {
				return nil, err
			}
			if err := printNoNewlineMessage(&buf); err != nil {
				return nil, err
			}
			if _, err := buf.Write(hunk.Body[hunk.OrigNoNewlineAt:]); err != nil {
				return nil, err
			}
		}

		if !bytes.HasSuffix(hunk.Body, []byte{'\n'}) {
			if _, err := fmt.Fprintln(&buf); err != nil {
				return nil, err
			}
			if err := printNoNewlineMessage(&buf); err != nil {
				return nil, err
			}
		}
	}
	return buf.Bytes(), nil
}

func printNoNewlineMessage(w io.Writer) error {
	if _, err := w.Write([]byte(noNewlineMessage)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return nil
}
