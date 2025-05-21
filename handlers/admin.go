/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/whisper-project/in-my-voice.server.golang/middleware"
	"github.com/whisper-project/in-my-voice.server.golang/services"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"go.uber.org/zap"
)

var emailPattern = regexp.MustCompile("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$")

func AuthMiddleware(c *gin.Context) {
	sessionId := c.Param("sessionId")
	if user, err := storage.GetSessionUser(sessionId); user != nil {
		setAuthenticatedUser(c, user)
		c.Next()
	} else if err != nil {
		logout := fmt.Sprintf("%s/%s/logout", storage.AdminGuiPath, sessionId)
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": logout})
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
	email := strings.TrimSpace(c.Request.FormValue("email"))
	if !emailPattern.MatchString(email) {
		c.HTML(http.StatusOK, "admin/login.tmpl.html", gin.H{"error": email})
		return
	}
	user, err := storage.LookupAdminUser(email)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html",
			gin.H{"logout": "./login"})
		return
	}
	if user != nil {
		if user.StudyId == "" {
			if !user.HasRole(storage.AdminRoleSuperAdmin) {
				c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html",
					gin.H{"logout": "./login"})
				return
			}
		}
		sessionId, _ := storage.FindSession(user.Id)
		if sessionId == "" {
			sessionId, err = storage.StartSession(user.Id)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html",
					gin.H{"logout": "./login"})
				return
			}
		}
		if sessionId != "" {
			path := fmt.Sprintf("%s/%s/home", storage.AdminGuiPath, sessionId)
			link := storage.ServerPrefix + path
			err = services.SendLinkViaEmail(email, link)
			if err != nil {
				// couldn't send the email, so remove the session
				_ = storage.DeleteSession(sessionId)
				middleware.CtxLog(c).Info("Failed to send a login link via email.",
					zap.String("userId", user.Id), zap.Error(err))
			}
		}
	} else {
		middleware.CtxLog(c).Info("Login attempt from an unauthorized user", zap.String("email", email))
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

func GetHomeHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	var directLink string
	roles := make(map[string]bool)
	var studyOptions []map[string]string
	if u.HasRole(storage.AdminRoleSuperAdmin) {
		studies, err := storage.GetAllStudies()
		if err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html",
				gin.H{"logout": "./logout"})
			return
		}
		slices.SortFunc(studies, func(a, b *storage.Study) int { return strings.Compare(a.Name, b.Name) })
		for _, study := range studies {
			if u.StudyId == "" {
				u.StudyId = study.Id
				if err := storage.SaveAdminUser(u); err != nil {
					c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html",
						gin.H{"logout": "./logout"})
					return
				}
			}
			option := map[string]string{"Name": study.Name, "Id": study.Id}
			if u.StudyId == study.Id {
				option["Selected"] = "selected"
			}
			studyOptions = append(studyOptions, option)
		}
		roles["developer"] = true
	}
	if u.HasRole(storage.AdminRoleUserManager) && u.StudyId != "" {
		roles["userManager"] = true
		directLink = "./users"
	}
	if u.HasRole(storage.AdminRoleParticipantManager) && u.StudyId != "" {
		roles["participantManager"] = true
		directLink = "./participants"
	}
	if u.HasRole(storage.AdminRoleResearcher) && u.StudyId != "" {
		roles["researcher"] = true
		directLink = "./reports"
	}
	if len(roles) == 0 {
		// should never happen
		c.Redirect(http.StatusSeeOther, "./logout")
		return
	}
	if len(roles) == 1 && directLink != "" {
		c.Redirect(http.StatusSeeOther, directLink)
		return
	}
	c.HTML(http.StatusOK, "admin/home.tmpl.html", gin.H{"Roles": roles, "StudyOptions": studyOptions})
}

func PostHomeHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || !u.HasRole(storage.AdminRoleSuperAdmin) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	studyId := c.PostForm("study")
	study, err := storage.GetStudy(studyId)
	if err != nil || study == nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	u.StudyId = studyId
	if err := storage.SaveAdminUser(u); err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	c.Redirect(http.StatusSeeOther, "./home")
}

func GetUsersHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || u.StudyId == "" {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	if !u.HasRole(storage.AdminRoleUserManager) {
		// user removed their edit privilege while on this page!
		c.Redirect(http.StatusSeeOther, "./admin")
		return
	}
	study, _ := storage.GetStudy(u.StudyId)
	if study == nil {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	users, err := storage.GetAllAdminUsers()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
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
		if user.StudyId != u.StudyId {
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
	c.HTML(http.StatusOK, "admin/users.tmpl.html",
		gin.H{"Study": study.Name, "Users": userList, "Edit": editUser, "Message": message})
}

func PostUsersHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || u.StudyId == "" {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
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
		user := storage.NewAdminUser(email, u.StudyId)
		user.SetRoles(roles)
		if err := storage.SaveAdminUser(user); err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
			return
		}
		msg := url.QueryEscape("User created successfully.")
		target := "./users?msg=" + msg
		c.Redirect(http.StatusSeeOther, target)
		return
	}
	user, err := storage.GetAdminUser(userId)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
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
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	msg := url.QueryEscape("User updated successfully.")
	target := "./users?msg=" + msg
	c.Redirect(http.StatusSeeOther, target)
}

func GetParticipantsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || u.StudyId == "" || !u.HasRole(storage.AdminRoleParticipantManager) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	study, _ := storage.GetStudy(u.StudyId)
	if study == nil {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	participants, err := storage.GetAllStudyParticipants(u.StudyId)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	slices.SortFunc(participants, CompareParticipantsFunc(c.Query("sort")))
	message := c.Query("msg")
	editId := c.Query("edit")
	deleteId := c.Query("delete")
	var pEdit map[string]string
	pList := make([]map[string]string, 0, len(participants))
	for _, p := range participants {
		if deleteId == p.Upn {
			if err := storage.DeleteStudyParticipant(u.StudyId, p.Upn); err != nil {
				message := url.QueryEscape("Failed to delete participant.")
				if errors.Is(err, storage.ParticipantInUseError) {
					message = url.QueryEscape("You can't delete a participant who is active in the study.")
				}
				c.Redirect(http.StatusSeeOther, "?msg="+message)
				return
			}
			continue
		}
		if editId == p.Upn {
			editId = ""
			pEdit = map[string]string{"UPN": p.Upn}
			pEdit["Memo"] = p.Memo
			if p.Assigned > 0 {
				pEdit["Assigned"] = formatDateTime(p.Assigned)
			}
			pEdit["Key"] = p.ApiKey
			pEdit["Voice"] = p.VoiceId
			if p.Started > 0 {
				pEdit["Started"] = formatDateTime(p.Started)
			}
			if p.Finished > 0 {
				pEdit["Finished"] = formatDateTime(p.Finished)
			}
		}
		pList = append(pList, MakeParticipantMap(p))
	}
	if editId != "" {
		c.Redirect(http.StatusSeeOther, "./participants")
		return
	}
	c.HTML(http.StatusOK, "admin/participants.tmpl.html",
		gin.H{"Study": study.Name, "Participants": pList, "Edit": pEdit, "Message": message})
}

func PostParticipantsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || u.StudyId == "" || !u.HasRole(storage.AdminRoleParticipantManager) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	op := c.PostForm("op")
	upn := c.PostForm("upn")
	msg := ""
	editAgain := false
	memo := strings.TrimSpace(c.PostForm("memo"))
	apiKey := strings.TrimSpace(c.PostForm("key"))
	voiceId := strings.TrimSpace(c.PostForm("voice"))
	var p *storage.StudyParticipant
	var err error
	if op == "add" {
		p, err = storage.CreateStudyParticipant(u.StudyId, upn)
		if err != nil {
			if errors.Is(err, storage.ParticipantAlreadyExistsError) {
				msg = url.QueryEscape(fmt.Sprintf("A participant with UPN %s already exists.", upn))
				target := "./participants?msg=" + msg
				c.Redirect(http.StatusSeeOther, target)
				return
			}
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
			return
		}
		msg = url.QueryEscape("UPN added successfully.")
	} else {
		// op == edit
		p, err = storage.GetStudyParticipant(u.StudyId, upn)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
			return
		}
		if p == nil {
			msg = url.QueryEscape("Participant not found.")
			target := "./participants?msg=" + msg
			c.Redirect(http.StatusSeeOther, target)
			return
		}
		msg = url.QueryEscape("Participant info updated successfully.")
	}
	// process edits
	if memo != p.Memo {
		if memo == "" {
			msg = "Assignment memo cannot be blank."
			editAgain = true
		} else if err = p.UpdateAssignment(memo); err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
			return
		}
	}
	// edits to apiKey or voiceId must be processed together
	if apiKey != p.ApiKey || voiceId != p.VoiceId {
		allowChanges := true
		voiceName := ""
		ok := false
		if p.Started > 0 {
			if apiKey == "" || voiceId == "" {
				allowChanges = false
				msg = "You can't remove ElevenLabs settings once the participant has used the app."
				editAgain = true
			} else if voiceName, ok, err = services.ElevenValidateVoiceId(apiKey, voiceId); !ok || err != nil {
				allowChanges = false
				msg = url.QueryEscape("The ElevenLabs API key and voice ID are invalid or incompatible.")
				editAgain = true
			}
		}
		// first process API Key changes
		if allowChanges && apiKey != p.ApiKey {
			if apiKey == "" {
				// if you clear the API key, that clears the voiceID!
				allowChanges = false
			}
			if ok, err := p.UpdateApiKey(apiKey); err != nil {
				c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
				return
			} else if !ok {
				allowChanges = false
				msg = url.QueryEscape("Invalid API key.")
				editAgain = true
			}
		}
		if allowChanges && voiceId != p.VoiceId {
			if p.ApiKey == "" {
				allowChanges = false
				msg = url.QueryEscape("Can't set voice ID without an API key.")
				editAgain = true
			} else if ok, err := p.UpdateVoiceId(voiceId); err != nil {
				c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
				return
			} else if !ok {
				allowChanges = false
				msg = url.QueryEscape("Invalid voice ID.")
				editAgain = true
			}
		}
		if allowChanges && p.Started > 0 {
			// update the user to their new settings
			didUpdate, err := storage.UpdateSpeechSettings(p.ProfileId, apiKey, voiceId, voiceName)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "database failure"})
				return
			}
			if didUpdate {
				if err := storage.ProfileClientSpeechDidUpdate(p.ProfileId, "none"); err != nil {
					middleware.CtxLog(c).Info("ignoring update notifications error",
						zap.String("profileId", p.ProfileId), zap.Error(err))
				}
			}
		}
	}
	target := "./participants?msg=" + msg
	if editAgain {
		target = fmt.Sprintf("./participants?edit=%s&msg=%s", upn, msg)
	}
	c.Redirect(http.StatusSeeOther, target)
}

func GetReportsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || u.StudyId == "" || !u.HasRole(storage.AdminRoleResearcher) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	study, _ := storage.GetStudy(u.StudyId)
	if study == nil {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	if generateId := c.Query("generate"); generateId != "" {
		report, err := storage.GetStudyReport(u.StudyId, generateId)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
			return
		}
		if report == nil {
			// shouldn't happen
			msg := url.QueryEscape("Report not found.")
			c.Redirect(http.StatusSeeOther, "./reports?msg="+msg)
			return
		}
		if err = report.Generate(); err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
			return
		}
		msg := url.QueryEscape("Report generated successfully.")
		c.Redirect(http.StatusSeeOther, "./reports?msg="+msg)
	}
	message := c.Query("msg")
	deleteId := c.Query("delete")
	// create the list of existing reports
	reports, err := storage.FetchAllStudyReports(u.StudyId)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	slices.SortFunc(reports, CompareReportsFunc(c.Query("sort")))
	reportList := make([]map[string]string, 0, len(reports))
	for _, r := range reports {
		if r.ReportId == deleteId || r.Generated == 0 {
			if err := r.Delete(); err != nil {
				c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
				return
			}
			continue
		}
		var restricted string
		if len(r.Upns) > 0 {
			restricted = "Yes"
		}
		reportList = append(reportList, map[string]string{
			"Id":        r.ReportId,
			"Name":      r.Name,
			"Type":      r.Type,
			"Start":     formatDate(r.Start),
			"End":       formatDate(r.End),
			"Upns":      restricted,
			"Generated": formatDateTime(r.Generated),
			"Filename":  r.Filename,
		})
	}
	// create select for report generation
	participants, err := storage.GetAllStudyParticipants(u.StudyId)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	slices.SortFunc(participants, CompareParticipantsFunc(""))
	upns := make([]string, 0, len(participants))
	for _, p := range participants {
		if p.Started == 0 {
			continue
		}
		upns = append(upns, p.Upn)
	}
	c.HTML(http.StatusOK, "admin/reports.tmpl.html",
		gin.H{"Study": study.Name, "Upns": upns, "Message": message, "Reports": reportList})
}

func PostReportsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || u.StudyId == "" || !u.HasRole(storage.AdminRoleResearcher) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	study, err := storage.GetStudy(u.StudyId)
	if err != nil || study == nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	op := c.PostForm("op")
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		// shouldn't happen
		message := url.QueryEscape("The report name must not be empty.")
		c.Redirect(http.StatusSeeOther, "./reports?msg="+message)
		return
	}
	// create the report object
	var r *storage.StudyReport
	if op == storage.ReportTypeLines {
		startString, endString := c.PostForm("start"), c.PostForm("end")
		start, end, err := storage.ComputeReportDates(startString, endString, "2006-01-02")
		if err != nil {
			// shouldn't happen
			middleware.CtxLog(c).Info("Invalid date in a posted report request",
				zap.String("start", startString), zap.String("end", endString), zap.Error(err))
			message := url.QueryEscape("Invalid start or end date.")
			c.Redirect(http.StatusSeeOther, "./reports?msg="+message)
			return
		}
		upns := c.PostFormArray("upns")
		r = storage.NewStudyReport(study.Id, name, storage.ReportTypeLines, start, end, upns)
	} else if op == storage.ReportTypePhrases {
		r = storage.NewStudyReport(study.Id, name, storage.ReportTypePhrases, 0, 0, nil)
	} else {
		// shouldn't happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	if err := r.Generate(); err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	msg := url.QueryEscape("Report generated successfully.")
	c.Redirect(http.StatusSeeOther, "./reports?msg="+msg)
}

func DownloadReportHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || u.StudyId == "" || !u.HasRole(storage.AdminRoleResearcher) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	report, err := storage.GetStudyReport(u.StudyId, c.Param("reportId"))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if report == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	data, err := report.Retrieve()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer data.Close()
	//goland:noinspection SpellCheckingInspection
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Status(http.StatusOK)
	c.Stream(func(w io.Writer) bool {
		if _, err := io.Copy(w, data); err != nil {
			return true
		}
		return false
	})
}

func GetAdminsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || !u.HasRole(storage.AdminRoleSuperAdmin) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	users, err := storage.GetAllAdminUsers()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	deleteId := c.Query("delete")
	editId := c.Query("edit")
	message := c.Query("msg")
	var editUser map[string]string
	slices.SortFunc(users, func(a, b *storage.AdminUser) int { return strings.Compare(a.Email, b.Email) })
	userList := make([]map[string]string, 0, len(users))
	for _, user := range users {
		if !user.HasRole(storage.AdminRoleSuperAdmin) {
			continue
		}
		if deleteId == user.Id {
			if deleteId == u.Id {
				msg := url.QueryEscape("You can't delete yourself!")
				c.Redirect(http.StatusSeeOther, "./admins?msg="+msg)
				return
			} else {
				if err := storage.DeleteSuperAdmin(deleteId); err != nil {
					message = fmt.Sprintf("Failed to delete %s!", user.Email)
					deleteId = ""
				} else {
					msg := url.QueryEscape("User deleted successfully.")
					c.Redirect(http.StatusSeeOther, "./admins?msg="+msg)
					return
				}
			}
		} else if editId == user.Id {
			editUser = map[string]string{"Id": user.Id, "Email": user.Email}
			editId = ""
		}
		userMap := map[string]string{"Id": user.Id, "Email": user.Email}
		userList = append(userList, userMap)
	}
	if deleteId != "" || editId != "" {
		// didn't find this user, clear the query and try again
		c.Redirect(http.StatusSeeOther, "./admins")
		return
	}
	c.HTML(http.StatusOK, "admin/admins.tmpl.html",
		gin.H{"Users": userList, "Edit": editUser, "Message": message})
}

func PostAdminsHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || !u.HasRole(storage.AdminRoleSuperAdmin) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	op := c.PostForm("op")
	if op == "edit" {
		userId := c.PostForm("id")
		email := strings.TrimSpace(c.PostForm("email"))
		if !emailPattern.MatchString(email) {
			msg := url.QueryEscape("You must provide a valid email address.")
			target := fmt.Sprintf("./admins?edit=%s&msg=%s", userId, msg)
			c.Redirect(http.StatusSeeOther, target)
			return
		}
		u, _ := storage.GetAdminUser(userId)
		if u == nil || !u.HasRole(storage.AdminRoleSuperAdmin) {
			// should never happen
			msg := url.QueryEscape("User not found.")
			target := "./admins?msg=" + msg
			c.Redirect(http.StatusSeeOther, target)
			return
		}
		u.Email = email
		if err := storage.SaveAdminUser(u); err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
			return
		}
		msg := url.QueryEscape("Admin updated successfully.")
		c.Redirect(http.StatusSeeOther, "./admins?msg="+msg)
	} else {
		// op == add
		email := strings.TrimSpace(c.PostForm("email"))
		user, err := storage.LookupAdminUser(email)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
			return
		}
		if user != nil {
			if user.HasRole(storage.AdminRoleSuperAdmin) {
				msg := url.QueryEscape(fmt.Sprintf("An admin with email %s already exists.", email))
				target := "./admins?msg=" + msg
				c.Redirect(http.StatusSeeOther, target)
				return
			}
		} else {
			user = storage.NewAdminUser(email, u.StudyId)
		}
		user.SetRoles([]storage.AdminRole{storage.AdminRoleSuperAdmin})
		if err := storage.SaveAdminUser(user); err != nil {
			c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
			return
		}
		msg := url.QueryEscape("Admin added successfully.")
		target := "./admins?msg=" + msg
		c.Redirect(http.StatusSeeOther, target)
	}
}

func GetStudiesHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || !u.HasRole(storage.AdminRoleSuperAdmin) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	studies, err := storage.GetAllStudies()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	deleteId := c.Query("delete")
	editId := c.Query("edit")
	message := c.Query("msg")
	var editStudy map[string]string
	slices.SortFunc(studies, func(a, b *storage.Study) int { return strings.Compare(a.Name, b.Name) })
	studyList := make([]map[string]string, 0, len(studies))
	for _, study := range studies {
		if deleteId == study.Id {
			if err := storage.DeleteStudy(deleteId); err != nil {
				message = fmt.Sprintf("Failed to delete %s!", study.Name)
				deleteId = ""
			} else {
				msg := url.QueryEscape("User deleted successfully.")
				c.Redirect(http.StatusSeeOther, "./studies?msg="+msg)
				return
			}
		} else if editId == study.Id {
			editStudy = map[string]string{"Id": study.Id, "Name": study.Name, "Email": study.AdminEmail}
			editId = ""
		}
		studyMap := map[string]string{"Id": study.Id, "Name": study.Name, "Email": study.AdminEmail}
		studyList = append(studyList, studyMap)
	}
	if deleteId != "" || editId != "" {
		// didn't find this study, clear the query and try again
		c.Redirect(http.StatusSeeOther, "./admins")
		return
	}
	c.HTML(http.StatusOK, "admin/studies.tmpl.html",
		gin.H{"Studies": studyList, "Edit": editStudy, "Message": message})
}

