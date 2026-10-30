package function

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"google.golang.org/api/iterator"
)

var (
	firestoreClient *firestore.Client
	initOnce        sync.Once
	initErr         error
)

func init() {
	functions.HTTP("ProcessHRequest", ProcessHRequest)
}

// getFirestoreClient initializes a thread-safe singleton Firestore client
func getFirestoreClient(ctx context.Context) (*firestore.Client, error) {
	initOnce.Do(func() {
		projectID := os.Getenv("GCP_PROJECT")
		if projectID == "" {
			projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		firestoreClient, initErr = firestore.NewClient(ctx, projectID)
	})
	return firestoreClient, initErr
}

func ProcessHRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// 1. Extract and sanitize query parameter 'h'
	hParam := html.EscapeString(r.URL.Query().Get("h"))

	matchH := regexp.MustCompile(`\D{1,3}\d{3}`)
	matchT30 := regexp.MustCompile(`t30`)

	// 2. Branch: Match pattern \D{1,3}\d\d\d
	if matchH.MatchString(hParam) {
		client, err := getFirestoreClient(ctx)
		if err != nil {
			http.Error(w, "Database connection error", http.StatusInternalServerError)
			return
		}

		// Fetch document from zlist where document ID or 'h' field matches hParam
		doc, err := client.Collection("zlist").Doc(hParam).Get(ctx)
		if err != nil {
			// Query by field 'h' if document ID is different[cite: 3]
			iter := client.Collection("zlist").Where("h", "==", hParam).Limit(1).Documents(ctx)
			doc, err = iter.Next()
			if err != nil {
				return // Record not found
			}
		}

		data := doc.Data()
		if ipVal, ok := data["i"].(string); ok && ipVal != "" {
			// Transform IP address[cite: 3]
			transformed := transformIPReverse(ipVal)
			fmt.Fprint(w, transformed)
		}
		return
	}

	// 3. Branch: Match pattern 't30'[cite: 3]
	if matchT30.MatchString(hParam) {
		client, err := getFirestoreClient(ctx)
		if err != nil {
			http.Error(w, "Database connection error", http.StatusInternalServerError)
			return
		}

		// TIMESTAMPDIFF(MINUTE, t, NOW()) > 30 translates to: t < (NOW - 30 minutes)[cite: 3]
		thirtyMinsAgo := time.Now().UTC().Add(-30 * time.Minute)
		iter := client.Collection("zlist").Where("t", "<", thirtyMinsAgo).Documents(ctx)

		var resultStr strings.Builder
		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				http.Error(w, "Error querying database", http.StatusInternalServerError)
				return
			}

			hValue := doc.Ref.ID
			if val, ok := doc.Data()["h"].(string); ok && val != "" {
				hValue = val
			}
			// Output comma-separated list of h values[cite: 3]
			resultStr.WriteString(hValue + ",")
		}

		fmt.Fprint(w, resultStr.String())
		return
	}

	// 4. Fallback output[cite: 3]
	fmt.Fprint(w, "0")
}

// Replicates: ia[0].ia[1].(ia[3]).(ia[2]-1)[cite: 3]
func transformIPReverse(ipStr string) string {
	parts := strings.Split(ipStr, ".")
	if len(parts) != 4 {
		return ipStr
	}

	ia2, err := strconv.Atoi(parts[2])
	if err != nil {
		return ipStr
	}

	return fmt.Sprintf("%s.%s.%s.%d", parts[0], parts[1], parts[3], ia2-1)
}