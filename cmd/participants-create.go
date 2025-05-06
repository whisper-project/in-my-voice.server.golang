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

var participantsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create study participants.",
	Long: `Creates study participants with template data.
They have templated names and, if you don't specify a voice ID to use,
each will use one picked at random from the API key's list of voices.
The API key must be specified and be a valid ElevenLabs API key.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		apiKey, _ := cmd.Flags().GetString("key")
		if ok, err := services.ElevenValidateApiKey(apiKey); err != nil {
			log.Fatalf("Failed to validate ElevenLabs API key: %v", err)
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
	participantsCmd.AddCommand(participantsCreateCmd)
	participantsCreateCmd.Args = cobra.NoArgs
	participantsCreateCmd.Flags().StringP("env", "e", "development", "The environment to run in")
	participantsCreateCmd.Flags().IntP("count", "c", 1, "The number of participants to create")
	participantsCreateCmd.Flags().StringP("key", "k", "", "The API key for the created participants")
	participantsCreateCmd.Flags().StringP("port", "p", "", "The voice ID for the created participants")
	participantsCreateCmd.MarkFlagsOneRequired("key")
}

func createParticipant(apiKey, voiceId string) {
	upn := templateName()
	id := voiceId
	var name string
	if id == "" {
		id, name = pickVoice(apiKey)
	}
	p, err := storage.CreateStudyParticipant(upn)
	if err != nil {
		log.Fatalf("Can't create participant %s: %v", upn, err)
	}
	if ok, err := p.UpdateApiKey(apiKey); err != nil {
		log.Fatalf("Can't assign API key to participant %s: %v", upn, err)
	} else if !ok {
		log.Fatalf("Invalid API key for participant %s: %s", upn, apiKey)
	}
	if ok, err := p.UpdateVoiceId(id); err != nil {
		log.Fatalf("Can't assign voice ID to participant %s: %v", upn, err)
	} else if !ok {
		log.Fatalf("Invalid voice ID for participant %s: %s", upn, id)
	}
	if name != "" {
		log.Printf("Created participant %q with voice %s", upn, name)
	} else {
		log.Printf("Created participant %q", upn)
	}
}

//goland:noinspection SpellCheckingInspection
const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

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
	next := seededRand.Intn(len(voices))
	voice := voices[next]
	return voice.VoiceId, voice.Name
}
