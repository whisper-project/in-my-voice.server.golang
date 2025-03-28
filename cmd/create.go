/*
Copyright © 2025 Daniel C. Brotsky

*/

package cmd

import (
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"log"
	"math/rand"
	"time"

	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create dummy study participants.",
	Long: `Creates study participants with template data.
They have templated names and, if you don't specify a voice ID to use,
each will use one picked at random from the API key's list of voices.
The API key must be specified and be a valid ElevenLabs API key.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		apiKey, _ := cmd.Flags().GetString("key")
		if ok, err := services.ElevenValidateApiKey(apiKey); err != nil {
			log.Fatalf("Failed to validate API key: %v", err)
		} else if !ok {
			log.Fatalf("Not a valid ElevenLabs API key: %s", apiKey)
		}
		voiceId, _ := cmd.Flags().GetString("voice")
		count, _ := cmd.Flags().GetInt("count")
		err := platform.PushConfig(env)
		if err != nil {
			log.Fatalf("Can't load configuration: %v", err)
		}
		defer platform.PopConfig()
		for i := 0; i < count; i++ {
			createParticipant(apiKey, voiceId)
		}
	},
}

func init() {
	participantsCmd.AddCommand(createCmd)
	createCmd.Args = cobra.NoArgs
	createCmd.Flags().StringP("env", "e", "development", "The environment to run in")
	createCmd.Flags().IntP("count", "c", 1, "The number of participants to create")
	createCmd.Flags().StringP("key", "k", "", "The API key for the created participants")
	createCmd.Flags().StringP("port", "p", "", "The voice ID for the created participants")
	createCmd.MarkFlagsOneRequired("key")
}

func createParticipant(apiKey, voiceId string) {
	upn := templateName()
	id := voiceId
	if id == "" {
		id, name := pickVoice(apiKey)
		log.Printf("Picked voice %s (%s) for user %s", name, id, upn)
	}
	if err := storage.AddStudyParticipant(upn, apiKey, id); err != nil {
		log.Fatalf("Can't add participant %s: %v", upn, err)
	}
	log.Printf("Added participant %s", upn)
}

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func templateName() string {
	b := make([]byte, 17)
	for i, j := 0, 0; i < 17; i += 1 {
		if j > 0 && j%5 == 0 {
			b[i] = '-'
			j = 0
		} else {
			b[i] = charset[seededRand.Intn(len(charset))]
			j += 1
		}
	}
	return "sp-" + string(b)
}

var voices []services.VoiceInfo

func pickVoice(apiKey string) (string, string) {
	if voices == nil {
		var err error
		voices, err = services.ElevenFetchVoices(apiKey)
		if err != nil {
			log.Fatalf("Failed to fetch voices: %v", err)
		}
	}
	voice := voices[seededRand.Intn(len(voices))]
	return voice.VoiceId, voice.Name
}
