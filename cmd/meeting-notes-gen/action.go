package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
)

const datefmt = "2006-01-02"

func action(cctx *cli.Context) error {
	tok := cctx.String(GithubTokenFlag.Name)
	repoflag := cctx.String(GithubRepoFlag.Name)
	sinceptr := cctx.Timestamp(SinceFlag.Name)
	pages := cctx.Int(PagesFlag.Name)

	if sinceptr == nil {
		return errors.New("invalid time")
	}
	since := *sinceptr

	client := github.NewClient(nil).WithAuthToken(tok)

	// Split up the repo
	toks := strings.SplitN(repoflag, "/", 2)
	owner := toks[0]
	repo := toks[1]

	ctx, cancel := context.WithTimeout(cctx.Context, 10*time.Minute)
	defer cancel()

	var prNew []*github.PullRequest
	var prClosed []*github.PullRequest
	var prUpdated []*github.PullRequest
	var prNoMovement []*github.PullRequest

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)

	prch := make(chan *github.PullRequest)

	// Get the changes, bucket them for dumping
	go func() {
		for pr := range prch {
			// Check for performance labels
			perf := false
			for _, label := range pr.Labels {
				if strings.ToLower(label.GetName()) == "performance" {
					perf = true
				}
			}
			if !perf {
				continue
			}

			// Get dates
			created := pr.GetCreatedAt()
			updated := pr.GetUpdatedAt()
			closed := pr.GetClosedAt()
			merged := pr.GetMergedAt()

			// Bucket them into groups
			if created.After(since) {
				prNew = append(prNew, pr)
			} else if closed.After(since) || merged.After(since) {
				prClosed = append(prClosed, pr)
			} else if updated.After(since) {
				prUpdated = append(prUpdated, pr)
			} else if updated.Before(since.Add(-60 * 24 * time.Hour)) {
				// probably beyond no-movement, very stale
			} else {
				// No movement includes things that are not merged, and are not closed
				// (open things that haven't been updated in a while)
				// and we'll drop anything else on the floor
				if merged.IsZero() && closed.IsZero() {
					prNoMovement = append(prNoMovement, pr)
				}
			}
		}
	}()

	// Scan as many pages as we were asked to
	for i := 1; i <= pages; i++ {
		pg := i
		g.Go(func() error {
			// Get all of them that are merging into master (skips backports)
			// sorted by latest update first, as that's what we care about in meetings
			opts := &github.PullRequestListOptions{
				State:     "all",
				Head:      "master",
				Sort:      "updated",
				Direction: "desc",
				ListOptions: github.ListOptions{
					Page:    pg,
					PerPage: 100,
				},
			}

			prs, _, err := client.PullRequests.List(gctx, owner, repo, opts)
			if _, ok := err.(*github.RateLimitError); ok {
				fmt.Printf("rate limited\n")
				return err
			} else if err != nil {
				return err
			}

			for _, pr := range prs {
				// If merged, get the user as that's not embedded
				if !pr.GetMergedAt().IsZero() {
					pr, _, err = client.PullRequests.Get(gctx, owner, repo, pr.GetNumber())
					if _, ok := err.(*github.RateLimitError); ok {
						fmt.Printf("rate limited\n")
						return err
					} else if err != nil {
						return err
					}
				}

				// If it's not merged, and is closed, find out if it was closed by the bot
				if pr.GetMergedAt().IsZero() && !pr.GetClosedAt().IsZero() {
					issue, _, err := client.Issues.Get(gctx, owner, repo, pr.GetNumber())
					if _, ok := err.(*github.RateLimitError); ok {
						fmt.Printf("rate limited\n")
						return err
					} else if err != nil {
						return err
					}

					// Be really lazy and store the closed by user as the merged user
					// since we checked above that merged_at is zero
					pr.MergedBy = issue.GetClosedBy()
				}

				select {
				case prch <- pr:
				case <-gctx.Done():
					return gctx.Err()
				}
			}

			return nil
		})
	}

	go func() {
		g.Wait()
		close(prch)
	}()

	if err := g.Wait(); err != nil {
		return err
	}

	// Output
	fmt.Printf("%s\n----------\n\n", time.Now().Format(datefmt))
	fmt.Printf("- CURRENT STATUS OF PULL REQUESTS (since %s)\n\n", since.Format(datefmt))
	fmt.Printf("  new:\n")
	prDump(prNew, repoflag)
	fmt.Printf("\n  closed:\n")
	prDump(prClosed, repoflag)
	fmt.Printf("\n  updated:\n")
	prDump(prUpdated, repoflag)
	fmt.Printf("\n  no movement:\n")
	prDump(prNoMovement, repoflag)
	fmt.Printf("\n- DISCUSSION TOPICS:\n\n")

	return nil
}

func prDump(prs []*github.PullRequest, repo string) {
	for _, pr := range prs {
		// Get dates
		closed := pr.GetClosedAt()
		merged := pr.GetMergedAt()

		id := pr.GetNumber()
		title := pr.GetTitle()
		isDraft := pr.GetDraft()
		isMergeable := pr.GetMergeable()

		u := pr.GetUser()
		submitter := u.GetLogin()
		company := u.GetCompany()
		if company != "" {
			submitter = fmt.Sprintf("%s (%s)", submitter, company)
		}

		// Label checks
		stale := false
		needsRebase := false
		for _, l := range pr.Labels {
			if l.GetName() == "stale" {
				stale = true
			} else if l.GetName() == "needs-rebase" {
				needsRebase = true
			}
		}

		icon := ""
		if isDraft {
			icon += " ✏️"
		}
		if isMergeable {
			icon += " ✅"
		}
		if !merged.IsZero() {
			icon += " 🎉"
		}
		if !closed.IsZero() && merged.IsZero() {
			icon += " ❌"
		}

		fmt.Printf("    https://github.com/%s/pull/%d%s (%s, %s)", repo, id, icon, title, submitter)

		if !merged.IsZero() {
			fmt.Printf(" merged by %s", pr.GetMergedBy().GetLogin())
		}

		// If it's not merged and is closed, we should have a closed by user
		// stored in the merged_by field, which is a dirty hack above
		if merged.IsZero() && !closed.IsZero() {
			fmt.Printf(" closed by %s", pr.GetMergedBy().GetLogin())
		}

		if stale {
			fmt.Printf(" (💀 stale)")
		}
		if needsRebase {
			fmt.Printf(" (💀 needs rebase)")
		}

		fmt.Printf("\n")
	}
}
