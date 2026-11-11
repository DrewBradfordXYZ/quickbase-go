package quickbase_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v2"
)

// exportSolution returns QBL as application/x-yaml, and createSolution takes
// it. Both were declared JSON, so ExportSolution failed on every success and
// CreateSolution sent JSON.
func TestSolutionsSendAndReturnQBL(t *testing.T) {
	const qbl = "Solution:\n  Name: Example solution\nVersion: 0.2\nResources: {}\n"
	var gotType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/solutions/abc":
			w.Header().Set("Content-Type", "application/x-yaml")
			_, _ = io.WriteString(w, qbl)
		case r.Method == http.MethodPost && r.URL.Path == "/solutions":
			gotType = r.Header.Get("Content-Type")
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"solutionId":"abc"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	qb, err := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	resp, err := qb.ExportSolution("abc").Run(ctx)
	if err != nil {
		t.Fatalf("ExportSolution: %v", err)
	}
	if string(resp.Body) != qbl {
		t.Errorf("ExportSolution body = %q, want the QBL", resp.Body)
	}

	if _, err := qb.CreateSolution().Body([]byte(qbl)).Run(ctx); err != nil {
		t.Fatalf("CreateSolution: %v", err)
	}
	if gotType != "application/x-yaml" || gotBody != qbl {
		t.Errorf("CreateSolution sent %q as %q, want the QBL as application/x-yaml", gotBody, gotType)
	}
}
