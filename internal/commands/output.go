package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format(time.RFC3339)
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "-"
	}
	return value.Format(time.RFC3339)
}

func formatList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

func printRows(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for idx, header := range headers {
		widths[idx] = len(header)
	}
	for _, row := range rows {
		for idx, col := range row {
			if idx < len(widths) && len(col) > widths[idx] {
				widths[idx] = len(col)
			}
		}
	}

	for idx, header := range headers {
		if idx > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprintf(w, "%-*s", widths[idx], header)
	}
	fmt.Fprintln(w)

	for _, row := range rows {
		for idx, col := range row {
			if idx > 0 {
				fmt.Fprint(w, "  ")
			}
			if idx < len(widths) {
				fmt.Fprintf(w, "%-*s", widths[idx], col)
			}
		}
		fmt.Fprintln(w)
	}
}
