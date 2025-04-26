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
	Name           string
	HttpScheme     string
	HttpHost       string
	HttpPort       int
	SmtpHost       string
	SmtpPort       int
	SmtpCredSecret string
	SmtpCredId     string
	DbUrl          string
	DbKeyPrefix    string
}

//goland:noinspection SpellCheckingInspection
var (
	ciConfig = Environment{
		Name:           "CI",
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
	loadedConfig = ciConfig
	configStack  []Environment
)

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
}

func pushCiConfig() error {
	configStack = append(configStack, loadedConfig)
	loadedConfig = ciConfig
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
	getEnvInt := func(s string) int {
		val, _ := strconv.Atoi(s)
		if val <= 0 {
			return 25
		} else {
			return val
		}
	}
	loadedConfig = Environment{
		Name:           os.Getenv("ENVIRONMENT_NAME"),
		HttpScheme:     os.Getenv("HTTP_SCHEME"),
		HttpHost:       os.Getenv("HTTP_HOST"),
		HttpPort:       getEnvInt(os.Getenv("HTTP_PORT")),
		SmtpHost:       os.Getenv("SMTP_HOST"),
		SmtpPort:       getEnvInt(os.Getenv("SMTP_PORT")),
		SmtpCredSecret: os.Getenv("SMTP_CRED_SECRET"),
		SmtpCredId:     os.Getenv("SMTP_CRED_ID"),
		DbUrl:          os.Getenv("REDIS_URL"),
		DbKeyPrefix:    os.Getenv("DB_KEY_PREFIX"),
	}
	return nil
}

func PopConfig() {
	if len(configStack) == 0 {
		return
	}
	loadedConfig = configStack[len(configStack)-1]
	configStack = configStack[:len(configStack)-1]
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
	return "", fmt.Errorf("no file %q found in path", name)
}
