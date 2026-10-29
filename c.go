package function

import (
	"context"
	"fmt"
	"html"
	"net"
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
	initClientOnce  sync.Once
	initErr         error
)

func init() {
	// Register the HTTP function with Functions Framework
	functions.HTTP("ProcessRequest", ProcessRequest)
}

// getFirestoreClient initializes a singleton Firestore client using default GCP credentials.
func getFirestoreClient(ctx context.Context) (*firestore.Client, error) {
	initClientOnce.Do(func() {
		projectID := os.Getenv("GCP_PROJECT")
		if projectID == "" {
			projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		firestoreClient, initErr = firestore.NewClient(ctx, projectID)
	})
	return firestoreClient, initErr
}

// ProcessRequest handles incoming HTTP requests and executes Firestore operations.
func ProcessRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Extract and sanitize query parameters
	query := r.URL.Query()
	hParam := html.EscapeString(query.Get("h"))

	cParam := ""
	if query.Has("c") {
		cParam = html.EscapeString(query.Get("c"))
	}

	rParam := "-"
	if query.Has("r") {
		rParam = html.EscapeString(query.Get("r"))
	}

	// 2. Extract client IP address (supporting GCP load balancers via X-Forwarded-For)
	clientIP := getClientIP(r)

	// Transform IP: ia[0].ia[1].(ia[3]+1).ia[2]
	transformedIP := transformIP(clientIP)

	// 3. Regex match on parameter 'h'
	matchH := regexp.MustCompile(`\D{1,3}\d{3}`)
	if !matchH.MatchString(hParam) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "0")
		return
	}

	// Echo transformed IP address to response
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, transformedIP)

	// Initialize Firestore client
	client, err := getFirestoreClient(ctx)
	if err != nil {
		http.Error(w, "Database initialization failed", http.StatusInternalServerError)
		return
	}

	// 4. Branching database logic based on 'r' and 'c' parameters
	matchR := regexp.MustCompile(`r`)
	matchDigitR := regexp.MustCompile(`\d+`)
	matchC := regexp.MustCompile(`c`)
	matchZeroC := regexp.MustCompile(`0`)
	matchDashR := regexp.MustCompile(`-`)

	if matchR.MatchString(rParam) {
		// UPDATE zlist SET r=0;
		_ = updateAllDocuments(ctx, client, "zlist", map[string]interface{}{
			"r": 0,
		})

	} else if matchDigitR.MatchString(rParam) {
		// UPDATE zlist SET r="$r" WHERE h="$h";
		docRef := client.Collection("zlist").Doc(hParam)
		_, _ = docRef.Set(ctx, map[string]interface{}{
			"r": rParam,
		}, firestore.MergeAll)

	} else if matchC.MatchString(cParam) {
		// INSERT INTO zlist (h,i,c,t) VALUES ("$h","$i",0,CURRENT_TIMESTAMP)
		// ON DUPLICATE KEY UPDATE i="$i", t=CURRENT_TIMESTAMP;
		docRef := client.Collection("zlist").Doc(hParam)
		_ = client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			doc, err := tx.Get(docRef)
			now := time.Now().UTC()

			if err != nil && !doc.Exists() {
				// Insert new record with c = 0
				return tx.Set(docRef, map[string]interface{}{
					"h": hParam,
					"i": transformedIP,
					"c": 0,
					"t": now,
				})
			}
			// Update existing record without overwriting c
			return tx.Set(docRef, map[string]interface{}{
				"i": transformedIP,
				"t": now,
			}, firestore.MergeAll)
		})

	} else if matchZeroC.MatchString(cParam) {
		// UPDATE zlist SET c=0 WHERE h LIKE '%<last 3 chars>';
		var suffix string
		if len(hParam) >= 3 {
			suffix = hParam[len(hParam)-3:]
		} else {
			suffix = hParam
		}
		_ = updateDocumentsEndingWith(ctx, client, "zlist", "h", suffix, map[string]interface{}{
			"c": 0,
		})

		// DELETE FROM zlist WHERE t < DATE_SUB(NOW(), INTERVAL 1 DAY);
		oneDayAgo := time.Now().UTC().AddDate(0, 0, -1)
		_ = deleteDocumentsBefore(ctx, client, "zlist", "t", oneDayAgo)

		// DELETE FROM zda WHERE ct < DATE_SUB(NOW(), INTERVAL 15 DAY);
		fifteenDaysAgo := time.Now().UTC().AddDate(0, 0, -15)
		_ = deleteDocumentsBefore(ctx, client, "zda", "ct", fifteenDaysAgo)

	} else if matchDashR.MatchString(rParam) {
		// 1 & 2. UPDATE zlist SET d=0 WHERE h="$h" AND i<>"$i" / d=d+1 WHERE h="$h" AND i="$i"
		docRef := client.Collection("zlist").Doc(hParam)
		_ = client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			doc, err := tx.Get(docRef)
			now := time.Now().UTC()

			if err == nil && doc.Exists() {
				data := doc.Data()
				existingI, _ := data["i"].(string)

				var updates []firestore.Update
				if existingI == transformedIP {
					updates = append(updates, firestore.Update{Path: "d", Value: firestore.Increment(1)})
				} else {
					updates = append(updates, firestore.Update{Path: "d", Value: 0})
				}
				// ON DUPLICATE KEY UPDATE c=c+1, i="$i", t=CURRENT_TIMESTAMP
				updates = append(updates,
					firestore.Update{Path: "c", Value: firestore.Increment(1)},
					firestore.Update{Path: "i", Value: transformedIP},
					firestore.Update{Path: "t", Value: now},
				)
				return tx.Update(docRef, updates)
			}

			// Insert new record if doc doesn't exist
			return tx.Set(docRef, map[string]interface{}{
				"h": hParam,
				"i": transformedIP,
				"c": 1,
				"d": 0,
				"t": now,
			})
		})
	}
}

