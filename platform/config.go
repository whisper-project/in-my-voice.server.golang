/*
 * Copyright 2024-2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dotenv-org/godotenvvault"
)

type Environment struct {
	AgePublicKey    string
	AgeSecretKey    string
	AwsAccessKey    string
	AwsBucket       string
	AwsReportFolder string
	AwsRegion       string
	AwsSecretKey    string
	DbKeyPrefix     string
	DbUrl           string
	HttpHost        string
	HttpPort        int
	HttpScheme      string
	Name            string
	SmtpCredId      string
	SmtpCredSecret  string
	SmtpHost        string
	SmtpPort        int
}

//goland:noinspection SpellCheckingInspection
var (
	ciConfig = Environment{
		Name:           "CI",
		AgePublicKey:   "age1kq7jnct8jv0d2hr7u3vpf6804emfqwptz6svy9mc9nv7pnkzqc5qsm4wrz",
		AgeSecretKey:   "AGE-SECRET-KEY-1H30T40KMX70VYEHM2ATF9N02U6KRL46660EFPU8GT76DAJNZN6EQRU7CGA",
		HttpScheme:     "http",
		HttpHost:       "localhost",
		SmtpHost:       "localhost",
		HttpPort:       8080,
		SmtpPort:       2500,
		SmtpCredSecret: "",
		SmtpCredId:     "",
		DbUrl:          "redis://",
		DbKeyPrefix:    "c:",
	}
	loadedConfig        = ciConfig
	configStack         []Environment
	configChangeActions = make(map[string]func())
)

func RegisterForConfigChange(name string, action func()) {
	if _, ok := configChangeActions[name]; ok {
		panic(`Duplicate action registration for "` + name + `"`)
	}
	configChangeActions[name] = action
}

func UnregisterForConfigChange(name string) {
	delete(configChangeActions, name)
}

func runConfigChangeActions() {
	for _, action := range configChangeActions {
		action()
	}
}

func GetConfig() Environment {
	return loadedConfig
}

func PushConfig(name string) error {
	if name == "" {
		return pushEnvConfig("")
	}
	if strings.HasPrefix(name, "c") {
		return pushCiConfig()
	}
	if strings.HasPrefix(name, "d") {
		return pushEnvConfig(".env")
	}
	if strings.HasPrefix(name, "s") {
		return pushEnvConfig(".env.staging")
	}
	if strings.HasPrefix(name, "p") {
		return pushEnvConfig(".env.production")
	}
	if strings.HasPrefix(name, "t") {
		return pushEnvConfig(".env.testing")
	}
	return fmt.Errorf("unknown environment: %s", name)
}

func PushAlteredConfig(env Environment) {
	configStack = append(configStack, loadedConfig)
	loadedConfig = env
	runConfigChangeActions()
}

func pushCiConfig() error {
	configStack = append(configStack, loadedConfig)
	loadedConfig = ciConfig
	runConfigChangeActions()
	return nil
}

func pushEnvConfig(filename string) error {
	var d string
	var err error
	if filename == "" {
		if d, err = FindEnvFile(".env.vault", true); err == nil {
			if d == "" {
				err = godotenvvault.Overload()
			} else {
				var c string
				if c, err = os.Getwd(); err == nil {
					if err = os.Chdir(d); err == nil {
						err = godotenvvault.Overload()
						// if we fail to change back to the prior working directory, so be it.
						_ = os.Chdir(c)
					}
				}
			}
		}
	} else {
		if d, err = FindEnvFile(filename, false); err == nil {
			err = godotenvvault.Overload(d + filename)
		}
	}
	if err != nil {
		return fmt.Errorf("error loading .env vars: %v", err)
	}
	configStack = append(configStack, loadedConfig)
	getEnvPort := func(s string, d int) int {
		val, _ := strconv.Atoi(s)
		if val <= 0 {
			return d
		} else {
			return val
		}
	}
	loadedConfig = Environment{
		AgePublicKey:    os.Getenv("AGE_PUBLIC_KEY"),
		AgeSecretKey:    os.Getenv("AGE_SECRET_KEY"),
		AwsAccessKey:    os.Getenv("AWS_ACCESS_KEY"),
		AwsBucket:       os.Getenv("AWS_BUCKET"),
		AwsReportFolder: os.Getenv("AWS_REPORT_FOLDER"),
		AwsRegion:       os.Getenv("AWS_REGION"),
		AwsSecretKey:    os.Getenv("AWS_SECRET_KEY"),
		DbKeyPrefix:     os.Getenv("DB_KEY_PREFIX"),
		DbUrl:           os.Getenv("REDIS_URL"),
		HttpHost:        os.Getenv("HTTP_HOST"),
		HttpPort:        getEnvPort(os.Getenv("HTTP_PORT"), 8080),
		HttpScheme:      os.Getenv("HTTP_SCHEME"),
		Name:            os.Getenv("ENVIRONMENT_NAME"),
		SmtpCredId:      os.Getenv("SMTP_CRED_ID"),
		SmtpCredSecret:  os.Getenv("SMTP_CRED_SECRET"),
		SmtpHost:        os.Getenv("SMTP_HOST"),
		SmtpPort:        getEnvPort(os.Getenv("SMTP_PORT"), 2025),
	}
	runConfigChangeActions()
	return nil
}

func PopConfig() {
	if len(configStack) == 0 {
		return
	}
	loadedConfig = configStack[len(configStack)-1]
	configStack = configStack[:len(configStack)-1]
	runConfigChangeActions()
	return
}

func FindEnvFile(name string, fallback bool) (string, error) {
	for i := range 5 {
		d := ""
		for range i {
			d += "../"
		}
		if _, err := os.Stat(d + name); err == nil {
			return d, nil
		}
		if fallback {
			if _, err := os.Stat(d + ".env"); err == nil {
				return d, nil
			}
		}
	}
	return "", fmt.Errorf("no file %q found in parent", name)
}
