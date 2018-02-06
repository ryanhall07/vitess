/*
Copyright 2017 Google Inc.

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

package vindexes

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/youtube/vitess/go/sqltypes"
	"github.com/youtube/vitess/go/vt/proto/topodata"
)

var (
	_ NonUnique = (*LookupHash)(nil)
	_ Lookup    = (*LookupHash)(nil)
	_ Unique    = (*LookupHashUnique)(nil)
	_ Lookup    = (*LookupHashUnique)(nil)
)

func init() {
	Register("lookup_hash", NewLookupHash)
	Register("lookup_hash_unique", NewLookupHashUnique)
}

//====================================================================

// LookupHash defines a vindex that uses a lookup table.
// The table is expected to define the id column as unique. It's
// NonUnique and a Lookup.
// Warning: This Vindex is being depcreated in favor of Lookup
type LookupHash struct {
	name            string
	scatterIfAbsent bool
	ignore          bool
	lkp             lookupInternal
}

// NewLookupHash creates a LookupHash vindex.
// The supplied map has the following required fields:
//   table: name of the backing table. It can be qualified by the keyspace.
//   from: list of columns in the table that have the 'from' values of the lookup vindex.
//   to: The 'to' column name of the table.
//   autocommit_on_insert: setting this to "true" will cause inserts to autocommit.
//   disallow_update: this will fail attempts to change vindex values through update statements.
//   upsert_on_insert: this will change inserts to upserts, which can overwrite an existing mapping. Use with caution.
//   upsert_on_update: this will change updates to upserts, which can create new entries where there were none. Use with caution.
//   scatter_if_absent: if an entry is missing, this flag will the query to be sent to all shards.
func NewLookupHash(name string, m map[string]string) (Vindex, error) {
	lh := &LookupHash{name: name}
	if err := lh.lkp.Init(m); err != nil {
		return nil, err
	}
	var err error
	lh.scatterIfAbsent, err = boolFromMap(m, "scatter_if_absent")
	lh.ignore, err = boolFromMap(m, "ignore")
	if err != nil {
		return nil, err
	}
	return lh, nil
}

// String returns the name of the vindex.
func (lh *LookupHash) String() string {
	return lh.name
}

// Cost returns the cost of this vindex as 20.
func (lh *LookupHash) Cost() int {
	return 20
}

// Map returns the corresponding KeyspaceId values for the given ids.
func (lh *LookupHash) Map(vcursor VCursor, ids []sqltypes.Value) ([]Ksids, error) {
	out := make([]Ksids, 0, len(ids))
	if lh.ignore {
		for range ids {
			out = append(out, Ksids{Range: &topodata.KeyRange{}})
		}
		return out, nil
	}
	results, err := lh.lkp.Lookup(vcursor, ids)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		if len(result.Rows) == 0 {
			if lh.scatterIfAbsent {
				out = append(out, Ksids{Range: &topodata.KeyRange{}})
				continue
			}
			out = append(out, Ksids{})
			continue
		}
		ksids := make([][]byte, 0, len(result.Rows))
		for _, row := range result.Rows {
			num, err := sqltypes.ToUint64(row[0])
			if err != nil {
				// A failure to convert is equivalent to not being
				// able to map.
				continue
			}
			ksids = append(ksids, vhash(num))
		}
		out = append(out, Ksids{IDs: ksids})
	}
	return out, nil
}

// Verify returns true if ids maps to ksids.
func (lh *LookupHash) Verify(vcursor VCursor, ids []sqltypes.Value, ksids [][]byte) ([]bool, error) {
	if lh.scatterIfAbsent || lh.ignore {
		out := make([]bool, len(ids))
		for i := range ids {
			out[i] = true
		}
		return out, nil
	}
	values, err := unhashList(ksids)
	if err != nil {
		return nil, fmt.Errorf("lookup.Verify.vunhash: %v", err)
	}
	return lh.lkp.Verify(vcursor, ids, values)
}

// Create reserves the id by inserting it into the vindex table.
func (lh *LookupHash) Create(vcursor VCursor, rowsColValues [][]sqltypes.Value, ksids [][]byte, ignoreMode bool) error {
	values, err := unhashList(ksids)
	if err != nil {
		return fmt.Errorf("lookup.Create.vunhash: %v", err)
	}
	return lh.lkp.Create(vcursor, rowsColValues, values, ignoreMode)
}

// Update updates the entry in the vindex table.
func (lh *LookupHash) Update(vcursor VCursor, oldValues []sqltypes.Value, ksid []byte, newValues []sqltypes.Value) error {
	v, err := vunhash(ksid)
	if err != nil {
		return fmt.Errorf("lookup.Update.vunhash: %v", err)
	}
	return lh.lkp.Update(vcursor, oldValues, sqltypes.NewUint64(v), newValues)
}

// Delete deletes the entry from the vindex table.
func (lh *LookupHash) Delete(vcursor VCursor, rowsColValues [][]sqltypes.Value, ksid []byte) error {
	v, err := vunhash(ksid)
	if err != nil {
		return fmt.Errorf("lookup.Delete.vunhash: %v", err)
	}
	return lh.lkp.Delete(vcursor, rowsColValues, sqltypes.NewUint64(v))
}

// MarshalJSON returns a JSON representation of LookupHash.
func (lh *LookupHash) MarshalJSON() ([]byte, error) {
	return json.Marshal(lh.lkp)
}

// unhashList unhashes a list of keyspace ids into []sqltypes.Value.
func unhashList(ksids [][]byte) ([]sqltypes.Value, error) {
	values := make([]sqltypes.Value, 0, len(ksids))
	for _, ksid := range ksids {
		v, err := vunhash(ksid)
		if err != nil {
			return nil, err
		}
		values = append(values, sqltypes.NewUint64(v))
	}
	return values, nil
}

//====================================================================

// LookupHashUnique defines a vindex that uses a lookup table.
// The table is expected to define the id column as unique. It's
// Unique and a Lookup.
// Warning: This Vindex is being depcreated in favor of LookupUnique
type LookupHashUnique struct {
	name string
	lkp  lookupInternal
}

// NewLookupHashUnique creates a LookupHashUnique vindex.
// The supplied map has the following required fields:
//   table: name of the backing table. It can be qualified by the keyspace.
//   from: list of columns in the table that have the 'from' values of the lookup vindex.
//   to: The 'to' column name of the table.
//   autocommit_on_insert: setting this to "true" will cause inserts to autocommit.
//   disallow_update: this will fail attempts to change vindex values through update statements.
//   upsert_on_insert: this will change inserts to upserts, which can overwrite an existing mapping. Use with caution.
//   upsert_on_update: this will change updates to upserts, which can create new entries where there were none. Use with caution.
func NewLookupHashUnique(name string, m map[string]string) (Vindex, error) {
	lhu := &LookupHashUnique{name: name}
	if err := lhu.lkp.Init(m); err != nil {
		return nil, err
	}
	scatter, err := boolFromMap(m, "scatter_if_absent")
	if err != nil {
		return nil, err
	}
	if scatter {
		return nil, errors.New("scatter_if_absent cannot be true for a unique lookup vindex")
	}
	ignore, err := boolFromMap(m, "ignore")
	if err != nil {
		return nil, err
	}
	if ignore {
		return nil, errors.New("ignore cannot be true for a unique lookup vindex")
	}
	return lhu, nil
}

// String returns the name of the vindex.
func (lhu *LookupHashUnique) String() string {
	return lhu.name
}

// Cost returns the cost of this vindex as 10.
func (lhu *LookupHashUnique) Cost() int {
	return 10
}

// Map returns the corresponding KeyspaceId values for the given ids.
func (lhu *LookupHashUnique) Map(vcursor VCursor, ids []sqltypes.Value) ([][]byte, error) {
	out := make([][]byte, 0, len(ids))
	results, err := lhu.lkp.Lookup(vcursor, ids)
	if err != nil {
		return nil, err
	}
	for i, result := range results {
		switch len(result.Rows) {
		case 0:
			out = append(out, nil)
		case 1:
			num, err := sqltypes.ToUint64(result.Rows[0][0])
			if err != nil {
				out = append(out, nil)
				continue
			}
			out = append(out, vhash(num))
		default:
			return nil, fmt.Errorf("LookupHash.Map: unexpected multiple results from vindex %s: %v", lhu.lkp.Table, ids[i])
		}
	}
	return out, nil
}

// Verify returns true if ids maps to ksids.
func (lhu *LookupHashUnique) Verify(vcursor VCursor, ids []sqltypes.Value, ksids [][]byte) ([]bool, error) {
	values, err := unhashList(ksids)
	if err != nil {
		return nil, fmt.Errorf("lookup.Verify.vunhash: %v", err)
	}
	return lhu.lkp.Verify(vcursor, ids, values)
}

// Create reserves the id by inserting it into the vindex table.
func (lhu *LookupHashUnique) Create(vcursor VCursor, rowsColValues [][]sqltypes.Value, ksids [][]byte, ignoreMode bool) error {
	values, err := unhashList(ksids)
	if err != nil {
		return fmt.Errorf("lookup.Create.vunhash: %v", err)
	}
	return lhu.lkp.Create(vcursor, rowsColValues, values, ignoreMode)
}

// Delete deletes the entry from the vindex table.
func (lhu *LookupHashUnique) Delete(vcursor VCursor, rowsColValues [][]sqltypes.Value, ksid []byte) error {
	v, err := vunhash(ksid)
	if err != nil {
		return fmt.Errorf("lookup.Delete.vunhash: %v", err)
	}
	return lhu.lkp.Delete(vcursor, rowsColValues, sqltypes.NewUint64(v))
}

// Update updates the entry in the vindex table.
func (lhu *LookupHashUnique) Update(vcursor VCursor, oldValues []sqltypes.Value, ksid []byte, newValues []sqltypes.Value) error {
	v, err := vunhash(ksid)
	if err != nil {
		return fmt.Errorf("lookup.Update.vunhash: %v", err)
	}
	return lhu.lkp.Update(vcursor, oldValues, sqltypes.NewUint64(v), newValues)
}

// MarshalJSON returns a JSON representation of LookupHashUnique.
func (lhu *LookupHashUnique) MarshalJSON() ([]byte, error) {
	return json.Marshal(lhu.lkp)
}
