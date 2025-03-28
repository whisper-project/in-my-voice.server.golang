/*
Copyright © 2025 Daniel C. Brotsky
*/
package cmd

import (
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/api/swift"
	"github.com/whisper-project/in-my-voice.server.golang/lifecycle"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"log"

	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the server.",
	Long:  `Runs the In My Voice server until it's signaled to stop.'`,
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
	r, err := lifecycle.CreateEngine()
	if err != nil {
		panic(err)
	}
	swiftGroup := r.Group("/api/swift/v1")
	swift.AddRoutes(swiftGroup)
	lifecycle.Startup(r, fmt.Sprintf("%s:%s", address, port))
}
