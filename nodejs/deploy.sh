#!/bin/bash
set -e

# ==============================================================================
# CONFIGURATION VARIABLES
# ==============================================================================
# Set your desired region and Node.js runtime version here
REGION="us-central1"
REGION="asia-east2"
REGION="europe-west1"
NODE_RUNTIME="nodejs24"

# Auto-detect active GCP Project ID from environment context
PROJECT_ID=$(gcloud config get-value project)

if [ -z "$PROJECT_ID" ]; then
  echo "Error: No active GCP project set. Run 'gcloud config set project YOUR_PROJECT_ID' first."
  exit 1
fi

echo "======================================================================"
echo "Starting deployment for Project: ${PROJECT_ID}"
echo "Region: ${REGION} | Runtime: ${NODE_RUNTIME}"
echo "======================================================================"

# ==============================================================================
# 1. ENABLE REQUIRED GCP APIS
# ==============================================================================
echo "[1/4] Enabling required Google Cloud APIs..."
gcloud services enable \
  firestore.googleapis.com \
  cloudfunctions.googleapis.com \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  artifactregistry.googleapis.com \
  --project="${PROJECT_ID}"

# ==============================================================================
# 2. INITIALIZE FIRESTORE DATABASE
# ==============================================================================
echo "[2/4] Initializing Firestore database (Native Mode)..."
if ! gcloud firestore databases describe --project="${PROJECT_ID}" >/dev/null 2>&1; then
  gcloud firestore databases create \
    --location="${REGION}" \
    --type="firestore-native" \
    --project="${PROJECT_ID}"
  echo "Firestore database created successfully."
else
  echo "Firestore database already exists."
fi

# ==============================================================================
# 3. DEPLOY CLOUD RUN FUNCTIONS
# ==============================================================================

# Deploy Function 1 (c)
echo "[3/4] Deploying Function 1 (c)..."
gcloud functions deploy handle-c \
  --gen2 \
  --runtime="${NODE_RUNTIME}" \
  --region="${REGION}" \
  --source=./c \
  --entry-point=handleRequest \
  --trigger-http \
  --allow-unauthenticated \
  --project="${PROJECT_ID}"

# Deploy Function 2 (v)
echo "[4/4] Deploying Function 2 (v)..."
gcloud functions deploy handle-v \
  --gen2 \
  --runtime="${NODE_RUNTIME}" \
  --region="${REGION}" \
  --source=./v \
  --entry-point=handleView \
  --trigger-http \
  --allow-unauthenticated \
  --project="${PROJECT_ID}"

# Deploy Function 3 (h)
echo "[5/5] Deploying Function 3 (h)..."
gcloud functions deploy handle-h \
  --gen2 \
  --runtime="${NODE_RUNTIME}" \
  --region="${REGION}" \
  --source=./h \
  --entry-point=handleH \
  --trigger-http \
  --allow-unauthenticated \
  --project="${PROJECT_ID}"

echo "======================================================================"
echo "Deployment Complete! Function URLs:"
echo "======================================================================"
gcloud functions describe handle-c --gen2 --region="${REGION}" --format="value(serviceConfig.uri)"
gcloud functions describe handle-v --gen2 --region="${REGION}" --format="value(serviceConfig.uri)"
gcloud functions describe handle-h --gen2 --region="${REGION}" --format="value(serviceConfig.uri)"
