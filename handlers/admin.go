/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

var emailPattern = regexp.MustCompile("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$")

func AuthMiddleware(c *gin.Context) {
	sessionId := c.Param("sessionId")
	if user, err := storage.GetSessionUser(sessionId); user != nil {
		setAuthenticatedUser(c, user)
		c.Next()
	} else if err != nil {
		logout := fmt.Sprintf("%s/%s/logout", storage.AdminGuiPath, sessionId)
		retry := c.Request.URL.Path
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": retry, "logout": logout})
		c.Abort()
	} else {
		c.Redirect(http.StatusSeeOther, storage.AdminGuiPath+"/login")
		c.Abort()
	}
}

func GetLoginHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "admin/login.tmpl.html", gin.H{})
}

func PostLoginHandler(c *gin.Context) {
	retry := c.Request.URL.Path
	email := strings.TrimSpace(c.Request.FormValue("email"))
	if !emailPattern.MatchString(email) {
		c.HTML(http.StatusOK, "admin/login.tmpl.html", gin.H{"error": email})
		return
	}
	user, err := storage.LookupAdminUser(email)
	if user != nil {
		var sessionId string
		sessionId, err = storage.StartSession(user.Id)
		if sessionId != "" {
			path := fmt.Sprintf("%s/%s/admin", storage.AdminGuiPath, sessionId)
			link := storage.ServerPrefix + path
			err = services.SendLinkViaEmail(email, link)
			if err != nil {
				// couldn't send the email, so remove the session
				_ = storage.DeleteSession(sessionId)
			}
		}
	}
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": retry, "logout": retry})
		return
	}
	c.HTML(http.StatusOK, "admin/login.tmpl.html", gin.H{"success": email})
}

func LogoutHandler(c *gin.Context) {
	sessionId := c.Param("sessionId")
	login := "./login"
	if sessionId != "" {
		_ = storage.DeleteSession(sessionId)
		login = "../login"
	}
	c.Redirect(http.StatusSeeOther, login)
}

func AdminHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	roles := make(map[string]bool)
	var directLink string
	if u.HasRole(storage.AdminRoleUserManager) {
		roles["userManager"] = true
		directLink = "./users"
	}
	if u.HasRole(storage.AdminRoleParticipantManager) {
		roles["participantManager"] = true
		directLink = "./participants"
	}
	if u.HasRole(storage.AdminRoleResearcher) {
		roles["researcher"] = true
		directLink = "./stats"
	}
	if len(roles) == 0 {
		// should never happen
		c.Redirect(http.StatusSeeOther, "./logout")
	}
	if len(roles) == 1 {
		c.Redirect(http.StatusSeeOther, directLink)
	}
	c.HTML(http.StatusOK, "admin/admin.tmpl.html", roles)
}

func GetUsersHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	if !u.HasRole(storage.AdminRoleUserManager) {
		// user removed their edit privilege while on this page!
		c.Redirect(http.StatusSeeOther, "./admin")
		return
	}
	users, err := storage.GetAllAdminUsers()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	deleteId := c.Query("delete")
	editId := c.Query("edit")
	message := c.Query("msg")
	var editUser map[string]string
	slices.SortFunc(users, func(a, b *storage.AdminUser) int { return strings.Compare(a.Email, b.Email) })
	userList := make([]map[string]string, 0, len(users))
	for _, user := range users {
		if user.HasRole(storage.AdminRoleSuperAdmin) {
			continue
		}
		if deleteId == user.Id {
			if deleteId == u.Id {
				msg := url.QueryEscape("You can't delete yourself!")
				c.Redirect(http.StatusSeeOther, "./users?msg="+msg)
				return
			} else {
				if err := storage.DeleteAdminUser(deleteId); err != nil {
					message = fmt.Sprintf("Failed to delete %s!", user.Email)
					deleteId = ""
				} else {
					msg := url.QueryEscape("User deleted successfully.")
					c.Redirect(http.StatusSeeOther, "./users?msg="+msg)
					return
				}
			}
		} else if editId == user.Id {
			editUser = map[string]string{"Id": user.Id, "Email": user.Email}
			for _, r := range user.GetRoles() {
				editUser[storage.RoleLabels[r]] = "true"
			}
			editId = ""
		}
		userMap := map[string]string{"Id": user.Id, "Email": user.Email, "Roles": user.RoleStorage}
		userList = append(userList, userMap)
	}
	if deleteId != "" || editId != "" {
		c.Redirect(http.StatusSeeOther, "./users")
		return
	}
	c.HTML(http.StatusOK, "admin/users.tmpl.html", gin.H{"Users": userList, "Edit": editUser, "Message": message})
}

func PostUsersHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	if !u.HasRole(storage.AdminRoleUserManager) {
		// user removed their edit privilege while on this page!
		c.Redirect(http.StatusSeeOther, "./admin")
		return
	}
	userId := c.PostForm("id")
	email := strings.TrimSpace(c.PostForm("email"))
	if !emailPattern.MatchString(email) {
		msg := url.QueryEscape("You must provide a valid email address.")
		target := "./users?msg=" + msg
		if userId != "" {
			target = fmt.Sprintf("./users?edit=%s&msg=%s", userId, msg)
		}
		c.Redirect(http.StatusSeeOther, target)
		return
	}
	var roles []storage.AdminRole
	for _, r := range storage.AllRoles {
		if c.PostForm(storage.RoleLabels[r]) == "on" {
			roles = append(roles, r)
		}
	}
	if roles == nil {
		msg := url.QueryEscape("You must specify at least one role.")
		target := "./users?msg=" + msg
		if userId != "" {
			target = fmt.Sprintf("./users?edit=%s&msg=%s", userId, msg)
		}
		c.Redirect(http.StatusSeeOther, target)
		return
	}
	if userId == "" {
		if user, err := storage.LookupAdminUser(email); err == nil && user != nil {
			msg := url.QueryEscape(fmt.Sprintf("A user with email %s already exists.", email))
			target := "./users?msg=" + msg
			c.Redirect(http.StatusSeeOther, target)
			return
		}
		user := storage.NewAdminUser(email)
		user.SetRoles(roles)
		if err := storage.SaveAdminUser(user); err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
			return
		}
		msg := url.QueryEscape("User created successfully.")
		target := "./users?msg=" + msg
		c.Redirect(http.StatusSeeOther, target)
		return
	}
	user, err := storage.GetAdminUser(userId)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	if user == nil {
		msg := url.QueryEscape("User not found.")
		target := "./users?msg=" + msg
		c.Redirect(http.StatusSeeOther, target)
		return
	}
	user.Email = email
	user.SetRoles(roles)
	if err := storage.SaveAdminUser(user); err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	msg := url.QueryEscape("User updated successfully.")
	target := "./users?msg=" + msg
	c.Redirect(http.StatusSeeOther, target)
}

func GetParticipantsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || !u.HasRole(storage.AdminRoleParticipantManager) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	participants, err := storage.GetAllStudyParticipants()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	slices.SortFunc(participants, CompareParticipantsFunc(c.Query("sort")))
	message := c.Query("msg")
	editId := c.Query("edit")
	var pEdit map[string]string
	pList := make([]map[string]string, 0, len(participants))
	for _, p := range participants {
		if editId == p.Upn {
			editId = ""
			pEdit = map[string]string{"UPN": p.Upn}
			pEdit["Memo"] = p.Memo
			if p.Assigned > 0 {
				pEdit["Assigned"] = FormatParticipantDateTime(p.Assigned)
			}
			pEdit["Key"] = p.ApiKey
			pEdit["Voice"] = p.VoiceId
			if p.Started > 0 {
				pEdit["Started"] = FormatParticipantDateTime(p.Started)
			}
			if p.Finished > 0 {
				pEdit["Finished"] = FormatParticipantDateTime(p.Finished)
			}
		}
		pList = append(pList, MakeParticipantMap(p))
	}
	if editId != "" {
		c.Redirect(http.StatusSeeOther, "./participants")
		return
	}
	c.HTML(http.StatusOK, "admin/participants.tmpl.html", gin.H{"Participants": pList, "Edit": pEdit, "Message": message})
}

func PostParticipantsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || !u.HasRole(storage.AdminRoleParticipantManager) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	op := c.PostForm("op")
	upn := c.PostForm("upn")
	memo := strings.TrimSpace(c.PostForm("memo"))
	if op == "add" {
		p, err := storage.CreateStudyParticipant(upn)
		if err != nil {
			if errors.Is(err, storage.ParticipantAlreadyExistsError) {
				msg := url.QueryEscape(fmt.Sprintf("A participant with UPN %s already exists.", p))
				target := "./participants?msg=" + msg
				c.Redirect(http.StatusSeeOther, target)
				return
			}
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
			return
		}
		if memo != "" {
			if err = p.UpdateAssignment(memo); err != nil {
				c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
				return
			}
		}
		msg := url.QueryEscape("UPN added successfully.")
		if memo != "" {
			msg = url.QueryEscape("UPN added and assigned successfully.")
		}
		target := "./participants?msg=" + msg
		c.Redirect(http.StatusSeeOther, target)
		return
	}
	// op == edit
	p, err := storage.GetStudyParticipant(upn)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
	if p == nil {
		msg := url.QueryEscape("Participant not found.")
		target := "./participants?msg=" + msg
		c.Redirect(http.StatusSeeOther, target)
		return
	}
	if memo != p.Memo {
		if memo == "" {
			msg := "Assignment memo cannot be blank."
			target := "./participants?edit=" + upn + "&msg=" + msg
			c.Redirect(http.StatusSeeOther, target)
			return
		}
		if err = p.UpdateAssignment(memo); err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
			return
		}
	}
	if key := c.PostForm("key"); key != "" && key != p.ApiKey {
		if ok, err := p.UpdateApiKey(key); err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
			return
		} else if !ok {
			msg := url.QueryEscape("Invalid API key.")
			target := "./participants?edit=" + upn + "&msg=" + msg
			c.Redirect(http.StatusSeeOther, target)
			return
		}
	}
	if voice := c.PostForm("voice"); voice != "" && voice != p.VoiceId {
		if p.ApiKey == "" {
			msg := url.QueryEscape("Can't set voice ID without an API key.")
			target := "./participants?edit=" + upn + "&msg=" + msg
			c.Redirect(http.StatusSeeOther, target)
			return
		}
		if ok, err := p.UpdateVoiceId(voice); err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
			return
		} else if !ok {
			msg := url.QueryEscape("Invalid voice ID.")
			target := "./participants?edit=" + upn + "&msg=" + msg
			c.Redirect(http.StatusSeeOther, target)
			return
		}
	}
	msg := url.QueryEscape("Participant info updated successfully.")
	target := "./participants?msg=" + msg
	c.Redirect(http.StatusSeeOther, target)
}

func GetStatsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || !u.HasRole(storage.AdminRoleResearcher) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
}

func PostStatsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || !u.HasRole(storage.AdminRoleResearcher) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"retry": "", "logout": "./logout"})
		return
	}
}

func getAuthenticatedUser(c *gin.Context) *storage.AdminUser {
	val, ok := c.Get("authenticatedUser")
	if !ok || val == nil {
		return nil
	}
	user, ok := val.(*storage.AdminUser)
	if !ok {
		return nil
	}
	return user
}

func setAuthenticatedUser(c *gin.Context, user *storage.AdminUser) {
	c.Set("authenticatedUser", user)
}

func CompareParticipantsFunc(sort string) func(a, b *storage.StudyParticipant) int {
	return func(a, b *storage.StudyParticipant) int {
		upnCompare := strings.Compare(a.Upn, b.Upn)
		timeCompare := func(t1, t2 int64) int {
			if t1 == t2 {
				return upnCompare
			} else if t1 < t2 {
				return -1
			} else {
				return 1
			}
		}
		switch sort {
		case "assigned":
			return timeCompare(a.Assigned, b.Assigned)
		case "start":
			return timeCompare(a.Started, b.Started)
		case "end":
			return timeCompare(a.Finished, b.Finished)
		default:
			return upnCompare
		}
	}
}

func MakeParticipantMap(p *storage.StudyParticipant) map[string]string {
	pMap := map[string]string{"UPN": p.Upn}
	if p.Assigned > 0 {
		memo := p.Memo
		if len(memo) > 20 {
			memo = memo[:17] + "..."
		}
		pMap["Assigned"] = FormatParticipantDate(p.Assigned) + " (" + memo + ")"
		if p.ApiKey != "" {
			if p.VoiceId != "" {
				pMap["Configured"] = "Yes"
			} else {
				pMap["Configured"] = "API Key Only"
			}
		} else {
			pMap["Configured"] = "No"
		}
	}
	if p.Started > 0 {
		pMap["Started"] = FormatParticipantDate(p.Started)
	}
	if p.Finished > 0 {
		pMap["Finished"] = FormatParticipantDate(p.Finished)
	}
	return pMap
}

func FormatParticipantDate(t int64) string {
	return time.UnixMilli(t).In(storage.AdminTZ).Format("01/02/2006")
}

func FormatParticipantDateTime(t int64) string {
	return time.UnixMilli(t).In(storage.AdminTZ).Format("01/02/2006 3:04pm MST")
}