func PostStudiesHandler(c *gin.Context) {
	u := getAuthenticatedUser(c)
	if u == nil || !u.HasRole(storage.AdminRoleSuperAdmin) {
		// should never happen
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	op := c.PostForm("op")
	name := strings.TrimSpace(c.PostForm("name"))
	email := strings.TrimSpace(c.PostForm("email"))
	if len(name) < 5 {
		msg := url.QueryEscape("Study names must have at least five characters.")
		target := fmt.Sprintf("./studies?msg=%s", msg)
		c.Redirect(http.StatusSeeOther, target)
		return
	}
	if !emailPattern.MatchString(email) {
		msg := url.QueryEscape("You must provide a valid email address.")
		target := fmt.Sprintf("./studies?msg=%s", msg)
		c.Redirect(http.StatusSeeOther, target)
		return
	}
	var s *storage.Study
	if op == "edit" {
		studyId := c.PostForm("id")
		s, _ = storage.GetStudy(studyId)
		if s == nil {
			// should never happen
			msg := url.QueryEscape("Study not found.")
			target := "./studies?msg=" + msg
			c.Redirect(http.StatusSeeOther, target)
			return
		}
		s.Name = name
		s.AdminEmail = email
	} else {
		s = &storage.Study{Id: uuid.NewString(), Name: name, AdminEmail: email, Active: true}
	}
	if err := storage.EnsureStudyAdminUser(s.Id, email); err != nil {
		msg := url.QueryEscape(fmt.Sprintf("Cannot use %s as the admin for this study.", email))
		c.Redirect(http.StatusSeeOther, "./studies?msg="+msg)
		return
	}
	if err := s.Save(); err != nil {
		c.HTML(http.StatusInternalServerError, "admin/error.tmpl.html", gin.H{"logout": "./logout"})
		return
	}
	var msg string
	if op == "edit" {
		msg = url.QueryEscape("Study updated successfully.")
	} else {
		msg = url.QueryEscape("Study added successfully.")
	}
	c.Redirect(http.StatusSeeOther, "./studies?msg="+msg)
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
		configuredCompare := func(a, b *storage.StudyParticipant) int {
			if a.ApiKey != "" && b.ApiKey != "" {
				if a.VoiceId != "" && b.VoiceId != "" {
					return upnCompare
				} else if a.VoiceId != "" {
					return -1
				} else if b.VoiceId != "" {
					return 1
				} else {
					return upnCompare
				}
			} else if a.ApiKey != "" {
				return -1
			} else if b.ApiKey != "" {
				return 1
			} else {
				return upnCompare
			}
		}
		switch sort {
		case "assigned":
			return timeCompare(a.Assigned, b.Assigned, upnCompare)
		case "configured":
			return configuredCompare(a, b)
		case "start":
			return timeCompare(a.Started, b.Started, upnCompare)
		case "end":
			return timeCompare(a.Finished, b.Finished, upnCompare)
		default:
			return upnCompare
		}
	}
}

func CompareReportsFunc(sort string) func(a, b *storage.StudyReport) int {
	return func(a, b *storage.StudyReport) int {
		nameCompare := strings.Compare(a.Name, b.Name)
		switch sort {
		case "type":
			return nameCompare
		case "start":
			return timeCompare(a.Start, b.Start, nameCompare)
		case "end":
			return timeCompare(a.End, b.Start, nameCompare)
		case "restricted":
			if (len(a.Upns) == 0 && len(b.Upns) == 0) || (len(a.Upns) > 0 && len(b.Upns) > 0) {
				return nameCompare
			} else if len(a.Upns) == 0 {
				return 1
			} else {
				return -1
			}
		case "generated":
			return timeCompare(a.Generated, b.Generated, nameCompare)
		case "schedule":
			if a.Schedule == b.Schedule {
				return nameCompare
			} else if a.Schedule == "" {
				return 1
			} else if b.Schedule == "" {
				return -1
			} else {
				return strings.Compare(a.Schedule, b.Schedule)
			}
		default:
			return strings.Compare(a.Filename, b.Filename)
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
		pMap["Assigned"] = formatDate(p.Assigned) + " (" + memo + ")"
	}
	if p.ApiKey != "" {
		if p.VoiceId != "" {
			pMap["Configured"] = "Yes"
		} else {
			pMap["Configured"] = "API Key Only"
		}
	} else {
		pMap["Configured"] = "No"
	}
	if p.Started > 0 {
		pMap["Started"] = formatDate(p.Started)
	}
	if p.Finished > 0 {
		pMap["Finished"] = formatDate(p.Finished)
	}
	return pMap
}

func timeCompare(t1, t2 int64, fallback int) int {
	if t1 == t2 {
		return fallback
	} else if t1 == 0 {
		return 1
	} else if t2 == 0 {
		return -1
	} else if t1 < t2 {
		return -1
	} else {
		return 1
	}
}

func formatDate(t int64) string {
	if t == 0 {
		return ""
	}
	return time.UnixMilli(t).In(storage.AdminTZ).Format("01/02/2006")
}

func formatDateTime(t int64) string {
	if t == 0 {
		return ""
	}
	return time.UnixMilli(t).In(storage.AdminTZ).Format("01/02/2006 3:04pm MST")
}
