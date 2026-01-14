// Package templates provides email template rendering for the notification service.
// This file contains customer, inventory, coupon, auth, and tenant email render methods.
package templates

import (
	"errors"
	"fmt"
)

// validateCustomerData validates customer-related data.
func validateCustomerData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	return nil
}

// validateCouponData validates required coupon fields.
func validateCouponData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	if data.CouponCode == "" {
		return errors.New("coupon code is required")
	}
	// Set default currency if not provided
	if data.Currency == "" {
		data.Currency = "$"
	}
	return nil
}

// RenderCustomerWelcome renders the customer welcome template.
// Returns subject, body, and error.
func (r *Renderer) RenderCustomerWelcome(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateCustomerData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	if data.BusinessName != "" {
		data.Subject = fmt.Sprintf("Welcome to %s!", data.BusinessName)
		data.Preheader = fmt.Sprintf("Thanks for joining %s! Start shopping and discover amazing products.", data.BusinessName)
	} else {
		data.Subject = "Welcome to Your New Account!"
		data.Preheader = "Thanks for joining! Start shopping and discover amazing products."
	}

	body, err := r.Render("customer_welcome", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render customer_welcome: %w", err)
	}

	return data.Subject, body, nil
}

// RenderLowStockAlert renders the low stock alert template.
// Returns subject, body, and error.
func (r *Renderer) RenderLowStockAlert(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if data == nil {
		return "", "", errors.New("email data is nil")
	}

	// Set defaults for affected products count
	if data.TotalAffectedProducts == 0 && len(data.Products) > 0 {
		data.TotalAffectedProducts = len(data.Products)
	}

	if data.TotalAffectedProducts > 0 {
		data.Subject = fmt.Sprintf("Low Stock Alert - %d Products Need Attention", data.TotalAffectedProducts)
		data.Preheader = fmt.Sprintf("%d products are running low on stock. Review inventory now.", data.TotalAffectedProducts)
	} else {
		data.Subject = "Low Stock Alert - Products Need Attention"
		data.Preheader = "Some products are running low on stock. Review inventory now."
	}

	body, err := r.Render("low_stock_alert", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render low_stock_alert: %w", err)
	}

	return data.Subject, body, nil
}

// RenderCouponApplied renders the coupon applied confirmation.
// Returns subject, body, and error.
func (r *Renderer) RenderCouponApplied(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateCouponData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.Subject = fmt.Sprintf("Coupon Applied - %s", data.CouponCode)
	if data.DiscountAmount != "" {
		data.Preheader = fmt.Sprintf("You saved %s%s on your order!", data.Currency, data.DiscountAmount)
	} else {
		data.Preheader = fmt.Sprintf("Coupon %s has been applied to your order!", data.CouponCode)
	}

	body, err := r.Render("coupon_applied", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render coupon_applied: %w", err)
	}

	return data.Subject, body, nil
}

// RenderCouponCreated renders the new coupon notification.
// Returns subject, body, and error.
func (r *Renderer) RenderCouponCreated(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateCouponData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	discount := data.DiscountValue
	if data.DiscountType == "percentage" {
		discount = discount + "%"
	} else if discount != "" {
		discount = data.Currency + discount
	}

	if discount != "" {
		data.Subject = fmt.Sprintf("Special Offer: Save %s with code %s", discount, data.CouponCode)
	} else {
		data.Subject = fmt.Sprintf("Special Offer: Use code %s", data.CouponCode)
	}
	data.Preheader = fmt.Sprintf("Use code %s to save on your next order!", data.CouponCode)

	body, err := r.Render("coupon_created", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render coupon_created: %w", err)
	}

	return data.Subject, body, nil
}

// RenderCouponExpired renders the coupon expired admin notification.
// Returns subject, body, and error.
func (r *Renderer) RenderCouponExpired(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateCouponData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.Subject = fmt.Sprintf("Coupon Expired - %s", data.CouponCode)
	data.Preheader = fmt.Sprintf("The coupon %s has expired.", data.CouponCode)

	body, err := r.Render("coupon_expired", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render coupon_expired: %w", err)
	}

	return data.Subject, body, nil
}

// RenderPasswordReset renders the password reset email.
// Returns subject, body, and error.
func (r *Renderer) RenderPasswordReset(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if data == nil {
		return "", "", errors.New("email data is nil")
	}

	data.Subject = "Password Reset Request"
	data.Preheader = "Use the code below to reset your password. This link expires in 15 minutes."

	body, err := r.Render("password_reset", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render password_reset: %w", err)
	}

	return data.Subject, body, nil
}

// RenderTenantWelcomePack renders the tenant onboarding welcome pack email.
// Returns subject, body, and error.
func (r *Renderer) RenderTenantWelcomePack(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if data == nil {
		return "", "", errors.New("email data is nil")
	}

	if data.BusinessName != "" {
		data.Subject = fmt.Sprintf("Welcome to Tesseract Hub - %s is Ready!", data.BusinessName)
		data.Preheader = fmt.Sprintf("Your store %s is now live! Access your admin panel and start selling.", data.BusinessName)
	} else {
		data.Subject = "Welcome to Tesseract Hub - Your Store is Ready!"
		data.Preheader = "Your store is now live! Access your admin panel and start selling."
	}

	body, err := r.Render("tenant_welcome_pack", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render tenant_welcome_pack: %w", err)
	}

	return data.Subject, body, nil
}

// RenderVerificationLink renders the email verification link email.
// Returns subject, body, and error.
func (r *Renderer) RenderVerificationLink(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if data == nil {
		return "", "", errors.New("email data is nil")
	}

	if data.BusinessName != "" {
		data.Subject = fmt.Sprintf("Verify your email for %s", data.BusinessName)
		data.Preheader = fmt.Sprintf("Click the link to verify your email and complete your %s setup.", data.BusinessName)
	} else {
		data.Subject = "Verify your email address"
		data.Preheader = "Click the link to verify your email and complete your account setup."
	}

	body, err := r.Render("verification_link", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render verification_link: %w", err)
	}

	return data.Subject, body, nil
}
