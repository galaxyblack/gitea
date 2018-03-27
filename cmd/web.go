// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"net"
	"net/http"
	"net/http/fcgi"
	_ "net/http/pprof" // Used for debugging if enabled and a web server is running
	"os"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/external"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/routes"

	"github.com/Unknwon/com"
	context2 "github.com/gorilla/context"
	"github.com/urfave/cli"
	ini "gopkg.in/ini.v1"
)

// CmdWeb represents the available web sub-command.
var CmdWeb = cli.Command{
	Name:  "web",
	Usage: "Start Gitea web server",
	Description: `Gitea web server is the only thing you need to run,
and it takes care of all the other things for you`,
	Action: runWeb,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "port, p",
			Value: "3000",
			Usage: "Temporary port number to prevent conflict",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "custom/conf/app.ini",
			Usage: "Custom configuration file path",
		},
		cli.StringFlag{
			Name:  "pid, P",
			Value: "/var/run/gitea.pid",
			Usage: "Custom pid file path",
		},
	},
}

func runHTTPRedirector() {
	source := fmt.Sprintf("%s:%s", setting.HTTPAddr, setting.PortToRedirect)
	dest := strings.TrimSuffix(setting.AppURL, "/")
	log.Info("Redirecting: %s to %s", source, dest)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := dest + r.URL.Path
		if len(r.URL.RawQuery) > 0 {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	})

	var err = runHTTP(source, context2.ClearHandler(handler))

	if err != nil {
		log.Fatal(4, "Failed to start port redirection: %v", err)
	}
}

func runWeb(ctx *cli.Context) error {
	if ctx.IsSet("config") {
		setting.CustomConf = ctx.String("config")
	}

	if ctx.IsSet("pid") {
		setting.CustomPID = ctx.String("pid")
	}

	routers.GlobalInit()

	external.RegisterParsers()

	m := routes.NewMacaron()
	routes.RegisterRoutes(m)

	// Flag for port number in case first time run conflict.
	if ctx.IsSet("port") {
		setting.AppURL = strings.Replace(setting.AppURL, setting.HTTPPort, ctx.String("port"), 1)
		setting.HTTPPort = ctx.String("port")

		switch setting.Protocol {
		case setting.UnixSocket:
		case setting.FCGI:
		default:
			// Save LOCAL_ROOT_URL if port changed
			cfg := ini.Empty()
			if com.IsFile(setting.CustomConf) {
				// Keeps custom settings if there is already something.
				if err := cfg.Append(setting.CustomConf); err != nil {
					return fmt.Errorf("Failed to load custom conf '%s': %v", setting.CustomConf, err)
				}
			}

			defaultLocalURL := string(setting.Protocol) + "://"
			if setting.HTTPAddr == "0.0.0.0" {
				defaultLocalURL += "localhost"
			} else {
				defaultLocalURL += setting.HTTPAddr
			}
			defaultLocalURL += ":" + setting.HTTPPort + "/"

			cfg.Section("server").Key("LOCAL_ROOT_URL").SetValue(defaultLocalURL)

			if err := cfg.SaveTo(setting.CustomConf); err != nil {
				return fmt.Errorf("Error saving generated JWT Secret to custom config: %v", err)
			}
		}
	}

	listenAddr := setting.HTTPAddr
	if setting.Protocol != setting.UnixSocket {
		listenAddr += ":" + setting.HTTPPort
	}
	log.Info("Listen: %v://%s%s", setting.Protocol, listenAddr, setting.AppSubURL)

	if setting.LFS.StartServer {
		log.Info("LFS server enabled")
	}

	if setting.EnablePprof {
		go func() {
			log.Info("Starting pprof server on localhost:6060")
			log.Info("%v", http.ListenAndServe("localhost:6060", nil))
		}()
	}

	var err error
	switch setting.Protocol {
	case setting.HTTP:
		err = runHTTP(listenAddr, context2.ClearHandler(m))
	case setting.HTTPS:
		if setting.RedirectOtherPort {
			go runHTTPRedirector()
		}
		err = runHTTPS(listenAddr, setting.CertFile, setting.KeyFile, context2.ClearHandler(m))
	case setting.FCGI:
		listener, err := net.Listen("tcp", listenAddr)
		if err != nil {
			log.Fatal(4, "Failed to bind %s", listenAddr, err)
		}
		defer listener.Close()
		err = fcgi.Serve(listener, context2.ClearHandler(m))
	case setting.UnixSocket:
		if err := os.Remove(listenAddr); err != nil && !os.IsNotExist(err) {
			log.Fatal(4, "Failed to remove unix socket directory %s: %v", listenAddr, err)
		}
		var listener *net.UnixListener
		listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: listenAddr, Net: "unix"})
		if err != nil {
			break // Handle error after switch
		}

		// FIXME: add proper implementation of signal capture on all protocols
		// execute this on SIGTERM or SIGINT: listener.Close()
		if err = os.Chmod(listenAddr, os.FileMode(setting.UnixSocketPermission)); err != nil {
			log.Fatal(4, "Failed to set permission of unix socket: %v", err)
		}
		err = http.Serve(listener, context2.ClearHandler(m))
	default:
		log.Fatal(4, "Invalid protocol: %s", setting.Protocol)
	}

	if err != nil {
		log.Fatal(4, "Failed to start server: %v", err)
	}

	return nil
}
