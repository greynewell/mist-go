// Package output provides JSON and table formatting shared across
// all MIST stack tools.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Writer formats output as JSON lines or aligned tables.
type Writer struct {
	Format string
	W      io.Writer
}

// New creates a Writer. Use "json" for machine-readable output.
func New(format string) *Writer {
	return &Writer{Format: format, W: os.Stdout}
}

// JSON writes v as a single JSON line.
func (w *Writer) JSON(v any) error {
	enc := json.NewEncoder(w.W)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// Table writes headers and rows as an aligned table.
func (w *Writer) Table(headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w.W, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	fmt.Fprintln(tw, strings.Repeat("-\t", len(headers)))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}

// Write dispatches to JSON or Table. For table output, callers should
// use Table directly since it requires structured headers and rows.
func (w *Writer) Write(v any) error {
	if w.Format == "json" {
		return w.JSON(v)
	}
	return w.JSON(v)
}

// Error writes an error message to stderr.
func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
