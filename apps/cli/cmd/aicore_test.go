package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAICoreDeploymentList(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "ai-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/lm/deployments" {
			t.Errorf("path=%q", r.URL.Path)
		}
		if got := r.URL.Query().Get("$resourceGroup"); got != "default" {
			t.Errorf("resourceGroup=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":1,"resources":[
			{"id":"d-1","status":"RUNNING","scenarioId":"foundation-models"}
		]}`))
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	aiShared = aiFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"aicore", "deployment", "list",
		"--cred", "ai-trial",
		"--api", api.URL,
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
	var got struct {
		Count       int `json:"count"`
		Deployments []struct {
			ID, Status, ScenarioID string
		} `json:"deployments"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if got.Count != 1 || got.Deployments[0].ID != "d-1" {
		t.Fatalf("output: %+v", got)
	}
}

func TestAICoreModels(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "ai-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/v2/admin/scenarios/foundation-models/models") {
			t.Errorf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resources":[
			{"model":"anthropic--claude-3-7-sonnet","description":"Anthropic Claude 3.7 Sonnet"},
			{"model":"openai--gpt-5","description":"OpenAI GPT-5"}
		]}`))
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	aiShared = aiFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"aicore", "genai-hub", "models",
		"--cred", "ai-trial",
		"--api", api.URL,
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
	var got struct {
		Count  int `json:"count"`
		Models []struct {
			Model string
		} `json:"models"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if got.Count != 2 {
		t.Fatalf("count=%d", got.Count)
	}
}

func TestAICoreComplete(t *testing.T) {
	withTempCache(t)

	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "ai-trial", oauth.URL+"/oauth/token")

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/v2/inference/deployments/d-1/chat/completions") {
			t.Errorf("path=%q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"role":"user"`) {
			t.Errorf("body=%s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	}))
	defer api.Close()

	globalFlags = GlobalFlags{}
	aiShared = aiFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"aicore", "genai-hub", "complete",
		"--cred", "ai-trial",
		"--api", api.URL,
		"--deployment", "d-1",
		"--prompt", "hello",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}
	if !strings.Contains(out.String(), `"choices"`) {
		t.Fatalf("missing pass-through body: %s", out.String())
	}
}

func TestAICoreMissingApi(t *testing.T) {
	withTempCache(t)
	oauth := btpFakeOAuth(t)
	defer oauth.Close()
	btpSeedXSUAACred(t, "ai-trial", oauth.URL+"/oauth/token")

	globalFlags = GlobalFlags{}
	aiShared = aiFlags{}

	root := NewRootCmd("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"aicore", "deployment", "list",
		"--cred", "ai-trial",
	})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--api is required") {
		t.Fatalf("expected --api required, got %v", err)
	}
}
