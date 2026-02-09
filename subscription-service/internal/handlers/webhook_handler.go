package handlers

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
	"subscription-service/internal/models"
	"subscription-service/internal/services"
)

type WebhookHandler struct {
	service *services.WebhookService
}

func NewWebhookHandler(service *services.WebhookService) *WebhookHandler {
	return &WebhookHandler{service: service}
}

func (h *WebhookHandler) HandleStripeWebhook(c *gin.Context) {
	const MaxBodyBytes = int64(65536)
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, MaxBodyBytes))
	if err != nil {
		log.Printf("Error reading webhook body: %v", err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Failed to read request body"})
		return
	}

	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	var event stripe.Event
	if webhookSecret != "" {
		event, err = webhook.ConstructEvent(body, c.GetHeader("Stripe-Signature"), webhookSecret)
		if err != nil {
			log.Printf("Webhook signature verification failed: %v", err)
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid signature"})
			return
		}
	} else {
		log.Println("WARNING: STRIPE_WEBHOOK_SECRET not set, skipping signature verification")
		if err := event.UnmarshalJSON(body); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid event payload"})
			return
		}
	}

	log.Printf("Processing Stripe event: %s (ID: %s)", event.Type, event.ID)

	if err := h.service.ProcessEvent(c.Request.Context(), event); err != nil {
		log.Printf("Error processing webhook event %s: %v", event.ID, err)
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}
