package templates

import (
	"errors"
	"fmt"
)

// Campaign status constants
const (
	CampaignStatusScheduled = "SCHEDULED"
	CampaignStatusSending   = "SENDING"
	CampaignStatusSent      = "SENT"
	CampaignStatusCompleted = "COMPLETED"
	CampaignStatusPaused    = "PAUSED"
	CampaignStatusCancelled = "CANCELLED"
)

// getCampaignAdminSubjectAndPreheader returns subject and preheader for admin campaign notifications.
func getCampaignAdminSubjectAndPreheader(status, campaignName string) (string, string) {
	switch status {
	case CampaignStatusScheduled:
		return fmt.Sprintf("Campaign Scheduled - %s", campaignName),
			fmt.Sprintf("Campaign \"%s\" has been scheduled for delivery.", campaignName)
	case CampaignStatusSending:
		return fmt.Sprintf("Campaign Sending - %s", campaignName),
			fmt.Sprintf("Campaign \"%s\" is now being sent.", campaignName)
	case CampaignStatusSent:
		return fmt.Sprintf("Campaign Sent - %s", campaignName),
			fmt.Sprintf("Campaign \"%s\" has been sent successfully.", campaignName)
	case CampaignStatusCompleted:
		return fmt.Sprintf("Campaign Completed - %s", campaignName),
			fmt.Sprintf("Campaign \"%s\" has completed. Review the performance stats.", campaignName)
	case CampaignStatusPaused:
		return fmt.Sprintf("Campaign Paused - %s", campaignName),
			fmt.Sprintf("Campaign \"%s\" has been paused.", campaignName)
	case CampaignStatusCancelled:
		return fmt.Sprintf("[CANCELLED] Campaign - %s", campaignName),
			fmt.Sprintf("Campaign \"%s\" has been cancelled.", campaignName)
	default:
		return fmt.Sprintf("Campaign Update - %s", campaignName),
			fmt.Sprintf("Campaign \"%s\" has been updated.", campaignName)
	}
}

// RenderCampaignAdmin renders the admin campaign notification template.
// Set data.CampaignStatus before calling (e.g., "SENT", "COMPLETED", "CANCELLED").
// Returns subject, body, and error.
func (r *Renderer) RenderCampaignAdmin(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if data == nil {
		return "", "", errors.New("email data is nil")
	}
	if data.CampaignName == "" {
		return "", "", errors.New("campaign name is required")
	}

	data.Subject, data.Preheader = getCampaignAdminSubjectAndPreheader(data.CampaignStatus, data.CampaignName)

	body, err := r.Render("campaign_admin", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render campaign_admin: %w", err)
	}

	return data.Subject, body, nil
}

// RenderCampaignBroadcast renders the promotional/broadcast campaign email.
// Returns subject, body, and error.
func (r *Renderer) RenderCampaignBroadcast(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if data == nil {
		return "", "", errors.New("email data is nil")
	}

	if data.Subject == "" {
		data.Subject = data.CampaignName
	}

	body, err := r.Render("campaign_broadcast", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render campaign_broadcast: %w", err)
	}

	return data.Subject, body, nil
}

// RenderCampaignNewsletter renders the newsletter campaign email.
// Returns subject, body, and error.
func (r *Renderer) RenderCampaignNewsletter(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if data == nil {
		return "", "", errors.New("email data is nil")
	}

	if data.Subject == "" {
		data.Subject = data.CampaignName
	}

	body, err := r.Render("campaign_newsletter", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render campaign_newsletter: %w", err)
	}

	return data.Subject, body, nil
}
