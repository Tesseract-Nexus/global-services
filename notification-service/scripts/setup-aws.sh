#!/bin/bash
# =============================================================================
# AWS SES & SNS Setup Script for Notification Service
# Region: ap-south-1 (Mumbai, India)
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
AWS_REGION="ap-south-1"
IAM_USER_NAME="notification-service"
POLICY_NAME="NotificationServicePolicy"

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}  AWS SES & SNS Setup for Notification Service  ${NC}"
echo -e "${BLUE}  Region: ${AWS_REGION} (Mumbai, India)         ${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# Check prerequisites
check_prerequisites() {
    echo -e "${YELLOW}Checking prerequisites...${NC}"

    if ! command -v aws &> /dev/null; then
        echo -e "${RED}Error: AWS CLI is not installed${NC}"
        echo "Install with: brew install awscli (macOS) or apt install awscli (Linux)"
        exit 1
    fi

    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Error: jq is not installed${NC}"
        echo "Install with: brew install jq (macOS) or apt install jq (Linux)"
        exit 1
    fi

    # Check AWS credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        echo -e "${RED}Error: AWS credentials not configured${NC}"
        echo "Run: aws configure"
        exit 1
    fi

    echo -e "${GREEN}✓ Prerequisites met${NC}"
    echo ""
}

# Get AWS account ID
get_account_id() {
    AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
    echo -e "${GREEN}✓ AWS Account ID: ${AWS_ACCOUNT_ID}${NC}"
}

# Create IAM policy
create_iam_policy() {
    echo -e "${YELLOW}Creating IAM policy...${NC}"

    # Check if policy exists
    if aws iam get-policy --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/${POLICY_NAME}" &> /dev/null; then
        echo -e "${GREEN}✓ Policy ${POLICY_NAME} already exists${NC}"
        return
    fi

    cat > /tmp/notification-policy.json << 'EOF'
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "SESPermissions",
            "Effect": "Allow",
            "Action": [
                "ses:SendEmail",
                "ses:SendRawEmail",
                "ses:GetSendQuota",
                "ses:GetSendStatistics"
            ],
            "Resource": "*"
        },
        {
            "Sid": "SNSPermissions",
            "Effect": "Allow",
            "Action": [
                "sns:Publish",
                "sns:GetSMSAttributes",
                "sns:SetSMSAttributes",
                "sns:CheckIfPhoneNumberIsOptedOut"
            ],
            "Resource": "*"
        }
    ]
}
EOF

    aws iam create-policy \
        --policy-name "${POLICY_NAME}" \
        --policy-document file:///tmp/notification-policy.json \
        --description "Policy for Notification Service SES and SNS access" \
        --output text > /dev/null

    rm /tmp/notification-policy.json
    echo -e "${GREEN}✓ Created IAM policy: ${POLICY_NAME}${NC}"
}

# Create IAM user and access keys
create_iam_user() {
    echo -e "${YELLOW}Creating IAM user...${NC}"

    # Check if user exists
    if aws iam get-user --user-name "${IAM_USER_NAME}" &> /dev/null; then
        echo -e "${YELLOW}⚠ User ${IAM_USER_NAME} already exists${NC}"

        read -p "Do you want to create new access keys? (y/n): " create_keys
        if [[ "$create_keys" != "y" ]]; then
            return
        fi
    else
        aws iam create-user --user-name "${IAM_USER_NAME}" --output text > /dev/null
        echo -e "${GREEN}✓ Created IAM user: ${IAM_USER_NAME}${NC}"

        # Attach policy
        aws iam attach-user-policy \
            --user-name "${IAM_USER_NAME}" \
            --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/${POLICY_NAME}"
        echo -e "${GREEN}✓ Attached policy to user${NC}"
    fi

    # Create access keys
    echo -e "${YELLOW}Creating access keys...${NC}"
    ACCESS_KEY_OUTPUT=$(aws iam create-access-key --user-name "${IAM_USER_NAME}" --output json)

    AWS_ACCESS_KEY_ID=$(echo "$ACCESS_KEY_OUTPUT" | jq -r '.AccessKey.AccessKeyId')
    AWS_SECRET_ACCESS_KEY=$(echo "$ACCESS_KEY_OUTPUT" | jq -r '.AccessKey.SecretAccessKey')

    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}  ACCESS KEYS CREATED - SAVE THESE SECURELY!   ${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo -e "AWS_ACCESS_KEY_ID:     ${YELLOW}${AWS_ACCESS_KEY_ID}${NC}"
    echo -e "AWS_SECRET_ACCESS_KEY: ${YELLOW}${AWS_SECRET_ACCESS_KEY}${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    echo -e "${RED}WARNING: The secret access key will not be shown again!${NC}"
    echo ""
}

