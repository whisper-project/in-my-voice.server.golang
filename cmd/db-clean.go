/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"context"
	"log"
	"slices"

	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/storage"

	"github.com/spf13/cobra"
)

// dbCleanCmd represents the clean command
var dbCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up the database",
	Long:  `Track down and remove unused database entries and stored objects.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		if err := platform.PushConfig(env); err != nil {
			log.Fatalf("Can't load environment %q: %v", env, err)
		}
		log.Println("Cleaning orphan reports...")
		cleanOrphanReports()
		log.Println("Cleaning empty speech settings...")
		cleanEmptySpeechSettings()
		log.Println("Done.")
	},
}

func init() {
	dbCmd.AddCommand(dbCleanCmd)
	dbCleanCmd.Args = cobra.NoArgs
}

func cleanOrphanReports() {
	ctx := context.Background()
	// first enumerate the study Ids
	sIds, err := storage.GetAllStudyIds()
	if err != nil {
		log.Fatal(err)
	}
	// next enumerate all the report Ids in all the studies
	var allReportIds []string
	for _, sId := range sIds {
		rIds, err := platform.MapGetKeys(ctx, storage.ReportIndex(sId))
		if err != nil {
			log.Fatal(err)
		}
		allReportIds = append(allReportIds, rIds...)
	}
	// next get all the saved report blob ids
	cfg := platform.GetConfig()
	folder := cfg.AwsReportFolder + "/" + cfg.Name
	allBlobIds, err := platform.S3ListBlobs(ctx, folder)
	if err != nil {
		log.Fatal(err)
	}
	// now sort and merge the two lists, deleting any blobs that don't match reports
	slices.Sort(allReportIds)
	slices.Sort(allBlobIds)
	i, j := 0, 0
	for i < len(allReportIds) && j < len(allBlobIds) {
		if allReportIds[i] == allBlobIds[j] {
			i++
			j++
		} else if allReportIds[i] < allBlobIds[j] {
			log.Printf("Warning: report %s has no matching blob!\n", allReportIds[i])
			i++
		} else {
			log.Printf("Deleting blob %s\n", allBlobIds[j])
			err := platform.S3DeleteBlob(ctx, platform.GetConfig().AwsReportFolder, allBlobIds[j])
			if err != nil {
				log.Fatal(err)
			}
			j++
		}
	}
	if i < len(allReportIds) {
		log.Printf("Warning: these reports have no matching blobs: %q\n", allReportIds[i:])
	} else if j < len(allBlobIds) {
		for _, blobId := range allBlobIds[j:] {
			log.Printf("Deleting blob %s\n", blobId)
			err := platform.S3DeleteBlob(ctx, platform.GetConfig().AwsReportFolder, blobId)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func cleanEmptySpeechSettings() {
	ctx := context.Background()
	s := new(storage.SpeechSettings)
	mapper := func() error {
		if s.ProfileId == "" || s.VoiceId == "" || s.VoiceName == "" {
			log.Printf("Deleting empty speech settings for profile %s", s.ProfileId)
			return platform.DeleteStorage(ctx, s)
		}
		return nil
	}
	if err := platform.MapObjects(ctx, mapper, s); err != nil {
		log.Fatal(err)
	}
}
