package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestServerListsTools(t *testing.T) {
	server := New("http://example.test", "secret")

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/list",
	})

	if response.Error != nil {
		t.Fatalf("tools/list error = %#v", response.Error)
	}
	body := mustJSON(t, response.Result)
	if !strings.Contains(body, "foliospace.client_info") ||
		!strings.Contains(body, "foliospace.list_games") ||
		!strings.Contains(body, "foliospace.open_game_manifest") ||
		!strings.Contains(body, "foliospace.list_favorites") ||
		!strings.Contains(body, "foliospace.list_collection_volumes") ||
		!strings.Contains(body, "foliospace.pause_job") ||
		!strings.Contains(body, "foliospace.library_health") {
		t.Fatalf("tools/list response %s missing expected tools", body)
	}
}

func TestServerCallsHTTPToolWithBearerToken(t *testing.T) {
	var sawAuth string
	server := New("http://foliospace.test", "secret")
	server.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		sawAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/client/info" {
			t.Fatalf("path = %s, want /api/client/info", r.URL.Path)
		}
		return jsonResponse(`{"serviceName":"FolioSpace Library"}`), nil
	})}

	response := server.Handle(context.Background(), toolCall(t, "foliospace.client_info", nil))

	if response.Error != nil {
		t.Fatalf("tool call error = %#v", response.Error)
	}
	if sawAuth != "Bearer secret" {
		t.Fatalf("Authorization = %q, want bearer token", sawAuth)
	}
	body := mustJSON(t, response.Result)
	if !strings.Contains(body, "FolioSpace Library") {
		t.Fatalf("tool response %s missing HTTP body", body)
	}
}

func TestServerCallsParameterizedTool(t *testing.T) {
	var gotPath string
	server := New("http://foliospace.test", "")
	server.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.RequestURI()
		return jsonResponse(`{"game":{"id":12},"fileUrl":"/api/client/games/12/file"}`), nil
	})}

	response := server.Handle(context.Background(), toolCall(t, "foliospace.open_game_manifest", map[string]any{"gameId": 12}))

	if response.Error != nil {
		t.Fatalf("tool call error = %#v", response.Error)
	}
	if gotPath != "/api/client/games/12/manifest" {
		t.Fatalf("path = %s, want game manifest path", gotPath)
	}
	body := mustJSON(t, response.Result)
	if !strings.Contains(body, "fileUrl") {
		t.Fatalf("tool response %s missing manifest", body)
	}
}

func TestServerCallsClientGamesTool(t *testing.T) {
	var gotPath string
	server := New("http://foliospace.test", "")
	server.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.RequestURI()
		return jsonResponse(`{"items":[],"total":0,"limit":50,"offset":0,"hasMore":false}`), nil
	})}

	response := server.Handle(context.Background(), toolCall(t, "foliospace.list_games", map[string]any{
		"limit":    50,
		"offset":   100,
		"q":        "contra",
		"platform": "nes",
		"format":   "nes",
		"sort":     "title",
	}))

	if response.Error != nil {
		t.Fatalf("tool call error = %#v", response.Error)
	}
	if gotPath != "/api/client/games?format=nes&limit=50&offset=100&platform=nes&q=contra&sort=title" {
		t.Fatalf("path = %s, want client games query", gotPath)
	}
}

func TestServerCallsPrivateShelfTools(t *testing.T) {
	var paths []string
	server := New("http://foliospace.test", "")
	server.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.RequestURI())
		return jsonResponse(`[]`), nil
	})}

	favoriteResponse := server.Handle(context.Background(), toolCall(t, "foliospace.list_favorites", map[string]any{"limit": 5}))
	if favoriteResponse.Error != nil {
		t.Fatalf("favorites tool error = %#v", favoriteResponse.Error)
	}
	statusResponse := server.Handle(context.Background(), toolCall(t, "foliospace.list_private_status", map[string]any{"status": "want", "limit": 7}))
	if statusResponse.Error != nil {
		t.Fatalf("private-status tool error = %#v", statusResponse.Error)
	}

	if strings.Join(paths, "\n") != "/api/client/books/favorites?limit=5\n/api/client/books/private-status/want?limit=7" {
		t.Fatalf("paths = %#v, want private shelf routes", paths)
	}
}

func TestServerCallsCollectionVolumesTool(t *testing.T) {
	var gotPath string
	server := New("http://foliospace.test", "")
	server.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.RequestURI()
		return jsonResponse(`{"items":[],"total":0}`), nil
	})}

	response := server.Handle(context.Background(), toolCall(t, "foliospace.list_collection_volumes", map[string]any{
		"collectionId": 42,
		"limit":        20,
		"offset":       40,
		"q":            "space",
		"sort":         "title",
	}))

	if response.Error != nil {
		t.Fatalf("collection volumes tool error = %#v", response.Error)
	}
	if gotPath != "/api/collections/42/volumes?limit=20&offset=40&q=space&sort=title" {
		t.Fatalf("path = %s, want collection volumes query", gotPath)
	}
}

func TestServerCallsJobControlTools(t *testing.T) {
	var calls []string
	server := New("http://foliospace.test", "")
	server.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.Method+" "+r.URL.RequestURI())
		return jsonResponse(`{"id":9,"status":"pause_requested"}`), nil
	})}

	for _, name := range []string{"foliospace.pause_job", "foliospace.cancel_job", "foliospace.resume_job"} {
		response := server.Handle(context.Background(), toolCall(t, name, map[string]any{"jobId": 9}))
		if response.Error != nil {
			t.Fatalf("%s error = %#v", name, response.Error)
		}
	}

	got := strings.Join(calls, "\n")
	want := "POST /api/jobs/9/pause\nPOST /api/jobs/9/cancel\nPOST /api/jobs/9/resume"
	if got != want {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestServerAggregatesLibraryHealth(t *testing.T) {
	server := New("http://foliospace.test", "")
	server.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/client/info":
			return jsonResponse(`{"serviceName":"FolioSpace Library"}`), nil
		case "/api/jobs":
			return jsonResponse(`[{"id":1,"status":"completed"},{"id":2,"status":"running"}]`), nil
		case "/api/errors":
			return jsonResponse(`[{"id":7,"code":"archive_open_failed"}]`), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return jsonResponse(`{}`), nil
	})}

	response := server.Handle(context.Background(), toolCall(t, "foliospace.library_health", nil))

	if response.Error != nil {
		t.Fatalf("tool call error = %#v", response.Error)
	}
	body := mustJSON(t, response.Result)
	if !strings.Contains(body, `\"jobCount\": 2`) || !strings.Contains(body, `\"errorCount\": 1`) {
		t.Fatalf("health response %s missing summary", body)
	}
}

func toolCall(t *testing.T, name string, arguments map[string]any) Request {
	t.Helper()
	if arguments == nil {
		arguments = map[string]any{}
	}
	params, err := json.Marshal(map[string]any{"name": name, "arguments": arguments})
	if err != nil {
		t.Fatal(err)
	}
	return Request{JSONRPC: "2.0", ID: float64(1), Method: "tools/call", Params: params}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
