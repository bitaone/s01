package function

import (
	"context"
	"html"
	"html/template"
	"net/http"
	"os"
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
	functions.HTTP("RenderDashboard", RenderDashboard)
}

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

type ZListItem struct {
	H  string
	I  string
	R  interface{}
	D  int64
	C  int64
	TT int64 // Minutes elapsed since timestamp t
}

func RenderDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fsClient, err := getFirestoreClient(ctx)
	if err != nil {
		http.Error(w, "Failed to initialize Firestore: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = r.ParseForm()
	zParam := html.EscapeString(r.FormValue("z"))[cite: 4]
	iParam := html.EscapeString(r.FormValue("i"))[cite: 4]
	codeParam := html.EscapeString(r.FormValue("code"))[cite: 4]

	// 1. Handle Reset Action ($z contains 'z')
	if strings.Contains(zParam, "z") {[cite: 4]
		// Truncate zimg collection
		_ = deleteAllDocuments(ctx, fsClient, "zimg")

		// UPDATE zlist SET c=0[cite: 4]
		_ = updateAllDocuments(ctx, fsClient, "zlist", map[string]interface{}{
			"c": 0,
		})
	}

	// 2. Upsert code into zimg if both i and code are provided[cite: 4]
	if len(codeParam) > 0 && len(iParam) > 0 {[cite: 4]
		_ = fsClient.Collection("zimg").Doc(iParam).Set(ctx, map[string]interface{}{
			"i":    iParam,
			"code": codeParam,
		}, firestore.MergeAll)[cite: 4]
	}

	// 3. Fetch zlist items and calculate elapsed time tt in minutes[cite: 4]
	zlistItems, err := getZListItems(ctx, fsClient)
	if err != nil {
		http.Error(w, "Error fetching zlist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Render HTML Output matching table formatting[cite: 4]
	renderHTML(w, zlistItems)
}

// Fetch all documents from zlist and compute minutes elapsed (tt)[cite: 4]
func getZListItems(ctx context.Context, client *firestore.Client) ([]ZListItem, error) {
	var items []ZListItem
	iter := client.Collection("zlist").Documents(ctx)

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		data := doc.Data()
		item := ZListItem{
			H: doc.Ref.ID,
		}

		if val, ok := data["i"].(string); ok {
			item.I = val
		}
		item.R = data["r"]

		if val, ok := data["d"].(int64); ok {
			item.D = val
		} else if val, ok := data["d"].(float64); ok {
			item.D = int64(val)
		}

		if val, ok := data["c"].(int64); ok {
			item.C = val
		} else if val, ok := data["c"].(float64); ok {
			item.C = int64(val)
		}

		// Calculate TIMESTAMPDIFF(MINUTE, t, NOW())[cite: 4]
		if tVal, ok := data["t"].(time.Time); ok {
			item.TT = int64(time.Since(tVal).Minutes())
		}

		items = append(items, item)
	}
	return items, nil
}

// Helper: Batch delete all documents in a collection[cite: 4]
func deleteAllDocuments(ctx context.Context, client *firestore.Client, collection string) error {
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

		batch.Delete(doc.Ref)
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

// Helper: Batch update field in collection[cite: 4]
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

// Render HTML template matching the table layout without image elements[cite: 4]
func renderHTML(w http.ResponseWriter, zlist []ZListItem) {
	tmplString := `<!doctype html>
<html lang="en">
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://code.jquery.com/mobile/1.4.5/jquery.mobile-1.4.5.min.css">
<script src="https://code.jquery.com/jquery-1.11.3.min.js"></script>
<script src="https://code.jquery.com/mobile/1.4.5/jquery.mobile-1.4.5.min.js"></script>
</head>
<body>
<table style="width: 100%;">
{{range .ZList}}
	<tr{{if gt .TT 200}} style='background-color: lightgrey;'{{else if gt .TT 60}} style='background-color: pink;'{{else if gt .TT 30}} style='background-color: yellow;'{{else if gt .D 5}} style=' color: red;'{{end}}>
		<td>{{.H}}</td>
		<td>{{.I}}</td>
		<td>{{.R}}</td>
		<td>{{.D}}</td>
		<td>{{.C}}</td>
		<td>{{.TT}}</td>
	</tr>
{{end}}
</table>

<form action="#" method="post">
<input type="hidden" name="z" value="z"/>
<input type="submit" value="Reload"/>
</form>
</body>
</html>`

	tmpl := template.Must(template.New("dashboard").Parse(tmplString))
	_ = tmpl.Execute(w, map[string]interface{}{
		"ZList": zlist,
	})
}