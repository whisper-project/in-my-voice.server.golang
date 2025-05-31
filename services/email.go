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

func SendLinkViaEmail(address, link string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", "noreply@whisper-project.org")
	m.SetHeader("To", address)
	m.SetHeader("Subject", "In My Voice login link")
	msg := `To log into the administration console, please copy/paste this link into your browser:`
	m.SetBody("text/plain", fmt.Sprintf("%s\n\n%s", msg, link))
	msg = `<p>To log into the administration console, please click <a href="%s">this link</a>.</p>`
	m.AddAlternative("text/html", fmt.Sprintf(msg, link))

	env := platform.GetConfig()
	d := gomail.NewDialer(env.SmtpHost, env.SmtpPort, env.SmtpCredId, env.SmtpCredSecret)
	return d.DialAndSend(m)
}
