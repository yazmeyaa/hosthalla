package cli

import (
	"fmt"
	"io"
)

func PrintHelp(w io.Writer, cmd *Command) {
	if cmd == nil {
		return
	}

	if cmd.Usage != "" {
		fmt.Fprintln(w, "Usage:")
		fmt.Fprintf(w, "  %s\n", cmd.Usage)
	}

	if cmd.Short != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, cmd.Short)
	}

	if len(cmd.Children) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Commands:")
		for _, child := range cmd.Children {
			if child == nil || child.Name == "" {
				continue
			}
			fmt.Fprintf(w, "  %-18s %s\n", child.Name, child.Short)
		}
	}
}
