package main

import (
	"time"

	"github.com/urfave/cli/v2"
)

// Flags
var (
	DebugFlag = &cli.BoolFlag{
		Name:     "debug",
		Usage:    "enable debug logging",
		Value:    false,
		Required: false,
	}

	GithubTokenFlag = &cli.StringFlag{
		Name:     "token",
		EnvVars:  []string{"GITHUB_TOKEN"},
		Usage:    "set your github token to scrape the list of merge requests",
		Required: true,
	}

	GithubRepoFlag = &cli.StringFlag{
		Name:  "repo",
		Usage: "set to the repo to scrape",
		Value: "ceph/ceph",
	}

	SinceFlag = &cli.TimestampFlag{
		Name:   "since",
		Usage:  "when to scan for",
		Layout: "2006-01-02",
		Value:  cli.NewTimestamp(time.Now().Add(-7 * 24 * time.Hour)),
	}
)

// Commands
var commands = []*cli.Command{
	{
		Name:   "gen",
		Usage:  "generate notes",
		Action: action,
		Flags: []cli.Flag{
			GithubTokenFlag,
			GithubRepoFlag,
			SinceFlag,
		},
	},
}
