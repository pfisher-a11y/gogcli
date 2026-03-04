package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

func TestExecute_SheetsListCmd(t *testing.T) {
	origNew := newDriveService
	t.Cleanup(func() { newDriveService = origNew })

	svc, closeSrv := newDriveTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/files") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"files": []map[string]any{
					{"id": "ssid1", "name": "Budget 2024", "modifiedTime": "2024-06-01T10:00:00Z"},
					{"id": "ssid2", "name": "Expenses", "modifiedTime": "2024-05-15T08:30:00Z"},
				},
				"nextPageToken": "",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer closeSrv()

	newDriveService = stubDriveService(svc)
	t.Setenv("GOG_ACCOUNT", "a@b.com")

	_ = captureStderr(t, func() {
		// Text mode — headers and IDs must appear.
		out := captureStdout(t, func() {
			if err := Execute([]string{"sheets", "list"}); err != nil {
				t.Fatalf("list: %v", err)
			}
		})
		if !strings.Contains(out, "ssid1") || !strings.Contains(out, "Budget 2024") {
			t.Fatalf("unexpected text output: %q", out)
		}

		// JSON mode — spreadsheets key must be present.
		jsonOut := captureStdout(t, func() {
			if err := Execute([]string{"--json", "sheets", "list"}); err != nil {
				t.Fatalf("list json: %v", err)
			}
		})
		if !strings.Contains(jsonOut, "ssid1") || !strings.Contains(jsonOut, "spreadsheets") {
			t.Fatalf("unexpected json output: %q", jsonOut)
		}

		// --query flag is forwarded to the Drive query.
		_ = captureStdout(t, func() {
			if err := Execute([]string{"sheets", "list", "--query", "Budget"}); err != nil {
				t.Fatalf("list --query: %v", err)
			}
		})

		// ls alias.
		_ = captureStdout(t, func() {
			if err := Execute([]string{"sheets", "ls"}); err != nil {
				t.Fatalf("list alias: %v", err)
			}
		})
	})
}

func TestExecute_SheetsListCmd_Empty(t *testing.T) {
	origNew := newDriveService
	t.Cleanup(func() { newDriveService = origNew })

	svc, closeSrv := newDriveTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/files") && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"files": []any{}})
			return
		}
		http.NotFound(w, r)
	}))
	defer closeSrv()

	newDriveService = stubDriveService(svc)
	t.Setenv("GOG_ACCOUNT", "a@b.com")

	_ = captureStderr(t, func() {
		// JSON mode with no results should still succeed.
		out := captureStdout(t, func() {
			if err := Execute([]string{"--json", "sheets", "list"}); err != nil {
				t.Fatalf("list empty: %v", err)
			}
		})
		if !strings.Contains(out, "spreadsheets") {
			t.Fatalf("unexpected empty json output: %q", out)
		}
	})
}

func TestExecute_SheetsMoreCommands(t *testing.T) {
	origNew := newSheetsService
	t.Cleanup(func() { newSheetsService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/v4/spreadsheets/id1/values/") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"range":  "Sheet1!A1:B1",
				"values": []any{[]any{"a", "b"}},
			})
			return
		case strings.Contains(path, "/v4/spreadsheets/id1/values/") && strings.Contains(path, ":clear") && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"clearedRange": "Sheet1!A1:B1",
			})
			return
		case strings.Contains(path, "/v4/spreadsheets/id1/values/") && strings.Contains(path, ":append") && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"updates": map[string]any{
					"updatedRange":   "Sheet1!A1:B1",
					"updatedRows":    1,
					"updatedColumns": 2,
					"updatedCells":   2,
				},
			})
			return
		case strings.Contains(path, "/v4/spreadsheets/id1/values/") && r.Method == http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"updatedRange":   "Sheet1!A1:B1",
				"updatedRows":    1,
				"updatedColumns": 2,
				"updatedCells":   2,
			})
			return
		case strings.Contains(path, "/v4/spreadsheets/id1") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "id1",
				"properties":    map[string]any{"title": "T"},
				"sheets": []map[string]any{
					{"properties": map[string]any{"sheetId": 0, "title": "Sheet1"}},
				},
			})
			return
		case strings.Contains(path, "/v4/spreadsheets") && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spreadsheetId": "id2",
				"properties":    map[string]any{"title": "New"},
			})
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("GOG_ACCOUNT", "a@b.com")

	svc, err := sheets.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newSheetsService = func(context.Context, string) (*sheets.Service, error) { return svc, nil }

	_ = captureStderr(t, func() {
		out := captureStdout(t, func() {
			// Text mode (covers table output).
			if err := Execute([]string{"sheets", "get", "id1", `Sheet1\\!A1:B1`}); err != nil {
				t.Fatalf("get: %v", err)
			}
		})
		if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
			t.Fatalf("unexpected out=%q", out)
		}

		plainOut := captureStdout(t, func() {
			if err := Execute([]string{"--plain", "sheets", "get", "id1", `Sheet1\\!A1:B1`}); err != nil {
				t.Fatalf("get plain: %v", err)
			}
		})
		if plainOut != "a\tb\n" {
			t.Fatalf("unexpected plain out=%q", plainOut)
		}

		_ = captureStdout(t, func() {
			if err := Execute([]string{"--json", "sheets", "update", "id1", "Sheet1!A1:B1", "a|b"}); err != nil {
				t.Fatalf("update: %v", err)
			}
		})
		_ = captureStdout(t, func() {
			if err := Execute([]string{"--json", "sheets", "update", "id1", "Sheet1!A1:B1", "--values-json", `[["a","b"]]`}); err != nil {
				t.Fatalf("update json: %v", err)
			}
		})
		_ = captureStdout(t, func() {
			if err := Execute([]string{"--json", "sheets", "append", "id1", "Sheet1!A1:B1", "a|b"}); err != nil {
				t.Fatalf("append: %v", err)
			}
		})
		_ = captureStdout(t, func() {
			if err := Execute([]string{"--json", "sheets", "append", "id1", "Sheet1!A1:B1", "--values-json", `[["a","b"]]`}); err != nil {
				t.Fatalf("append json: %v", err)
			}
		})
		_ = captureStdout(t, func() {
			if err := Execute([]string{"--json", "sheets", "clear", "id1", "Sheet1!A1:B1"}); err != nil {
				t.Fatalf("clear: %v", err)
			}
		})
		_ = captureStdout(t, func() {
			if err := Execute([]string{"--json", "sheets", "metadata", "id1"}); err != nil {
				t.Fatalf("metadata: %v", err)
			}
		})
		_ = captureStdout(t, func() {
			if err := Execute([]string{"--json", "sheets", "create", "New", "--sheets", "Income,Expenses"}); err != nil {
				t.Fatalf("create: %v", err)
			}
		})
	})
}
