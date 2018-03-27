// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Gitea (git with a cup of tea) is a painless self-hosted Git Service.
package main // import "code.gitea.io/gitea"

import (
	"os"
	"strings"

	"github.com/galaxyblack/gitea/cmd"
	"github.com/galaxyblack/gitea/modules/log"
	"github.com/galaxyblack/gitea/modules/setting"
	// register supported doc types
	_ "github.com/galaxyblack/gitea/modules/markup/markdown"
	_ "github.com/galaxyblack/gitea/modules/markup/orgmode"

	"github.com/urfave/cli"
)

// Version holds the current Gitea version
var Version = "1.5.0-dev"

// Tags holds the build tags used
var Tags = ""

func init() {
	setting.AppVer = Version
	setting.AppBuiltWith = formatBuiltWith(Tags)
}

func main() {
	app := cli.NewApp()
	app.Name = "OpenSci"
	app.Usage = "Open Source Science"
	app.Description = `By default, opensci.io will start serving using the webserver with no
arguments - which can alternatively be run by running the subcommand web.`
	app.Version = Version + formatBuiltWith(Tags)
	app.Commands = []cli.Command{
		cmd.CmdWeb,
		cmd.CmdServ,
		cmd.CmdHook,
		cmd.CmdDump,
		cmd.CmdCert,
		cmd.CmdAdmin,
		cmd.CmdGenerate,
	}
	app.Flags = append(app.Flags, []cli.Flag{}...)
	app.Action = cmd.CmdWeb.Action
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(4, "Failed to run app with %s: %v", os.Args, err)
	}
}

func formatBuiltWith(Tags string) string {
	if len(Tags) == 0 {
		return ""
	}

	return " built with: " + strings.Replace(Tags, " ", ", ", -1)
}
