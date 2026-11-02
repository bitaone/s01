gcloud functions deploy handle-zlist-view \
  --gen2 \
  --runtime=nodejs20 \
  --region=us-central1 \
  --source=. \
  --entry-point=handleView \
  --trigger-http \
  --allow-unauthenticated