/*
Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreedto in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vindexes

import (
	"reflect"
	"strings"
	"testing"

	"github.com/youtube/vitess/go/sqltypes"
)

func TestLookupUniqueNew(t *testing.T) {
	_ = createLookup(t, "lookup_unique", false)

	_, err := CreateVindex("lookup_unique", "lookup_unique", map[string]string{
		"table":             "t",
		"from":              "fromc",
		"to":                "toc",
		"scatter_if_absent": "true",
	})
	want := "scatter_if_absent cannot be true for a unique lookup vindex"
	if err == nil || err.Error() != want {
		t.Errorf("Create(bad_scatter): %v, want %s", err, want)
	}

	_, err = CreateVindex("lookup_unique", "lookup_unique", map[string]string{
		"table":             "t",
		"from":              "fromc",
		"to":                "toc",
		"scatter_if_absent": "invalid",
	})
	want = "scatter_if_absent value must be 'true' or 'false': 'invalid'"
	if err == nil || err.Error() != want {
		t.Errorf("Create(bad_scatter): %v, want %s", err, want)
	}
}

func TestLookupUniqueCost(t *testing.T) {
	lookupUnique := createLookup(t, "lookup_unique", false)
	if lookupUnique.Cost() != 10 {
		t.Errorf("Cost(): %d, want 10", lookupUnique.Cost())
	}
}

func TestLookupUniqueString(t *testing.T) {
	lookupUnique := createLookup(t, "lookup_unique", false)
	if strings.Compare("lookup_unique", lookupUnique.String()) != 0 {
		t.Errorf("String(): %s, want lookup_unique", lookupUnique.String())
	}
}

func TestLookupUniqueMap(t *testing.T) {
	lookupUnique := createLookup(t, "lookup_unique", false)
	vc := &vcursor{numRows: 1}

	got, err := lookupUnique.(Unique).Map(vc, []sqltypes.Value{sqltypes.NewInt64(1), sqltypes.NewInt64(2)})
	if err != nil {
		t.Error(err)
	}
	want := [][]byte{
		[]byte("1"),
		[]byte("1"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Map(): %+v, want %+v", got, want)
	}

	vc.numRows = 0
	got, err = lookupUnique.(Unique).Map(vc, []sqltypes.Value{sqltypes.NewInt64(1), sqltypes.NewInt64(2)})
	if err != nil {
		t.Error(err)
	}
	want = [][]byte{nil, nil}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Map(): %#v, want %+v", got, want)
	}

	vc.numRows = 2
	_, err = lookupUnique.(Unique).Map(vc, []sqltypes.Value{sqltypes.NewInt64(1), sqltypes.NewInt64(2)})
	wantErr := "Lookup.Map: unexpected multiple results from vindex t: INT64(1)"
	if err == nil || err.Error() != wantErr {
		t.Errorf("lookupUnique(query fail) err: %v, want %s", err, wantErr)
	}

	// Test query fail.
	vc.mustFail = true
	_, err = lookupUnique.(Unique).Map(vc, []sqltypes.Value{sqltypes.NewInt64(1)})
	wantErr = "lookup.Map: execute failed"
	if err == nil || err.Error() != wantErr {
		t.Errorf("lookupUnique(query fail) err: %v, want %s", err, wantErr)
	}
	vc.mustFail = false
}

func TestLookupUniqueVerify(t *testing.T) {
	lookupUnique := createLookup(t, "lookup_unique", false)
	vc := &vcursor{numRows: 1}

	_, err := lookupUnique.Verify(vc, []sqltypes.Value{sqltypes.NewInt64(1)}, [][]byte{[]byte("test")})
	if err != nil {
		t.Error(err)
	}
	if got, want := len(vc.queries), 1; got != want {
		t.Errorf("vc.queries length: %v, want %v", got, want)
	}
}

func TestLookupUniqueCreate(t *testing.T) {
	lookupUnique := createLookup(t, "lookup_unique", false)
	vc := &vcursor{}

	err := lookupUnique.(Lookup).Create(vc, [][]sqltypes.Value{{sqltypes.NewInt64(1)}}, [][]byte{[]byte("test")}, false /* ignoreMode */)
	if err != nil {
		t.Error(err)
	}
	if got, want := len(vc.queries), 1; got != want {
		t.Errorf("vc.queries length: %v, want %v", got, want)
	}
}

func TestLookupUniqueDelete(t *testing.T) {
	lookupUnique := createLookup(t, "lookup_unique", false)
	vc := &vcursor{}

	err := lookupUnique.(Lookup).Delete(vc, [][]sqltypes.Value{{sqltypes.NewInt64(1)}}, []byte("test"))
	if err != nil {
		t.Error(err)
	}
	if got, want := len(vc.queries), 1; got != want {
		t.Errorf("vc.queries length: %v, want %v", got, want)
	}
}

func TestLookupUniqueUpdate(t *testing.T) {
	lookupUnique := createLookup(t, "lookup_unique", false)
	vc := &vcursor{}

	err := lookupUnique.(Lookup).Update(vc, []sqltypes.Value{sqltypes.NewInt64(1)}, []byte("test"), []sqltypes.Value{sqltypes.NewInt64(2)})
	if err != nil {
		t.Error(err)
	}
	if got, want := len(vc.queries), 2; got != want {
		t.Errorf("vc.queries length: %v, want %v", got, want)
	}
}
