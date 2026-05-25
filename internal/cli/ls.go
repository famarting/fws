package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"text/tabwriter"

	"github.com/famarting/fws/internal/config"
	"github.com/famarting/fws/internal/git"
	"github.com/famarting/fws/internal/state"
	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all feature workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			scs, err := state.List(cfg.WorktreesDir)
			if err != nil {
				return err
			}

			sort.Slice(scs, func(i, j int) bool {
				if !scs[i].CreatedAt.Equal(scs[j].CreatedAt) {
					return scs[i].CreatedAt.After(scs[j].CreatedAt)
				}
				if scs[i].Repo != scs[j].Repo {
					return scs[i].Repo < scs[j].Repo
				}
				return scs[i].Name < scs[j].Name
			})

			// Each row needs 2-3 git forks (status, rev-parse, rev-list); fanning
			// out across workspaces drops wall-clock from ~N*100ms to ~max(per-row).
			type rowStatus struct {
				dirty string
				ab    string
			}
			statuses := make([]rowStatus, len(scs))
			var wg sync.WaitGroup
			for i := range scs {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					s := scs[i]
					dirty := "no"
					if d, err := git.IsDirty(s.WorktreePath); err != nil {
						dirty = "?"
					} else if d {
						dirty = "yes"
					}
					ab := "-"
					ahead, behind, err := git.AheadBehind(s.WorktreePath, remoteFromBase(s.Base), s.Branch)
					switch {
					case errors.Is(err, git.ErrNoRemoteRef):
						ab = "unpushed"
					case err != nil:
						ab = "?"
					default:
						ab = fmt.Sprintf("%d/%d", ahead, behind)
					}
					statuses[i] = rowStatus{dirty: dirty, ab: ab}
				}(i)
			}
			wg.Wait()

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tREPO\tCREATED\tDIRTY\tAHEAD/BEHIND\tPATH")
			for i, s := range scs {
				created := "-"
				if !s.CreatedAt.IsZero() {
					created = s.CreatedAt.Local().Format("2006-01-02 15:04")
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					s.Name, s.Repo, created, statuses[i].dirty, statuses[i].ab, shortenHome(s.WorktreePath))
			}
			return tw.Flush()
		},
	}
	return cmd
}
