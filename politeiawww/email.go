// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"net/url"
	"text/template"

	"github.com/dajohi/goemail"

	v1 "github.com/decred/politeia/politeiawww/api/v1"
	"github.com/decred/politeia/politeiawww/user"
)

func createBody(tpl *template.Template, tplData interface{}) (string, error) {
	var buf bytes.Buffer
	err := tpl.Execute(&buf, tplData)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (p *politeiawww) createEmailLink(path, email, token string) (string, error) {
	l, err := url.Parse(p.cfg.WebServerAddress + path)
	if err != nil {
		return "", err
	}

	q := l.Query()
	if email != "" {
		q.Set("email", email)
	}
	if token != "" {
		q.Set("verificationtoken", token)
	}
	l.RawQuery = q.Encode()

	return l.String(), nil
}

// sendEmailTo sends an email with the given subject and body to a single
// address.
func (p *politeiawww) sendEmailTo(subject, body, toAddress string) error {
	if p.smtp.disabled {
		return nil
	}
	return p.smtp.sendEmail(subject, body, func(msg *goemail.Message) error {
		msg.AddTo(toAddress)
		return nil
	})
}

// emailNewUserVerificationLink emails the link with the new user verification
// token if the email server is set up.
func (p *politeiawww) emailNewUserVerificationLink(email, token, username string) error {
	if p.smtp.disabled {
		return nil
	}

	link, err := p.createEmailLink(v1.RouteVerifyNewUser, email,
		token)
	if err != nil {
		return err
	}

	tplData := newUserEmailTemplateData{
		Username: username,
		Email:    email,
		Link:     link,
	}

	subject := "Verify Your Email"
	body, err := createBody(templateNewUserEmail, &tplData)
	if err != nil {
		return err
	}

	return p.sendEmailTo(subject, body, email)
}

// emailResetPasswordVerificationLink emails the link with the reset password
// verification token if the email server is set up.
func (p *politeiawww) emailResetPasswordVerificationLink(email, token string) error {
	if p.smtp.disabled {
		return nil
	}

	link, err := p.createEmailLink(v1.RouteResetPassword, email,
		token)
	if err != nil {
		return err
	}

	tplData := resetPasswordEmailTemplateData{
		Email: email,
		Link:  link,
	}

	subject := "Reset Your Password"
	body, err := createBody(templateResetPasswordEmail, &tplData)
	if err != nil {
		return err
	}

	return p.sendEmailTo(subject, body, email)
}

// emailAuthorForVettedProposal sends an email notification for a new proposal
// becoming vetted to the proposal's author.
func (p *politeiawww) emailAuthorForVettedProposal(proposal *v1.ProposalRecord, authorUser *user.User, adminUser *user.User) error {
	if p.smtp.disabled {
		return nil
	}

	l, err := url.Parse(p.cfg.WebServerAddress + "/proposals/" +
		proposal.CensorshipRecord.Token)
	if err != nil {
		return err
	}

	if authorUser.EmailNotifications&
		uint64(v1.NotificationEmailMyProposalStatusChange) == 0 {
		return nil
	}

	tplData := proposalStatusChangeTemplateData{
		Link:               l.String(),
		Name:               proposal.Name,
		StatusChangeReason: proposal.StatusChangeMessage,
	}

	subject := "Your Proposal Has Been Published"
	body, err := createBody(templateProposalVettedForAuthor, &tplData)
	if err != nil {
		return err
	}

	return p.sendEmailTo(subject, body, authorUser.Email)
}

// emailAuthorForCensoredProposal sends an email notification for a new
// proposal becoming censored to the proposal's author.
func (p *politeiawww) emailAuthorForCensoredProposal(proposal *v1.ProposalRecord, authorUser *user.User, adminUser *user.User) error {
	if p.smtp.disabled {
		return nil
	}

	l, err := url.Parse(p.cfg.WebServerAddress + "/proposals/" +
		proposal.CensorshipRecord.Token)
	if err != nil {
		return err
	}

	if authorUser.EmailNotifications&
		uint64(v1.NotificationEmailMyProposalStatusChange) == 0 {
		return nil
	}

	tplData := proposalStatusChangeTemplateData{
		Link:               l.String(),
		Name:               proposal.Name,
		StatusChangeReason: proposal.StatusChangeMessage,
	}

	subject := "Your Proposal Has Been Censored"
	body, err := createBody(templateProposalCensoredForAuthor, &tplData)
	if err != nil {
		return err
	}

	return p.sendEmailTo(subject, body, authorUser.Email)
}

// emailUsersForVettedProposal sends an email notification for a new proposal
// becoming vetted.
func (p *politeiawww) emailUsersForVettedProposal(proposal *v1.ProposalRecord, authorUser *user.User, adminUser *user.User) error {
	if p.smtp.disabled {
		return nil
	}

	// Create the template data.
	l, err := url.Parse(p.cfg.WebServerAddress + "/proposals/" +
		proposal.CensorshipRecord.Token)
	if err != nil {
		return err
	}

	tplData := proposalStatusChangeTemplateData{
		Link:     l.String(),
		Name:     proposal.Name,
		Username: authorUser.Username,
	}

	// Send email to users.
	subject := "New Proposal Published"
	body, err := createBody(templateProposalVetted, &tplData)
	if err != nil {
		return err
	}

	return p.smtp.sendEmail(subject, body, func(msg *goemail.Message) error {
		// Add user emails to the goemail.Message
		return p.db.AllUsers(func(u *user.User) {
			// Don't notify the user under certain conditions.
			if u.NewUserPaywallTx == "" || u.Deactivated ||
				u.ID == adminUser.ID || u.ID == authorUser.ID ||
				(u.EmailNotifications&
					uint64(v1.NotificationEmailRegularProposalVetted)) == 0 {
				return
			}

			msg.AddBCC(u.Email)
		})
	})
}

// emailUsersForEditedProposal sends an email notification for a proposal being
// edited.
func (p *politeiawww) emailUsersForEditedProposal(proposal *v1.ProposalRecord, authorUser *user.User) error {
	if p.smtp.disabled {
		return nil
	}

	// Create the template data.
	l, err := url.Parse(p.cfg.WebServerAddress + "/proposals/" +
		proposal.CensorshipRecord.Token)
	if err != nil {
		return err
	}

	tplData := proposalEditedTemplateData{
		Link:     l.String(),
		Name:     proposal.Name,
		Version:  proposal.Version,
		Username: authorUser.Username,
	}

	// Send email to users.
	subject := "Proposal Edited"
	body, err := createBody(templateProposalEdited, &tplData)
	if err != nil {
		return err
	}

	return p.smtp.sendEmail(subject, body, func(msg *goemail.Message) error {
		// Add user emails to the goemail.Message
		return p.db.AllUsers(func(u *user.User) {
			// Don't notify the user under certain conditions.
			if u.NewUserPaywallTx == "" || u.Deactivated ||
				u.ID == authorUser.ID ||
				(u.EmailNotifications&
					uint64(v1.NotificationEmailRegularProposalEdited)) == 0 {
				return
			}

			msg.AddBCC(u.Email)
		})
	})
}

// emailUsersForProposalVoteStarted sends an email notification for a proposal
// entering the voting state.
func (p *politeiawww) emailUsersForProposalVoteStarted(proposal *v1.ProposalRecord, authorUser *user.User, adminUser *user.User) error {
	if p.smtp.disabled {
		return nil
	}

	// Create the template data.
	l, err := url.Parse(p.cfg.WebServerAddress + "/proposals/" +
		proposal.CensorshipRecord.Token)
	if err != nil {
		return err
	}

	tplData := proposalVoteStartedTemplateData{
		Link:     l.String(),
		Name:     proposal.Name,
		Username: authorUser.Username,
	}

	// Send email to author.
	if authorUser.EmailNotifications&
		uint64(v1.NotificationEmailMyProposalVoteStarted) != 0 {

		subject := "Your Proposal Has Started Voting"
		body, err := createBody(templateProposalVoteStartedForAuthor, &tplData)
		if err != nil {
			return err
		}

		err = p.sendEmailTo(subject, body, authorUser.Email)
		if err != nil {
			return err
		}
	}

	subject := "Voting Started for Proposal"
	body, err := createBody(templateProposalVoteStarted, &tplData)
	if err != nil {
		return err
	}

	return p.smtp.sendEmail(subject, body, func(msg *goemail.Message) error {
		// Add user emails to the goemail.Message
		return p.db.AllUsers(func(u *user.User) {
			// Don't notify the user under certain conditions.
			if u.NewUserPaywallTx == "" || u.Deactivated ||
				u.ID == adminUser.ID ||
				u.ID == authorUser.ID ||
				(u.EmailNotifications&
					uint64(v1.NotificationEmailRegularProposalVoteStarted)) == 0 {
				return
			}

			msg.AddBCC(u.Email)
		})
	})
}

func (p *politeiawww) emailAdminsForNewSubmittedProposal(token string, propName string, username string, userEmail string) error {
	if p.smtp.disabled {
		return nil
	}

	l, err := url.Parse(p.cfg.WebServerAddress + "/proposals/" + token)
	if err != nil {
		return err
	}

	tplData := newProposalSubmittedTemplateData{
		Link:     l.String(),
		Name:     propName,
		Username: username,
		Email:    userEmail,
	}

	subject := "New Proposal Submitted"
	body, err := createBody(templateNewProposalSubmitted, &tplData)
	if err != nil {
		return err
	}

	return p.smtp.sendEmail(subject, body, func(msg *goemail.Message) error {
		// Add admin emails to the goemail.Message
		return p.db.AllUsers(func(u *user.User) {
			if !u.Admin || u.Deactivated ||
				(u.EmailNotifications&
					uint64(v1.NotificationEmailAdminProposalNew) == 0) {
				return
			}
			msg.AddBCC(u.Email)
		})
	})
}

func (p *politeiawww) emailAdminsForProposalVoteAuthorized(proposal *v1.ProposalRecord, authorUser *user.User) error {
	if p.smtp.disabled {
		return nil
	}

	l, err := url.Parse(fmt.Sprintf("%v/proposals/%v", p.cfg.WebServerAddress,
		proposal.CensorshipRecord.Token))
	if err != nil {
		return err
	}

	tplData := proposalVoteAuthorizedTemplateData{
		Link:     l.String(),
		Name:     proposal.Name,
		Username: authorUser.Username,
		Email:    authorUser.Email,
	}

	subject := "Proposal Authorized To Start Voting"
	body, err := createBody(templateProposalVoteAuthorized, &tplData)
	if err != nil {
		return err
	}

	return p.smtp.sendEmail(subject, body, func(msg *goemail.Message) error {
		// Add admin emails to the goemail.Message
		return p.db.AllUsers(func(u *user.User) {
			if !u.Admin || u.Deactivated ||
				(u.EmailNotifications&
					uint64(v1.NotificationEmailAdminProposalVoteAuthorized) == 0) {
				return
			}
			msg.AddBCC(u.Email)
		})
	})
}

// emailAuthorForCommentOnProposal sends an email notification to a proposal
// author for a new comment.
func (p *politeiawww) emailAuthorForCommentOnProposal(proposal *v1.ProposalRecord, authorUser *user.User, commentID, username string) error {
	if p.smtp.disabled {
		return nil
	}

	l, err := url.Parse(fmt.Sprintf("%v/proposals/%v/comments/%v",
		p.cfg.WebServerAddress, proposal.CensorshipRecord.Token, commentID))
	if err != nil {
		return err
	}

	if authorUser.EmailNotifications&
		uint64(v1.NotificationEmailCommentOnMyProposal) == 0 {
		return nil
	}

	// Don't send email when author comments on own proposal
	if username == authorUser.Username {
		return nil
	}

	tplData := commentReplyOnProposalTemplateData{
		Commenter:    username,
		ProposalName: proposal.Name,
		CommentLink:  l.String(),
	}

	subject := "New Comment On Your Proposal"
	body, err := createBody(templateCommentReplyOnProposal, &tplData)
	if err != nil {
		return err
	}

	return p.sendEmailTo(subject, body, authorUser.Email)
}

// emailAuthorForCommentOnComment sends an email notification to a comment
// author for a new comment reply.
func (p *politeiawww) emailAuthorForCommentOnComment(proposal *v1.ProposalRecord, authorUser *user.User, commentID, username string) error {
	if p.smtp.disabled {
		return nil
	}

	l, err := url.Parse(fmt.Sprintf("%v/proposals/%v/comments/%v",
		p.cfg.WebServerAddress, proposal.CensorshipRecord.Token, commentID))
	if err != nil {
		return err
	}

	if authorUser.EmailNotifications&
		uint64(v1.NotificationEmailCommentOnMyComment) == 0 {
		return nil
	}

	// Don't send email when author replies to his own comment
	if username == authorUser.Username {
		return nil
	}

	tplData := commentReplyOnCommentTemplateData{
		Commenter:    username,
		ProposalName: proposal.Name,
		CommentLink:  l.String(),
	}

	subject := "New Comment On Your Comment"
	body, err := createBody(templateCommentReplyOnComment, &tplData)
	if err != nil {
		return err
	}

	return p.sendEmailTo(subject, body, authorUser.Email)
}

// emailUpdateUserKeyVerificationLink emails the link with the verification
// token used for setting a new key pair if the email server is set up.
func (p *politeiawww) emailUpdateUserKeyVerificationLink(email, publicKey, token string) error {
	if p.smtp.disabled {
		return nil
	}

	link, err := p.createEmailLink(v1.RouteVerifyUpdateUserKey, "", token)
	if err != nil {
		return err
	}

	tplData := updateUserKeyEmailTemplateData{
		Email:     email,
		PublicKey: publicKey,
		Link:      link,
	}

	subject := "Verify Your New Identity"
	body, err := createBody(templateUpdateUserKeyEmail, &tplData)
	if err != nil {
		return err
	}

	return p.sendEmailTo(subject, body, email)
}

// emailUserPasswordChanged notifies the user that his password was changed,
// and verifies if he was the author of this action, for security purposes.
func (p *politeiawww) emailUserPasswordChanged(email string) error {
	if p.smtp.disabled {
		return nil
	}

	tplData := userPasswordChangedTemplateData{
		Email: email,
	}

	subject := "Password Changed - Security Verification"
	body, err := createBody(templateUserPasswordChanged, &tplData)
	if err != nil {
		return err
	}

	return p.sendEmailTo(subject, body, email)
}

// emailUserLocked notifies the user its account has been locked and emails the
// link with the reset password verification token if the email server is set
// up.
func (p *politeiawww) emailUserLocked(email string) error {
	if p.smtp.disabled {
		return nil
	}

	link, err := p.createEmailLink(ResetPasswordGuiRoute,
		email, "")
	if err != nil {
		return err
	}

	tplData := userLockedResetPasswordEmailTemplateData{
		Email: email,
		Link:  link,
	}

	subject := "Locked Account - Reset Your Password"
	body, err := createBody(templateUserLockedResetPassword, &tplData)
	if err != nil {
		return err
	}

	return p.sendEmailTo(subject, body, email)
}
