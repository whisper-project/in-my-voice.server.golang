/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
)

// dbCliCommand represents the cli command
var dbCliCommand = &cobra.Command{
	Use:   "cli",
	Short: "Open redis-cli on the database",
	Long: `This opens a redis-cli session on the database.
Use the env flag to specify the database to connect to.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		if err := platform.PushConfig(env); err != nil {
			log.Fatalf("Can't load environment %q: %v", env, err)
		}
		cli()
	},
}

func init() {
	dbCmd.AddCommand(dbCliCommand)
	dbCliCommand.Args = cobra.NoArgs
}

func cli() {
	dbUrl := platform.GetConfig().DbUrl
	cli, err := exec.LookPath("redis-cli")
	if err != nil {
		log.Fatalf("Can't find redis-cli: %v", err)
	}
	args := []string{"redis-cli", "--no-auth-warning", "-u", dbUrl}
	env := os.Environ()
	if err := syscall.Exec(cli, args, env); err != nil {
		log.Fatalf("Can't exec redis-cli: %v", err)
	}
}
