package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SvenDowideit/desktop/commands"
	"github.com/SvenDowideit/desktop/config"
	"github.com/SvenDowideit/desktop/showuserlog"
	"github.com/SvenDowideit/desktop/util"

	"github.com/Shopify/logrus-bugsnag"
	log "github.com/Sirupsen/logrus"
	bugsnag "github.com/bugsnag/bugsnag-go"

	"github.com/urfave/cli"
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
	app.Version = fmt.Sprintf("%s, build %s", Version, CommitHash)
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
		commands.Start,
		commands.Stop,
		commands.Uninstall,
	}
	app.Before = func(context *cli.Context) error {
		if context.GlobalBool("debug") {
			initLogging(log.DebugLevel)
		} else {
			initLogging(log.InfoLevel)
		}

		return nil
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

var versionCommand = cli.Command{
	Name:  "version",
	Usage: "return the version",
	Action: func(context *cli.Context) error {
		fmt.Printf("%s version %s, build %s\n", context.App.Name, context.App.Version, CommitHash)
		// TODO: add versions of all the other SW we install
		return nil
	},
}

func fatal(err string, code int) {
	fmt.Fprintf(os.Stderr, "[ctr] %s\n", err)
	panic(Exit{code})
}

func initLogging(logLevel log.Level) {
	// TODO: i'm trusting that no-one has messed with it since we last made it..
	if _, err := os.Stat(config.LogDir); err != nil {
		if err := util.SudoRun("mkdir", "-p", config.LogDir); err != nil {
			log.Fatal(err)
		}
		if err := util.SudoRun("chmod", "777", config.LogDir); err != nil {
			log.Fatal(err)
		}
	}

	// Write all levels to a log file
	log.SetLevel(log.DebugLevel)
	filename := filepath.Join(config.LogDir, "verbose-"+time.Now().Format("2006-01-02T15.04")+".log")
	fmt.Printf("Debug log written to %s\n", filename)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open %s log file, %v", filename, err)
		log.Fatal(err)
	}
	log.SetOutput(f)

	// Filter what the user sees (info level, unless they set --debug)
	showuserHook, err := showuserlog.NewShowuserlogHook(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.StandardLogger().Hooks.Add(showuserHook)

	// We'll get a bugsnag entry for Error, Fatal and Panic
	bugsnag.Configure(bugsnag.Configuration{
		APIKey:       "ad1003e815853e3c15d939709618d50e",
		AppVersion:   Version,
		ReleaseStage: "initial",
		Synchronous:  true,
	})
	bugsnagHook, err := logrus_bugsnag.NewBugsnagHook()
	if err != nil {
		log.Fatal(err)
	}
	log.StandardLogger().Hooks.Add(bugsnagHook)

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	log.Debugf("START: %v in %s", os.Args, pwd)
}
