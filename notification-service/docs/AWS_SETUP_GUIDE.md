# AWS SES & SNS Setup Guide for Notification Service

This guide walks you through setting up AWS SES (Simple Email Service) and AWS SNS (Simple Notification Service) in the **ap-south-1 (Mumbai, India)** region.

## Table of Contents
1. [Prerequisites](#prerequisites)
2. [Step 1: Create IAM User](#step-1-create-iam-user)
3. [Step 2: Setup AWS SES](#step-2-setup-aws-ses)
4. [Step 3: Setup AWS SNS](#step-3-setup-aws-sns)
5. [Step 4: Create Sealed Secrets](#step-4-create-sealed-secrets)
6. [Step 5: Deploy to Kubernetes](#step-5-deploy-to-kubernetes)
7. [Testing](#testing)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

- AWS Account with admin access
- AWS CLI installed and configured
- kubectl configured for your cluster
- kubeseal CLI installed (for Sealed Secrets)

```bash
# Install AWS CLI (if not installed)
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install

# Configure AWS CLI
aws configure
# Enter your AWS Access Key ID, Secret Access Key, and set region to ap-south-1

# Install kubeseal (macOS)
brew install kubeseal

# Install kubeseal (Linux)
wget https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/kubeseal-0.24.0-linux-amd64.tar.gz
tar -xvzf kubeseal-0.24.0-linux-amd64.tar.gz
sudo mv kubeseal /usr/local/bin/
```

---

## Step 1: Create IAM User

### 1.1 Create IAM Policy

Create a file `notification-service-policy.json`:

```json
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
```

### 1.2 Create the Policy and User via AWS CLI

```bash
# Create the IAM policy
aws iam create-policy \
    --policy-name NotificationServicePolicy \
    --policy-document file://notification-service-policy.json \
    --description "Policy for Notification Service SES and SNS access"

# Create the IAM user
aws iam create-user --user-name notification-service

# Attach the policy (replace ACCOUNT_ID with your AWS account ID)
aws iam attach-user-policy \
    --user-name notification-service \
    --policy-arn arn:aws:iam::ACCOUNT_ID:policy/NotificationServicePolicy

# Create access keys
aws iam create-access-key --user-name notification-service
```

**Save the AccessKeyId and SecretAccessKey from the output!**

### 1.3 Alternative: Via AWS Console

1. Go to **IAM Console** → **Users** → **Add users**
2. User name: `notification-service`
3. Select **Access key - Programmatic access**
4. Attach the policy created above
5. Download credentials CSV

---

## Step 2: Setup AWS SES

### 2.1 Request Production Access (Important!)

By default, SES is in **Sandbox Mode** which limits:
- Can only send to verified email addresses
- 200 emails per 24 hours
- 1 email per second

**Request production access:**

```bash
# Via AWS Console:
# 1. Go to SES Console → Account dashboard
# 2. Click "Request production access"
# 3. Fill the form explaining your use case
# 4. Wait for approval (usually 24-48 hours)
```

### 2.2 Verify Your Domain (Recommended)

```bash
# Via AWS CLI
aws ses verify-domain-identity \
    --domain yourdomain.com \
    --region ap-south-1
```

**Add DNS Records:**
The command returns a verification token. Add these DNS records:

| Type | Name | Value |
|------|------|-------|
| TXT | _amazonses.yourdomain.com | (verification token) |
| MX | yourdomain.com | 10 feedback-smtp.ap-south-1.amazonses.com |
| TXT | yourdomain.com | v=spf1 include:amazonses.com ~all |

### 2.3 Setup DKIM (Recommended for Deliverability)

```bash
aws ses verify-domain-dkim \
    --domain yourdomain.com \
    --region ap-south-1
```

Add the returned CNAME records to your DNS.

### 2.4 Verify Email Address (For Sandbox Testing)

If still in sandbox mode, verify sender email:

```bash
aws ses verify-email-identity \
    --email-address noreply@yourdomain.com \
    --region ap-south-1
```

Check your inbox and click the verification link.

### 2.5 Verify SES Setup

```bash
# Check verification status
aws ses get-identity-verification-attributes \
    --identities yourdomain.com noreply@yourdomain.com \
    --region ap-south-1

# Check sending quota
aws ses get-send-quota --region ap-south-1

# Send test email
aws ses send-email \
    --from noreply@yourdomain.com \
    --to your-test-email@example.com \
    --subject "Test from SES" \
    --text "This is a test email from AWS SES" \
    --region ap-south-1
```

---

## Step 3: Setup AWS SNS

### 3.1 Configure SMS Settings

```bash
# Set default SMS type to Transactional (higher delivery priority)
aws sns set-sms-attributes \
    --attributes '{
        "DefaultSMSType": "Transactional",
        "DefaultSenderID": "TESSERACT",
        "MonthlySpendLimit": "100"
    }' \
    --region ap-south-1
```

**Note:** Sender ID (like "TESSERACT") is NOT supported in India for transactional SMS. Messages will show a random sender number.

### 3.2 Important: India SMS Regulations

For sending SMS in India, you must comply with TRAI DLT (Distributed Ledger Technology) regulations:

1. **Register as an Enterprise** on a DLT platform (JIO, Airtel, BSNL, etc.)
2. **Register your Entity ID (PEID)**
3. **Register SMS Templates** with the DLT platform
4. **Register Headers/Sender IDs**

AWS SNS requires you to provide the **Entity ID** when sending SMS to Indian numbers:

```bash
# Set SMS attributes with Entity ID
aws sns set-sms-attributes \
    --attributes '{
        "DefaultSMSType": "Transactional"
    }' \
    --region ap-south-1
```

### 3.3 Test SMS Sending

```bash
# Send test SMS (replace with a real phone number)
aws sns publish \
    --phone-number "+919876543210" \
    --message "Test SMS from AWS SNS" \
    --region ap-south-1
```

### 3.4 Check SMS Spending

```bash
aws sns get-sms-attributes \
    --attributes MonthlySpendLimit \
    --region ap-south-1
```

---

## Step 4: Create Sealed Secrets

### 4.1 Install Sealed Secrets Controller (if not installed)

```bash
# Install Sealed Secrets controller
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/controller.yaml

# Wait for controller to be ready
kubectl wait --for=condition=available deployment/sealed-secrets-controller \
    -n kube-system --timeout=60s
```

### 4.2 Create Plain Secret File

Create `aws-secrets-plain.yaml` (DO NOT COMMIT THIS FILE):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: notification-secrets
  namespace: devtest
type: Opaque
stringData:
  # AWS Credentials
  aws-access-key-id: "YOUR_ACCESS_KEY_ID"
  aws-secret-access-key: "YOUR_SECRET_ACCESS_KEY"
  aws-ses-from: "noreply@yourdomain.com"

  # Database
  db-host: "your-db-host"
  db-user: "notification_user"
  db-password: "your-db-password"

  # Fallback providers (optional)
  sendgrid-api-key: "SG.xxxxx"
  twilio-account-sid: "ACxxxxx"
  twilio-auth-token: "your-token"
```

### 4.3 Create Sealed Secret

```bash
# Seal the secret
kubeseal --format yaml \
    --controller-namespace kube-system \
    --controller-name sealed-secrets-controller \
    < aws-secrets-plain.yaml > aws-secrets-sealed.yaml

# Delete the plain text file immediately
rm aws-secrets-plain.yaml
```

### 4.4 Apply Sealed Secret

```bash
kubectl apply -f aws-secrets-sealed.yaml
```

---

## Step 5: Deploy to Kubernetes

### 5.1 Update Deployment

The deployment already references the secrets. Apply the updated manifests:

```bash
cd services/notification-service/k8s

# Apply ConfigMap
kubectl apply -f configmap.yaml -n devtest

# Apply Sealed Secret (created above)
kubectl apply -f aws-secrets-sealed.yaml -n devtest

# Apply Deployment
kubectl apply -f deployment.yaml -n devtest
```

### 5.2 Verify Deployment

```bash
# Check pods
kubectl get pods -n devtest -l app=notification-service

# Check logs
kubectl logs -n devtest -l app=notification-service --tail=100

# Look for successful provider initialization
# Expected output:
# Email provider configured: AWS SES (primary) - region: ap-south-1
# Email provider configured: SendGrid (tertiary - fallback)
# SMS provider configured: AWS SNS (primary) - region: ap-south-1
```

---

## Testing

### Test Email via API

```bash
curl -X POST http://notification-service:8090/api/v1/notifications/send \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: your-tenant-id" \
  -d '{
    "channel": "EMAIL",
    "recipient": "test@example.com",
    "subject": "Test Email",
    "body": "This is a test email from AWS SES",
    "bodyHtml": "<h1>Test</h1><p>This is a test email from AWS SES</p>"
  }'
```

### Test SMS via API

```bash
curl -X POST http://notification-service:8090/api/v1/notifications/send \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: your-tenant-id" \
  -d '{
    "channel": "SMS",
    "recipient": "+919876543210",
    "body": "Test SMS from notification service"
  }'
```

---

## Troubleshooting

### SES Issues

| Issue | Solution |
|-------|----------|
| "Email address not verified" | Verify the sender email in SES console |
| "Daily sending quota exceeded" | Request production access or wait for quota reset |
| "Sending paused" | Check SES reputation dashboard, may be blocked due to bounces |
| "Invalid parameter value" | Check email format, ensure valid addresses |

### SNS Issues

| Issue | Solution |
|-------|----------|
| "Invalid phone number" | Use E.164 format: +919876543210 |
| "Monthly spend limit exceeded" | Increase limit in SNS settings |
| "Destination phone number not verified" | Only in sandbox - verify number or exit sandbox |
| "Entity ID required" | For India, register on DLT platform |

### Check AWS Credentials

```bash
# Verify credentials are set
kubectl exec -it <pod-name> -n devtest -- env | grep AWS

# Test credentials inside pod
kubectl exec -it <pod-name> -n devtest -- aws sts get-caller-identity
```

### View Provider Status

```bash
# Check service health
curl http://notification-service:8090/health

# Check provider chain (from logs)
kubectl logs -n devtest -l app=notification-service | grep "failover chain"
```

---

## Cost Estimation (ap-south-1)

| Service | Cost |
|---------|------|
| SES Email | $0.10 per 1,000 emails |
| SNS SMS (India) | ~₹0.20-0.30 per SMS |
| SNS SMS (International) | Varies by country |

**Monthly estimate for 10,000 emails + 1,000 SMS:**
- SES: ~$1
- SNS SMS: ~₹250 (~$3)
- **Total: ~$4/month**

---

## Security Best Practices

1. **Rotate Access Keys** regularly (every 90 days)
2. **Use IAM Roles** instead of access keys when possible (EKS IRSA)
3. **Enable CloudTrail** for audit logging
4. **Set up billing alerts** to monitor unexpected usage
5. **Never commit secrets** to git - always use Sealed Secrets
6. **Use least privilege** - the IAM policy above only grants necessary permissions