// Helper to extract client IP address
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// Replicates IP transformation: ia[0].ia[1].(ia[3]+1).ia[2]
func transformIP(ipStr string) string {
	parts := strings.Split(ipStr, ".")
	if len(parts) != 4 {
		return ipStr
	}
	p3, err := strconv.Atoi(parts[3])
	if err != nil {
		return ipStr
	}
	return fmt.Sprintf("%s.%s.%d.%s", parts[0], parts[1], p3+1, parts[2])
}

// Batch update for all documents in a collection
func updateAllDocuments(ctx context.Context, client *firestore.Client, collection string, updates map[string]interface{}) error {
	iter := client.Collection(collection).Documents(ctx)
	batch := client.Batch()
	count := 0

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		batch.Set(doc.Ref, updates, firestore.MergeAll)
		count++
		if count == 500 { // Firestore batch limit is 500 ops
			if _, err := batch.Commit(ctx); err != nil {
				return err
			}
			batch = client.Batch()
			count = 0
		}
	}
	if count > 0 {
		_, err := batch.Commit(ctx)
		return err
	}
	return nil
}

// Batch update for documents matching suffix on field
func updateDocumentsEndingWith(ctx context.Context, client *firestore.Client, collection, field, suffix string, updates map[string]interface{}) error {
	iter := client.Collection(collection).Documents(ctx)
	batch := client.Batch()
	count := 0

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		if val, ok := doc.Data()[field].(string); ok && strings.HasSuffix(val, suffix) {
			batch.Set(doc.Ref, updates, firestore.MergeAll)
			count++
			if count == 500 {
				if _, err := batch.Commit(ctx); err != nil {
					return err
				}
				batch = client.Batch()
				count = 0
			}
		}
	}
	if count > 0 {
		_, err := batch.Commit(ctx)
		return err
	}
	return nil
}

// Batch delete for documents with timestamp before threshold
func deleteDocumentsBefore(ctx context.Context, client *firestore.Client, collection, timeField string, threshold time.Time) error {
	iter := client.Collection(collection).Where(timeField, "<", threshold).Documents(ctx)
	batch := client.Batch()
	count := 0

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		batch.Delete(doc.Ref)
		count++
		if count == 500 {
			if _, err := batch.Commit(ctx); err != nil {
				return err
			}
			batch = client.Batch()
			count = 0
		}
	}
	if count > 0 {
		_, err := batch.Commit(ctx)
		return err
	}
	return nil
}