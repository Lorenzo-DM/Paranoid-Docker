package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"backend/internal/model"
	"backend/internal/service"
)

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printStacksTable(w io.Writer, stacks []model.ComposeStack) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tSERVICES\tMODE\tUPDATE")
	for _, s := range stacks {
		update := "-"
		if s.UpdateAvailable {
			update = "available"
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", s.Name, s.Status, len(s.Services), s.RollbackMode, update)
	}
	tw.Flush()
}

func printContainersTable(w io.Writer, containers []model.Container) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tIMAGE\tSTATE\tUPDATE")
	for _, c := range containers {
		update := "-"
		if c.UpdateAvailable {
			update = "available"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", c.ShortID, c.Name, c.Image, c.State, update)
	}
	tw.Flush()
}

func printRollbacksTable(w io.Writer, files []model.RollbackFile) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "FILENAME\tCREATED\tPREVIOUS IMAGE")
	for _, f := range files {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", f.Filename, f.CreatedAt.Format("2006-01-02 15:04:05"), f.PreviousImage)
	}
	tw.Flush()
}

// drainStackEvents prints stack events until the channel closes and
// returns the first terminal error event, if any.
func drainStackEvents(w io.Writer, ch <-chan model.StackEvent, asJSON bool) error {
	var terminal error
	for evt := range ch {
		if asJSON {
			b, _ := json.Marshal(evt)
			fmt.Fprintln(w, string(b))
		} else if evt.Type == "error" {
			fmt.Fprintf(w, "ERROR: %s\n", evt.Error)
		} else if evt.Line != "" {
			fmt.Fprintln(w, evt.Line)
		}
		if evt.Type == "error" && terminal == nil {
			terminal = fmt.Errorf("%s", evt.Error)
		}
	}
	return terminal
}

// drainPullEvents prints container update events until the channel
// closes and returns the first terminal error event, if any.
func drainPullEvents(w io.Writer, ch <-chan service.PullEvent, asJSON bool) error {
	var terminal error
	for evt := range ch {
		if asJSON {
			b, _ := json.Marshal(evt)
			fmt.Fprintln(w, string(b))
		} else if evt.Type == "error" {
			fmt.Fprintf(w, "ERROR: %s\n", evt.Error)
		} else {
			text := evt.Status
			if evt.Progress != "" {
				text += " " + evt.Progress
			}
			if text != "" {
				fmt.Fprintln(w, text)
			}
		}
		if evt.Type == "error" && terminal == nil {
			terminal = fmt.Errorf("%s", evt.Error)
		}
	}
	return terminal
}

// drainSaveEvents prints image-save progress until the channel closes.
func drainSaveEvents(w io.Writer, ch <-chan service.SaveProgress, asJSON bool) error {
	var terminal error
	for evt := range ch {
		if asJSON {
			b, _ := json.Marshal(evt)
			fmt.Fprintln(w, string(b))
		} else {
			switch evt.Type {
			case "error":
				fmt.Fprintf(w, "ERROR: %s\n", evt.Error)
			case "done":
				fmt.Fprintf(w, "saved %s (%d bytes)\n", evt.Filename, evt.SizeBytes)
			}
		}
		if evt.Type == "error" && terminal == nil {
			terminal = fmt.Errorf("%s", evt.Error)
		}
	}
	return terminal
}
