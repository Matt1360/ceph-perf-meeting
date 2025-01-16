package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "meeting-notes-gen"
	app.Usage = "A tool to generate ceph perf meeting notes"
	app.Version = "0.0.0"
	app.Commands = commands
	app.Flags = []cli.Flag{
		DebugFlag,
	}
	app.CommandNotFound = func(c *cli.Context, command string) {
		log.Fatal().Str("command", command).Msg("unknown command")
	}
	app.Before = func(c *cli.Context) error {
		handleDebug(c)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("command failed")
	}
}

func handleDebug(ctx *cli.Context) {
	if ctx.Bool(DebugFlag.Name) {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

}
