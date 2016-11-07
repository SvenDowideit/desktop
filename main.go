package main

import (
	"fmt"
	"os"

	"github.com/SvenDowideit/desktop/commands"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

// Version is set from the go build commandline
var Version string

// CommitHash is set from the go build commandline
var CommitHash string

type Exit struct {
	Code int
}

func main() {
	// We want our defer functions to be run when calling fatal()
	defer func() {
		if e := recover(); e != nil {
			if ex, ok := e.(Exit); ok == true {
				os.Exit(ex.Code)
			}
			panic(e)
		}
	}()
	app := cli.NewApp()
	app.Name = "desktop"
	app.Version = Version
	app.Usage = "Rancher on the Desktop"
	app.EnableBashCompletion = true

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output in the logs",
		},
	}
	app.Commands = []cli.Command{
		versionCommand,
		commands.Install,
	}
	app.Before = func(context *cli.Context) error {
		if context.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

var versionCommand = cli.Command{
	Name:  "version",
	Usage: "return the version",
	Action: func(context *cli.Context) error {
		fmt.Println(context.App.Version)
		fmt.Println(CommitHash)
		return nil
	},
}

func fatal(err string, code int) {
	fmt.Fprintf(os.Stderr, "[ctr] %s\n", err)
	panic(Exit{code})
}
