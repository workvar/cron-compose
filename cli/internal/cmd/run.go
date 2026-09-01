package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/croncompose/croncompose/cli/internal/render"
)

func init() {
	run := &cobra.Command{
		Use:     "run",
		Aliases: []string{"runs"},
		Short:   "Inspect runs",
	}
	Root.AddCommand(run)

	run.AddCommand(&cobra.Command{
		Use:   "ls <job_id>",
		Short: "List recent runs for a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requireLogin()
			var resp struct {
				Items []struct {
					ID, Status, Trigger string
					ExitCode            *int   `json:"exit_code"`
					DurationMs          *int   `json:"duration_ms"`
					CreatedAt           string `json:"created_at"`
				} `json:"items"`
			}
			if err := API().JSON(http.MethodGet, "/jobs/"+args[0]+"/runs?limit=20", nil, &resp); err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tTRIGGER\tEXIT\tDUR(ms)\tWHEN")
			for _, r := range resp.Items {
				exit := "-"
				if r.ExitCode != nil {
					exit = fmt.Sprintf("%d", *r.ExitCode)
				}
				dur := "-"
				if r.DurationMs != nil {
					dur = fmt.Sprintf("%d", *r.DurationMs)
				}
				ts := r.CreatedAt
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					render.ShortID(r.ID), r.Status, r.Trigger, exit, dur, ts)
			}
			return w.Flush()
		},
	})

	run.AddCommand(&cobra.Command{
		Use:   "get <run_id>",
		Short: "Print run detail as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requireLogin()
			res, err := API().Do(http.MethodGet, "/runs/"+args[0], nil)
			if err != nil {
				return err
			}
			defer res.Body.Close()
			if res.StatusCode/100 != 2 {
				b, _ := io.ReadAll(res.Body)
				return fmt.Errorf("get: %d %s", res.StatusCode, string(b))
			}
			_, err = io.Copy(cmd.OutOrStdout(), res.Body)
			return err
		},
	})

	var follow bool
	logsCmd := &cobra.Command{
		Use:   "logs <run_id>",
		Short: "Print run logs; --follow streams live via SSE",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requireLogin()
			if !follow {
				var resp struct {
					Items []struct{ Stream, Chunk string } `json:"items"`
				}
				if err := API().JSON(http.MethodGet, "/runs/"+args[0]+"/logs", nil, &resp); err != nil {
					return err
				}
				for _, l := range resp.Items {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", l.Stream, l.Chunk)
				}
				return nil
			}
			return API().StreamSSE("/runs/"+args[0]+"/logs/stream", func(event, data string) error {
				switch event {
				case "log":
					var p struct{ Stream, Chunk string }
					if json.Unmarshal([]byte(data), &p) == nil {
						fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", p.Stream, p.Chunk)
					}
				case "done":
					var p struct{ Status string }
					if json.Unmarshal([]byte(data), &p) == nil {
						fmt.Fprintf(cmd.OutOrStdout(), "-- done: %s --\n", p.Status)
					}
					return io.EOF
				}
				return nil
			})
		},
	}
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "stream live logs via SSE")
	run.AddCommand(logsCmd)
}