# Setup SES
setup_ses() {
    echo -e "${YELLOW}Setting up AWS SES...${NC}"

    read -p "Enter the domain to verify for SES (e.g., yourdomain.com): " SES_DOMAIN
    read -p "Enter the sender email address (e.g., noreply@yourdomain.com): " SES_EMAIL

    # Verify domain
    echo -e "${YELLOW}Verifying domain: ${SES_DOMAIN}${NC}"
    VERIFICATION_TOKEN=$(aws ses verify-domain-identity \
        --domain "${SES_DOMAIN}" \
        --region "${AWS_REGION}" \
        --query 'VerificationToken' \
        --output text)

    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}  ADD THESE DNS RECORDS TO YOUR DOMAIN         ${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    echo -e "${YELLOW}1. Domain Verification TXT Record:${NC}"
    echo "   Type:  TXT"
    echo "   Name:  _amazonses.${SES_DOMAIN}"
    echo "   Value: ${VERIFICATION_TOKEN}"
    echo ""

    # Get DKIM tokens
    echo -e "${YELLOW}Setting up DKIM...${NC}"
    DKIM_TOKENS=$(aws ses verify-domain-dkim \
        --domain "${SES_DOMAIN}" \
        --region "${AWS_REGION}" \
        --query 'DkimTokens' \
        --output json)

    echo -e "${YELLOW}2. DKIM CNAME Records:${NC}"
    for token in $(echo "$DKIM_TOKENS" | jq -r '.[]'); do
        echo "   Type:  CNAME"
        echo "   Name:  ${token}._domainkey.${SES_DOMAIN}"
        echo "   Value: ${token}.dkim.amazonses.com"
        echo ""
    done

    echo -e "${YELLOW}3. SPF Record (update existing or add new):${NC}"
    echo "   Type:  TXT"
    echo "   Name:  ${SES_DOMAIN}"
    echo "   Value: v=spf1 include:amazonses.com ~all"
    echo ""

    echo -e "${YELLOW}4. MX Record for bounce handling:${NC}"
    echo "   Type:  MX"
    echo "   Name:  ${SES_DOMAIN}"
    echo "   Value: 10 feedback-smtp.${AWS_REGION}.amazonses.com"
    echo ""
    echo -e "${GREEN}================================================${NC}"

    # Verify email
    echo -e "${YELLOW}Verifying sender email: ${SES_EMAIL}${NC}"
    aws ses verify-email-identity \
        --email-address "${SES_EMAIL}" \
        --region "${AWS_REGION}"

    echo -e "${GREEN}✓ Verification email sent to ${SES_EMAIL}${NC}"
    echo -e "${YELLOW}Please check your inbox and click the verification link${NC}"

    # Save for later
    echo "${SES_EMAIL}" > /tmp/ses_email.txt
}

# Setup SNS SMS
setup_sns() {
    echo -e "${YELLOW}Setting up AWS SNS for SMS...${NC}"

    # Set default SMS type to Transactional
    aws sns set-sms-attributes \
        --attributes '{"DefaultSMSType":"Transactional"}' \
        --region "${AWS_REGION}"

    echo -e "${GREEN}✓ SNS SMS configured with Transactional type${NC}"

    echo ""
    echo -e "${YELLOW}================================================${NC}"
    echo -e "${YELLOW}  IMPORTANT: India SMS Regulations (DLT)        ${NC}"
    echo -e "${YELLOW}================================================${NC}"
    echo ""
    echo "To send SMS in India, you must comply with TRAI DLT regulations:"
    echo ""
    echo "1. Register on a DLT platform (JIO, Airtel, BSNL, Vodafone)"
    echo "2. Get your Entity ID (PEID)"
    echo "3. Register your SMS templates"
    echo "4. Register sender headers"
    echo ""
    echo "DLT Registration Links:"
    echo "  - JIO: https://trueconnect.jio.com"
    echo "  - Airtel: https://www.airtel.in/business/commercial-communication"
    echo "  - BSNL: https://www.ucc-bsnl.co.in"
    echo "  - Vodafone: https://www.vilpower.in"
    echo ""
    echo -e "${YELLOW}================================================${NC}"
}

