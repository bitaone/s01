gcloud functions deploy handle-h \
  --gen2 \
  --runtime=nodejs20 \
  --region=us-central1 \
  --source=. \
  --entry-point=handleH \
  --trigger-http \
  --allow-unauthenticated