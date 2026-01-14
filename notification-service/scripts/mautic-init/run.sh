#!/bin/bash
# Mautic Campaign Initialization Script
# This script initializes benchmark campaigns, segments, and email templates in Mautic
# It also fixes the Postfix sender rewriting issue for AWS SES relay

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}"
echo "╔════════════════════════════════════════════════════════════╗"
echo "║     Mautic & Email Infrastructure Initialization           ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Function to get secret from Kubernetes
get_k8s_secret() {
    local secret_name=$1
    local key=$2
    local namespace=${3:-email}
    kubectl get secret "$secret_name" -n "$namespace" -o jsonpath="{.data.$key}" 2>/dev/null | base64 -d
}

# Function to fix Postfix sender rewriting for AWS SES
fix_postfix_sender() {
    echo -e "${YELLOW}Checking Postfix sender configuration...${NC}"

    if ! command -v kubectl &> /dev/null; then
        echo -e "${YELLOW}kubectl not available, skipping Postfix fix${NC}"
        return
    fi

    # Check if the ConfigMap already exists
    if kubectl get configmap postfix-generic-maps -n email &>/dev/null; then
        echo -e "${GREEN}  ✓ Postfix generic maps ConfigMap exists${NC}"
    else
        echo -e "${YELLOW}  Creating Postfix generic maps ConfigMap...${NC}"
        kubectl apply -f "$SCRIPT_DIR/postfix-fix.yaml" 2>/dev/null || true
    fi

    # Check if the deployment has the required env vars
    CURRENT_MYORIGIN=$(kubectl get deployment postal-worker -n email -o jsonpath='{.spec.template.spec.containers[?(@.name=="postfix-relay")].env[?(@.name=="POSTFIX_myorigin")].value}' 2>/dev/null || echo "")

    if [ "$CURRENT_MYORIGIN" != "tesserix.app" ]; then
        echo -e "${YELLOW}  Patching postal-worker deployment for sender fix...${NC}"

        # Add POSTFIX_myorigin
        kubectl patch deployment postal-worker -n email --type='json' -p='[
          {"op": "add", "path": "/spec/template/spec/containers/1/env/-", "value": {"name": "POSTFIX_myorigin", "value": "tesserix.app"}},
          {"op": "add", "path": "/spec/template/spec/containers/1/env/-", "value": {"name": "POSTFIX_smtp_generic_maps", "value": "regexp:/etc/postfix/generic"}}
        ]' 2>/dev/null || true

        # Add volume and mount
        kubectl patch deployment postal-worker -n email --type='json' -p='[
          {"op": "add", "path": "/spec/template/spec/volumes/-", "value": {"name": "postfix-generic", "configMap": {"name": "postfix-generic-maps"}}},
          {"op": "add", "path": "/spec/template/spec/containers/1/volumeMounts", "value": [{"name": "postfix-generic", "mountPath": "/etc/postfix/generic", "subPath": "generic"}]}
        ]' 2>/dev/null || true

        echo -e "${GREEN}  ✓ Postfix sender fix applied${NC}"
        echo -e "${YELLOW}  Waiting for postal-worker to restart...${NC}"
        kubectl rollout status deployment/postal-worker -n email --timeout=120s 2>/dev/null || true
    else
        echo -e "${GREEN}  ✓ Postfix sender configuration is correct${NC}"
    fi
}

# Check if running in-cluster or locally
if [ -n "$KUBERNETES_SERVICE_HOST" ]; then
    echo -e "${YELLOW}Running in Kubernetes cluster...${NC}"
    IN_CLUSTER=true
else
    echo -e "${YELLOW}Running locally...${NC}"
    IN_CLUSTER=false
fi

# Set Mautic URL
if [ -z "$MAUTIC_URL" ]; then
    if [ "$IN_CLUSTER" = true ]; then
        export MAUTIC_URL="http://mautic.email.svc.cluster.local"
    else
        export MAUTIC_URL="https://dev-mautic.tesserix.app"
    fi
fi
echo -e "${GREEN}Mautic URL: $MAUTIC_URL${NC}"

# Set Mautic credentials
if [ -z "$MAUTIC_USERNAME" ]; then
    export MAUTIC_USERNAME="admin"
fi

if [ -z "$MAUTIC_PASSWORD" ]; then
    if command -v kubectl &> /dev/null; then
        echo -e "${YELLOW}Fetching Mautic password from Kubernetes secret...${NC}"
        export MAUTIC_PASSWORD=$(get_k8s_secret "mautic-credentials" "admin-password")
    fi
fi

if [ -z "$MAUTIC_PASSWORD" ]; then
    echo -e "${RED}ERROR: MAUTIC_PASSWORD is required${NC}"
    echo "Set it via environment variable or ensure mautic-credentials secret exists in email namespace"
    exit 1
fi

# Set email settings
export FROM_EMAIL="${FROM_EMAIL:-noreply@mail.tesserix.app}"
export FROM_NAME="${FROM_NAME:-Tesseract Hub}"

# Optional: Test email
if [ -n "$1" ]; then
    export TEST_EMAIL="$1"
    echo -e "${GREEN}Test email will be sent to: $TEST_EMAIL${NC}"
fi

echo ""

# Fix Postfix sender rewriting for AWS SES relay (if kubectl available)
fix_postfix_sender

echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}ERROR: Go is not installed${NC}"
    echo "Please install Go 1.21+ to run this script"
    exit 1
fi

# Run the initialization
cd "$SCRIPT_DIR"
echo -e "${BLUE}Starting Mautic initialization...${NC}"
echo ""

go run main.go

echo ""
echo -e "${GREEN}Done!${NC}"
