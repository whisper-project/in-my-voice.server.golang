/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package services

import (
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"gopkg.in/gomail.v2"
)

func SendAuthenticationCodeViaEmail(address, code string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", "noreply@whisper-project.org")
	m.SetHeader("To", address)
	m.SetHeader("Subject", "In My Voice admin console login code")
	msg := `To log into the administration console, please enter the following code:`
	m.SetBody("text/plain", fmt.Sprintf("%s\n\n%s", msg, code))

	env := platform.GetConfig()
	d := gomail.NewDialer(env.SmtpHost, env.SmtpPort, env.SmtpCredId, env.SmtpCredSecret)
	return d.DialAndSend(m)
}
