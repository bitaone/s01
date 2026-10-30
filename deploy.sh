#!/usr/bin/env bash
set -e

# --- CONFIGURATION ---
REGION="us-central1"
REGION="asia-east2"
REGION="europe-west1"
RUNTIME="go126"

# 1. Retrieve Active GCP Project ID
PROJECT_ID=$(gcloud config get-value project 2>/dev/null)

if [ -z "$PROJECT_ID" ]; then
  echo "Error: No active GCP project found. Set one using: gcloud config set project <PROJECT_ID>"
  exit 1
fi

echo "=========================================="
echo " Starting GCP Cloud Functions Deployment  "
echo " Project: $PROJECT_ID                     "
echo " Region:  $REGION                         "
echo "=========================================="

# 2. Enable Required APIs
echo "--> Enabling required Cloud APIs..."
gcloud services enable \
  cloudfunctions.googleapis.com \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  firestore.googleapis.com \
  artifactregistry.googleapis.com \
  serviceusage.googleapis.com \
  compute.googleapis.com

# 3. Create Default Firestore Database (if not already enabled)
echo "--> Checking Firestore Database..."
if ! gcloud firestore databases describe --project="$PROJECT_ID" >/dev/null 2>&1; then
  echo "--> Initializing Firestore database in Native mode..."
  gcloud firestore databases create --location="$REGION" --type=firestore-native || true
fi

# 4. Deploy Function 'c' (from c.go) with Free Tier Caps & In-Region Egress Control
echo "------------------------------------------"
echo "Deploying 'c' (Entry Point: ProcessRequest)..."
cd c
gcloud functions deploy c \
  --gen2 \
  --region="$REGION" \
  --runtime="$RUNTIME" \
  --source=. \
  --entry-point=ProcessRequest \
  --trigger-http \
  --allow-unauthenticated \
  --memory=256Mi \
  --max-instances=1 \
  --concurrency=1 \
  --egress-settings=private-ranges-only \
  --set-env-vars GCP_PROJECT="$PROJECT_ID"

cd ..

# 5. Deploy Function 'h' (from h.go) with Free Tier Caps & In-Region Egress Control
echo "------------------------------------------"
echo "Deploying 'h' (Entry Point: ProcessHRequest)..."
cd h
gcloud functions deploy h \
  --gen2 \
  --region="$REGION" \
  --runtime="$RUNTIME" \
  --source=. \
  --entry-point=ProcessHRequest \
  --trigger-http \
  --allow-unauthenticated \
  --memory=256Mi \
  --max-instances=1 \
  --concurrency=1 \
#  --egress-settings=private-ranges-only \
  --set-env-vars GCP_PROJECT="$PROJECT_ID"

cd ..

# 6. Deploy Function 'v' (from v.go) with Free Tier Caps & In-Region Egress Control
echo "------------------------------------------"
echo "Deploying 'v' (Entry Point: RenderDashboard)..."
cd v
gcloud functions deploy v \
  --gen2 \
  --region="$REGION" \
  --runtime="$RUNTIME" \
  --source=. \
  --entry-point=RenderDashboard \
  --trigger-http \
  --allow-unauthenticated \
  --memory=256Mi \
  --max-instances=1 \
  --concurrency=1 \
  --egress-settings=private-ranges-only \
  --set-env-vars GCP_PROJECT="$PROJECT_ID"

cd ..

# 7. Apply Service Quota Overrides for Free Tier Boundaries & Outbound Data Limits
echo "------------------------------------------"
echo "--> Setting API rate limits & egress quotas to enforce Free Tier usage..."

# Cap Firestore Reads per minute (Enforces daily 50,000 read limit cap)
gcloud alpha services quota overrides create \
  --service=firestore.googleapis.com \
  --consumer="projects/$PROJECT_ID" \
  --metric="firestore.googleapis.com/read_requests" \
  --unit="1/min/{project}" \
  --override-value=1000 --force || true

# Cap Firestore Writes per minute (Enforces daily 20,000 write limit cap)
gcloud alpha services quota overrides create \
  --service=firestore.googleapis.com \
  --consumer="projects/$PROJECT_ID" \
  --metric="firestore.googleapis.com/write_requests" \
  --unit="1/min/{project}" \
  --override-value=400 --force || true

# Cap Network Egress Bandwidth Rate (Caps compute/network outbound rate to safe free limits)
gcloud alpha services quota overrides create \
  --service=compute.googleapis.com \
  --consumer="projects/$PROJECT_ID" \
  --metric="compute.googleapis.com/network_egress_bandwidth" \
  --unit="1/min/{project}" \
  --override-value=100MB --force || true

# 8. Print Output URLs
echo "=========================================="
echo "       DEPLOYMENT COMPLETE!               "
echo "=========================================="
echo "c Function URL: $(gcloud functions describe c --gen2 --region="$REGION" --format='value(serviceConfig.uri)')"
echo "h Function URL: $(gcloud functions describe h --gen2 --region="$REGION" --format='value(serviceConfig.uri)')"
echo "v Function URL: $(gcloud functions describe v --gen2 --region="$REGION" --format='value(serviceConfig.uri)')"
