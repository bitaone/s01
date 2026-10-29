#!/usr/bin/env bash
set -e

# --- CONFIGURATION ---
BUDGET_AMOUNT="10.00HKD"                  # 10 HKD Budget
ALERT_PERCENT="0.2"                       # 20% threshold (2 HKD)
NOTIFICATION_EMAIL="your-email@example.com"  # <-- REPLACE WITH YOUR EMAIL ADDRESS

# 1. Retrieve Active GCP Project ID & Billing Account
PROJECT_ID=$(gcloud config get-value project 2>/dev/null)

if [ -z "$PROJECT_ID" ]; then
  echo "Error: No active GCP project found. Set one using: gcloud config set project <PROJECT_ID>"
  exit 1
fi

PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
BILLING_ACCOUNT_ID=$(gcloud beta billing projects describe "$PROJECT_ID" --format='value(billingAccountId)')

if [ -z "$BILLING_ACCOUNT_ID" ]; then
  echo "Error: No billing account linked to project '$PROJECT_ID'."
  exit 1
fi

echo "=========================================="
echo " Setting up Billing Budget & Alerts       "
echo " Project:         $PROJECT_ID             "
echo " Billing Account: $BILLING_ACCOUNT_ID     "
echo " Budget Amount:   $BUDGET_AMOUNT          "
echo " Alert Threshold: 20% (2 HKD)             "
echo "=========================================="

# 2. Enable Billing Budgets and Monitoring APIs
echo "--> Enabling required APIs..."
gcloud services enable \
  billingbudgets.googleapis.com \
  monitoring.googleapis.com

# 3. Create Cloud Monitoring Email Notification Channel
echo "--> Creating Notification Channel for $NOTIFICATION_EMAIL..."
CHANNEL_ID=$(gcloud alpha monitoring channels create \
  --display-name="Budget Email Alert Channel" \
  --type="email" \
  --channel-labels="email_address=$NOTIFICATION_EMAIL" \
  --format="value(name)")

echo "Created Notification Channel ID: $CHANNEL_ID"

# 4. Create 10 HKD Budget with 20% Alerts (Actual & Forecasted Spend)
echo "--> Creating 10 HKD Billing Budget with 20% alerts..."
gcloud billing budgets create \
  --billing-account="$BILLING_ACCOUNT_ID" \
  --display-name="Budget-10HKD-$PROJECT_ID" \
  --budget-amount="$BUDGET_AMOUNT" \
  --threshold-rule=percent="$ALERT_PERCENT",basis=current-spend \
  --threshold-rule=percent="$ALERT_PERCENT",basis=forecasted-spend \
  --filter-projects="projects/$PROJECT_NUMBER" \
  --all-updates-rule-notification-channels="$CHANNEL_ID"

echo "=========================================="
echo "          SETUP COMPLETED!                "
echo "=========================================="
echo "1. 10 HKD budget created for project $PROJECT_ID."
echo "2. Alerts will be emailed to $NOTIFICATION_EMAIL when:"
echo "   - Current spend reaches 20% (2 HKD)."
echo "   - Forecasted spend is projected to reach 20% (2 HKD)."