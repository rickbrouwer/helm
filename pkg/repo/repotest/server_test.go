/*
Copyright The Helm Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package repotest

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/internal/test/ensure"
	"helm.sh/helm/v4/pkg/repo"
)

// Young'n, in these here parts, we test our tests.

func TestServer(t *testing.T) {
	ensure.HelmHome(t)

	rootDir := t.TempDir()

	srv := newServer(t, rootDir)
	defer srv.Stop()

	c, err := srv.CopyCharts("testdata/*.tgz")
	if err != nil {
		// Some versions of Go don't correctly fire defer on Fatal.
		t.Fatal(err)
	}

	if len(c) != 1 {
		t.Errorf("Unexpected chart count: %d", len(c))
	}

	if filepath.Base(c[0]) != "examplechart-0.1.0.tgz" {
		t.Errorf("Unexpected chart: %s", c[0])
	}

	res, err := http.Get(srv.URL() + "/examplechart-0.1.0.tgz")
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	if res.ContentLength < 500 {
		t.Errorf("Expected at least 500 bytes of data, got %d", res.ContentLength)
	}

	res, err = http.Get(srv.URL() + "/index.yaml")
	if err != nil {
		t.Fatal(err)
	}

	data, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	m := repo.NewIndexFile()
	if err := yaml.Unmarshal(data, m); err != nil {
		t.Fatal(err)
	}

	if l := len(m.Entries); l != 1 {
		t.Fatalf("Expected 1 entry, got %d", l)
	}

	expect := "examplechart"
	if !m.Has(expect, "0.1.0") {
		t.Errorf("missing %q", expect)
	}

	res, err = http.Get(srv.URL() + "/index.yaml-nosuchthing")
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", res.StatusCode)
	}
}

func TestNewTempServer(t *testing.T) {
	ensure.HelmHome(t)

	type testCase struct {
		options []ServerOption
	}

	testCases := map[string]testCase{
		"plainhttp": {
			options: []ServerOption{
				WithChartSourceGlob("testdata/examplechart-0.1.0.tgz"),
			},
		},
		"tls": {
			options: []ServerOption{
				WithChartSourceGlob("testdata/examplechart-0.1.0.tgz"),
				WithTLSConfig(MakeTestTLSConfig(t, "../../../testdata")),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			srv := NewTempServer(
				t,
				tc.options...,
			)
			defer srv.Stop()

			if srv.srv.URL == "" {
				t.Fatal("unstarted server")
			}

			client := srv.Client()

			{
				res, err := client.Head(srv.URL() + "/repositories.yaml")
				if err != nil {
					t.Error(err)
				}

				res.Body.Close()

				if res.StatusCode != http.StatusOK {
					t.Errorf("Expected 200, got %d", res.StatusCode)
				}

			}

			{
				res, err := client.Head(srv.URL() + "/examplechart-0.1.0.tgz")
				if err != nil {
					t.Error(err)
				}
				res.Body.Close()

				if res.StatusCode != http.StatusOK {
					t.Errorf("Expected 200, got %d", res.StatusCode)
				}
			}

			res, err := client.Get(srv.URL() + "/examplechart-0.1.0.tgz")
			res.Body.Close()
			if err != nil {
				t.Fatal(err)
			}

			if res.ContentLength < 500 {
				t.Errorf("Expected at least 500 bytes of data, got %d", res.ContentLength)
			}

			res, err = client.Get(srv.URL() + "/index.yaml")
			if err != nil {
				t.Fatal(err)
			}

			data, err := io.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				t.Fatal(err)
			}

			m := repo.NewIndexFile()
			if err := yaml.Unmarshal(data, m); err != nil {
				t.Fatal(err)
			}

			if l := len(m.Entries); l != 1 {
				t.Fatalf("Expected 1 entry, got %d", l)
			}

			expect := "examplechart"
			if !m.Has(expect, "0.1.0") {
				t.Errorf("missing %q", expect)
			}

			res, err = client.Get(srv.URL() + "/index.yaml-nosuchthing")
			res.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != http.StatusNotFound {
				t.Fatalf("Expected 404, got %d", res.StatusCode)
			}
		})
	}

}

func TestNewTempServer_TLS(t *testing.T) {
	ensure.HelmHome(t)

	srv := NewTempServer(
		t,
		WithChartSourceGlob("testdata/examplechart-0.1.0.tgz"),
		WithTLSConfig(MakeTestTLSConfig(t, "../../../testdata")),
	)
	defer srv.Stop()

	if !strings.HasPrefix(srv.URL(), "https://") {
		t.Fatal("non-TLS server")
	}
}
