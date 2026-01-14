// Package templates provides email template rendering for the notification service.
// This file contains all review-related email render methods using the unified review_customer template.
package templates

import (
	"errors"
	"fmt"
	"html/template"
)

// Review status constants for the unified review_customer template
const (
	ReviewStatusSubmitted = "SUBMITTED"
	ReviewStatusApproved  = "APPROVED"
	ReviewStatusRejected  = "REJECTED"
)

// DefaultMaxRating is the default maximum rating value.
const DefaultMaxRating = 5

// ErrMissingProductName is returned when product name is empty.
var ErrMissingProductName = errors.New("product name is required")

// validateReviewData validates required review fields and sets defaults.
func validateReviewData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	if data.ProductName == "" {
		return ErrMissingProductName
	}
	// Set default max rating if not provided
	if data.MaxRating == 0 {
		data.MaxRating = DefaultMaxRating
	}
	// Ensure rating is within bounds
	if data.Rating < 0 {
		data.Rating = 0
	}
	if data.Rating > data.MaxRating {
		data.Rating = data.MaxRating
	}
	return nil
}

// generateRatingStars generates HTML star display for ratings.
// Uses Unicode filled star (★) and empty star (☆) characters.
func generateRatingStars(rating, maxRating int) template.HTML {
	if maxRating <= 0 {
		maxRating = DefaultMaxRating
	}
	if rating < 0 {
		rating = 0
	}
	if rating > maxRating {
		rating = maxRating
	}

	stars := ""
	for i := 1; i <= maxRating; i++ {
		if i <= rating {
			stars += "★" // Filled star (Unicode U+2605)
		} else {
			stars += "☆" // Empty star (Unicode U+2606)
		}
	}
	return template.HTML(stars)
}

// RenderReviewCustomer renders the unified review customer template.
// The ReviewStatus field determines the content displayed.
// Returns subject, body, and error.
func (r *Renderer) RenderReviewCustomer(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateReviewData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	// Generate rating stars display
	data.RatingStars = generateRatingStars(data.Rating, data.MaxRating)

	// Set subject and preheader based on review status
	switch data.ReviewStatus {
	case ReviewStatusSubmitted:
		data.Subject = fmt.Sprintf("Thanks for Your Review of %s!", data.ProductName)
		data.Preheader = fmt.Sprintf("We've received your %d-star review. Here's a copy for your records.", data.Rating)
	case ReviewStatusApproved:
		data.Subject = fmt.Sprintf("Your Review Has Been Published! - %s", data.ProductName)
		data.Preheader = fmt.Sprintf("Great news! Your review for %s is now live.", data.ProductName)
	case ReviewStatusRejected:
		data.Subject = fmt.Sprintf("Review Update - %s", data.ProductName)
		data.Preheader = fmt.Sprintf("Your review for %s couldn't be published. Here's why.", data.ProductName)
	default:
		data.Subject = fmt.Sprintf("Review Update - %s", data.ProductName)
		data.Preheader = fmt.Sprintf("Here's an update on your review for %s.", data.ProductName)
	}

	body, err := r.Render("review_customer", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render review_customer: %w", err)
	}

	return data.Subject, body, nil
}

// RenderReviewSubmittedCustomer renders the review submitted customer copy template.
// This is a convenience wrapper around RenderReviewCustomer with status SUBMITTED.
// Returns subject, body, and error.
func (r *Renderer) RenderReviewSubmittedCustomer(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateReviewData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.ReviewStatus = ReviewStatusSubmitted
	return r.RenderReviewCustomer(data)
}

// RenderReviewSubmittedAdmin renders the review submitted admin notification template.
// Returns subject, body, and error.
func (r *Renderer) RenderReviewSubmittedAdmin(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateReviewData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.Subject = fmt.Sprintf("New Review Submitted - %s (%d Stars)", data.ProductName, data.Rating)
	data.Preheader = fmt.Sprintf("A new %d-star review needs moderation for %s.", data.Rating, data.ProductName)

	data.RatingStars = generateRatingStars(data.Rating, data.MaxRating)

	body, err := r.Render("review_submitted_admin", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render review_submitted_admin: %w", err)
	}

	return data.Subject, body, nil
}

// RenderReviewApproved renders the review approved notification template.
// This is a convenience wrapper around RenderReviewCustomer with status APPROVED.
// Returns subject, body, and error.
func (r *Renderer) RenderReviewApproved(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateReviewData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.ReviewStatus = ReviewStatusApproved
	return r.RenderReviewCustomer(data)
}

// RenderReviewRejected renders the review rejected notification template.
// This is a convenience wrapper around RenderReviewCustomer with status REJECTED.
// Returns subject, body, and error.
func (r *Renderer) RenderReviewRejected(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateReviewData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.ReviewStatus = ReviewStatusRejected
	return r.RenderReviewCustomer(data)
}