# Generate sealed secret template
generate_sealed_secret() {
    echo -e "${YELLOW}Generating secret template...${NC}"

    read -p "Enter Kubernetes namespace (default: devtest): " K8S_NAMESPACE
    K8S_NAMESPACE=${K8S_NAMESPACE:-devtest}

    SES_EMAIL=$(cat /tmp/ses_email.txt 2>/dev/null || echo "noreply@yourdomain.com")

    cat > notification-secrets-plain.yaml << EOF
apiVersion: v1
kind: Secret
metadata:
  name: notification-secrets
  namespace: ${K8S_NAMESPACE}
type: Opaque
stringData:
  # AWS Credentials
  aws-access-key-id: "${AWS_ACCESS_KEY_ID:-YOUR_ACCESS_KEY_ID}"
  aws-secret-access-key: "${AWS_SECRET_ACCESS_KEY:-YOUR_SECRET_ACCESS_KEY}"
  aws-ses-from: "${SES_EMAIL}"

  # Database (update these)
  db-host: "your-db-host"
  db-user: "notification_user"
  db-password: "your-db-password"

  # SendGrid fallback (optional)
  sendgrid-api-key: ""

  # Twilio fallback (optional)
  twilio-account-sid: ""
  twilio-auth-token: ""
EOF

    echo -e "${GREEN}✓ Created notification-secrets-plain.yaml${NC}"
    echo ""

    # Check if kubeseal is available
    if command -v kubeseal &> /dev/null; then
        echo -e "${YELLOW}Creating sealed secret...${NC}"

        if kubeseal --format yaml \
            --controller-namespace kube-system \
            --controller-name sealed-secrets-controller \
            < notification-secrets-plain.yaml > notification-secrets-sealed.yaml 2>/dev/null; then

            echo -e "${GREEN}✓ Created notification-secrets-sealed.yaml${NC}"
            echo ""
            echo -e "${RED}IMPORTANT: Delete the plain secret file:${NC}"
            echo "  rm notification-secrets-plain.yaml"
            echo ""
            echo "Apply the sealed secret:"
            echo "  kubectl apply -f notification-secrets-sealed.yaml"
        else
            echo -e "${YELLOW}⚠ Could not create sealed secret (controller may not be installed)${NC}"
            echo ""
            echo "To seal the secret manually:"
            echo "  kubeseal --format yaml < notification-secrets-plain.yaml > notification-secrets-sealed.yaml"
        fi
    else
        echo -e "${YELLOW}⚠ kubeseal not installed${NC}"
        echo ""
        echo "Install kubeseal and seal the secret:"
        echo "  brew install kubeseal  # macOS"
        echo "  kubeseal --format yaml < notification-secrets-plain.yaml > notification-secrets-sealed.yaml"
    fi

    rm -f /tmp/ses_email.txt
}

# Print summary
print_summary() {
    echo ""
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}                    SUMMARY                     ${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo ""
    echo -e "${GREEN}✓ IAM user created: ${IAM_USER_NAME}${NC}"
    echo -e "${GREEN}✓ IAM policy attached: ${POLICY_NAME}${NC}"
    echo -e "${GREEN}✓ SES domain verification initiated${NC}"
    echo -e "${GREEN}✓ SNS SMS configured${NC}"
    echo -e "${GREEN}✓ Secret template generated${NC}"
    echo ""
    echo -e "${YELLOW}NEXT STEPS:${NC}"
    echo "1. Add the DNS records shown above to your domain"
    echo "2. Click the verification link in your email"
    echo "3. Wait for domain verification (check with: aws ses get-identity-verification-attributes)"
    echo "4. Request SES production access (AWS Console → SES → Account dashboard)"
    echo "5. Register on DLT platform for India SMS (if needed)"
    echo "6. Seal and apply the Kubernetes secret"
    echo "7. Deploy the notification service"
    echo ""
    echo -e "${BLUE}================================================${NC}"
}

# Main
main() {
    check_prerequisites
    get_account_id

    echo "This script will:"
    echo "  1. Create IAM policy and user for SES/SNS access"
    echo "  2. Configure SES domain and email verification"
    echo "  3. Configure SNS SMS settings"
    echo "  4. Generate Kubernetes secret template"
    echo ""
    read -p "Continue? (y/n): " confirm
    if [[ "$confirm" != "y" ]]; then
        echo "Aborted."
        exit 0
    fi
    echo ""

    create_iam_policy
    create_iam_user
    setup_ses
    setup_sns
    generate_sealed_secret
    print_summary
}

# Run
main "$@"
