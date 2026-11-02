gcloud functions deploy handle-zlist \
  --gen2 \
  --runtime=nodejs20 \
  --region=us-central1 \
  --source=. \
  --entry-point=handleRequest \
  --trigger-http \
  --allow-unauthenticated