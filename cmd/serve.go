/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"bytes"
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/api/swift"
	"github.com/whisper-project/in-my-voice.server.golang/gui/admin"
	"github.com/whisper-project/in-my-voice.server.golang/lifecycle"
	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"log"

	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the server.",
	Long:  `Runs the In My Voice server until it's signaled to stop.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		address, _ := cmd.Flags().GetString("address")
		port, _ := cmd.Flags().GetString("port")
		err := platform.PushConfig(env)
		if err != nil {
			log.Fatalf("Can't load configuration: %v", err)
		}
		defer platform.PopConfig()
		serve(address, port)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Args = cobra.NoArgs
	serveCmd.Flags().StringP("env", "e", "development", "The environment to run in")
	serveCmd.Flags().StringP("address", "a", "127.0.0.1", "The IP address to listen on")
	serveCmd.Flags().StringP("port", "p", "8080", "The port to listen on")
}

func serve(address, port string) {
	//goland:noinspection SpellCheckingInspection
	if err := storage.EnsureSuperAdmin(user("WUc6n`bmkao+orplhgy4vzp")); err != nil {
		panic(err)
	}
	if err := storage.BuildServerPrefix(); err != nil {
		panic(err)
	}
	r, err := lifecycle.CreateEngine()
	if err != nil {
		panic(err)
	}
	swiftGroup := r.Group("/api/swift/v1")
	swift.AddRoutes(swiftGroup)
	adminGroup := r.Group("/gui/admin/v1")
	admin.AddRoutes(adminGroup)
	r.Static("/css", "static/css")
	r.Static("/root", "static/root")
	r.NoRoute(middleware.RewriteRoot(r))
	r.LoadHTMLGlob("templates/**/*")
	lifecycle.Startup(r, fmt.Sprintf("%s:%s", address, port))
}

func user(s string) string {
	b := bytes.NewBufferString(s)
	buf := b.Bytes()
	for i := 0; i < len(buf); i++ {
		buf[i] = buf[i] + 13 - byte(i)
	}
	return b.String()
}
