// Package main implements the MongoDB OKF (Open Knowledge Format) connector. It
// samples documents from each collection in a database to infer a top-level field
// schema (name, type, presence) and emits one concept per collection. ingest verifies
// the bundle against the live database and can create missing collections; MongoDB has
// no per-field comment store, so descriptions remain in the bundle.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// version is the build version, injected via -ldflags by install.sh.
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "produce":
		runProduce(os.Args[2:])
	case "ingest":
		runIngest(os.Args[2:])
	case "schema":
		if err := okf.PrintSchema(os.Stdout, buildSchema()); err != nil {
			log.Fatalf("Failed to print schema: %v", err)
		}
	case "version", "--version", "-v":
		fmt.Println(version)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: okf-mongodb <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create an OKF bundle from a MongoDB database (samples documents)")
	fmt.Println("  ingest   - Verify a bundle against the database; --sync creates missing collections")
}

// resolveURI returns the connection URI from the flag or the MONGODB_URI env var.
func resolveURI(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv("MONGODB_URI")
}

// connect dials MongoDB and pings to fail fast on a bad URI/credentials.
func connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return client, nil
}

func runProduce(args []string) {
	fsSet := flag.NewFlagSet("produce", flag.ExitOnError)
	uriFlag := fsSet.String("uri", "", "MongoDB connection URI (or MONGODB_URI env)")
	dbName := fsSet.String("db", "", "Database name (required)")
	collFilter := fsSet.String("collections", "", "Comma-separated collections to extract (optional)")
	sample := fsSet.Int("sample", 100, "Documents to sample per collection for schema inference")
	outDir := fsSet.String("out", "", "Output bundle directory (required)")
	fsSet.Parse(args)

	uri := resolveURI(*uriFlag)
	if uri == "" || *dbName == "" || *outDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := connect(ctx, uri)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)
	db := client.Database(*dbName)

	names, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		log.Fatalf("Failed to list collections: %v", err)
	}
	names = filterAndSort(names, *collFilter)

	timestamp := time.Now().Format(time.RFC3339)
	today := time.Now().Format("2006-01-02")
	cleanURI := stripCreds(uri)

	for _, name := range names {
		docs, err := sampleCollection(ctx, db.Collection(name), int64(*sample))
		if err != nil {
			log.Fatalf("Failed to sample collection %s: %v", name, err)
		}
		fields := inferFields(docs)

		fresh := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
				Type:        "MongoDB Collection",
				Title:       name,
				Description: fmt.Sprintf("MongoDB collection %s", name),
				Resource:    fmt.Sprintf("%s/%s/%s", cleanURI, *dbName, name),
				Tags:        []string{"mongodb", "collection"},
				Timestamp:   timestamp,
			},
			Body: renderColumns(fields),
		}

		conceptPath := filepath.Join(*outDir, "collections", name+".md")
		if err := os.MkdirAll(filepath.Dir(conceptPath), 0o755); err != nil {
			log.Fatalf("Failed to create concept subdirectories: %v", err)
		}
		var existing *okf.ConceptDoc
		if e, err := okf.ReadConceptDoc(conceptPath); err == nil {
			existing = e
		}
		merged, changed := okf.MergeConcept(existing, fresh)
		if !changed {
			fmt.Printf("Unchanged, preserved: %s\n", conceptPath)
			continue
		}
		if err := okf.WriteConceptDoc(conceptPath, merged); err != nil {
			log.Fatalf("Failed to write concept doc for %s: %v", name, err)
		}
		kind, action := "Update", "Structure changed for"
		if existing == nil {
			kind, action = "Creation", "Established"
		}
		if err := okf.AppendLogEntry(*outDir, today, kind, fmt.Sprintf("%s [%s](/collections/%s.md).", action, name, name)); err != nil {
			log.Fatalf("Failed to append log entry: %v", err)
		}
		fmt.Printf("Produced concept doc: %s\n", conceptPath)
	}

	// Root index.md (only okf_version frontmatter).
	var indexBody bytes.Buffer
	fmt.Fprintf(&indexBody, "# MongoDB Catalog: %s\n\n", *dbName)
	indexBody.WriteString("This OKF bundle documents a MongoDB database.\n\n## Collections\n\n")
	for _, name := range names {
		fmt.Fprintf(&indexBody, "- [%s](collections/%s.md) - MongoDB collection\n", name, name)
	}
	indexDoc := okf.ConceptDoc{
		Frontmatter: okf.Frontmatter{OKFVersion: "0.1"},
		Body:        indexBody.String(),
	}
	if err := okf.WriteConceptDoc(filepath.Join(*outDir, "index.md"), indexDoc); err != nil {
		log.Fatalf("Failed to write index.md: %v", err)
	}
	fmt.Println("Produced index.md successfully.")
}

func runIngest(args []string) {
	fsSet := flag.NewFlagSet("ingest", flag.ExitOnError)
	uriFlag := fsSet.String("uri", "", "MongoDB connection URI (or MONGODB_URI env)")
	dbName := fsSet.String("db", "", "Database name (required)")
	bundleDir := fsSet.String("bundle", "", "OKF bundle path (required)")
	sync := fsSet.Bool("sync", false, "Create missing collections in the database (structure only)")
	fsSet.Parse(args)

	uri := resolveURI(*uriFlag)
	if uri == "" || *dbName == "" || *bundleDir == "" {
		fsSet.Usage()
		os.Exit(1)
	}
	if _, err := os.Stat(*bundleDir); os.IsNotExist(err) {
		log.Fatalf("Bundle directory not found: %s", *bundleDir)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := connect(ctx, uri)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)
	db := client.Database(*dbName)

	existingNames, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		log.Fatalf("Failed to list collections: %v", err)
	}
	existing := map[string]bool{}
	for _, n := range existingNames {
		existing[n] = true
	}

	err = filepath.WalkDir(*bundleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(*bundleDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == "index.md" || relSlash == "log.md" || !strings.HasPrefix(relSlash, "collections/") {
			return nil
		}
		name := strings.TrimSuffix(filepath.Base(relSlash), ".md")
		if existing[name] {
			return nil
		}
		fmt.Printf("Collection '%s' is in the bundle but not in the database.\n", name)
		if *sync {
			if err := db.CreateCollection(ctx, name); err != nil {
				log.Fatalf("Failed to create collection %s: %v", name, err)
			}
			fmt.Printf("  -> Created collection '%s'.\n", name)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Walking bundle failed: %v", err)
	}
	fmt.Println("OKF bundle ingestion finished.")
}

// sampleCollection reads up to limit documents from a collection as generic maps.
func sampleCollection(ctx context.Context, coll *mongo.Collection, limit int64) ([]map[string]interface{}, error) {
	cur, err := coll.Find(ctx, bson.M{}, options.Find().SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []map[string]interface{}
	for cur.Next(ctx) {
		var m bson.M
		if err := cur.Decode(&m); err != nil {
			return nil, err
		}
		docs = append(docs, map[string]interface{}(m))
	}
	return docs, cur.Err()
}

// filterAndSort applies an optional comma-separated allowlist and sorts names.
func filterAndSort(names []string, filter string) []string {
	if strings.TrimSpace(filter) != "" {
		allow := map[string]bool{}
		for _, f := range strings.Split(filter, ",") {
			allow[strings.TrimSpace(f)] = true
		}
		var kept []string
		for _, n := range names {
			if allow[n] {
				kept = append(kept, n)
			}
		}
		names = kept
	}
	sort.Strings(names)
	return names
}
