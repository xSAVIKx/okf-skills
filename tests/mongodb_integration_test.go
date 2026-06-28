package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoDBIntegration(t *testing.T) {
	bin := getBinaryPath("okf-mongodb")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-mongodb binary not built: %v", err)
	}
	if !isPortOpen("127.0.0.1", 27017) {
		t.Skip("MongoDB not reachable on 127.0.0.1:27017 (start tests/docker-compose)")
	}
	uri := "mongodb://127.0.0.1:27017"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Disconnect(ctx)

	dbName := "okf_mongo_it"
	db := client.Database(dbName)
	_ = db.Drop(ctx) // clean slate
	defer db.Drop(ctx)

	// Seed an orders collection with a mixed-type field and an optional field.
	orders := db.Collection("orders")
	if _, err := orders.InsertMany(ctx, []interface{}{
		bson.M{"_id": 1, "total": 10.5, "status": "active"},
		bson.M{"_id": 2, "total": 20.0},
		bson.M{"_id": 3, "total": "n/a", "status": "pending"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out := filepath.Join(t.TempDir(), "bundle")
	if o, err := exec.Command(bin, "produce", "--uri", uri, "--db", dbName, "--out", out, "--sample", "100").CombinedOutput(); err != nil {
		t.Fatalf("produce failed: %v\n%s", err, o)
	}

	body, err := os.ReadFile(filepath.Join(out, "collections", "orders.md"))
	if err != nil {
		t.Fatalf("collections/orders.md not produced: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"type: MongoDB Collection",
		"# Columns",
		"| Name | Type | Presence |",
		"number|string", // total has mixed types across docs
		"| status |",    // present in 2 of 3 docs
	} {
		if !strings.Contains(s, want) {
			t.Errorf("orders.md missing %q:\n%s", want, s)
		}
	}

	// Root index.md carries only okf_version.
	idx, _ := os.ReadFile(filepath.Join(out, "index.md"))
	if !strings.Contains(string(idx), "okf_version: \"0.1\"") {
		t.Errorf("index.md missing okf_version")
	}

	// ingest --sync creates a missing collection present only in the bundle.
	if err := os.WriteFile(filepath.Join(out, "collections", "customers.md"),
		[]byte("---\ntype: MongoDB Collection\ntitle: customers\ndescription: x\n---\n# Columns\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if o, err := exec.Command(bin, "ingest", "--uri", uri, "--db", dbName, "--bundle", out, "--sync").CombinedOutput(); err != nil {
		t.Fatalf("ingest --sync failed: %v\n%s", err, o)
	}
	names, _ := db.ListCollectionNames(ctx, bson.M{})
	found := false
	for _, n := range names {
		if n == "customers" {
			found = true
		}
	}
	if !found {
		t.Errorf("ingest --sync did not create the missing 'customers' collection: %v", names)
	}
}
