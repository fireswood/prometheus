// Copyright 2013 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metric

import (
	"sort"

	"code.google.com/p/goprotobuf/proto"

	clientmodel "github.com/prometheus/client_golang/model"

	dto "github.com/prometheus/prometheus/model/generated"

	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/raw"
	"github.com/prometheus/prometheus/storage/raw/leveldb"
)

type FingerprintMetricMapping map[clientmodel.Fingerprint]clientmodel.Metric

type FingerprintMetricIndex interface {
	raw.Pruner

	IndexBatch(FingerprintMetricMapping) error
	Lookup(*clientmodel.Fingerprint) (m clientmodel.Metric, ok bool, err error)
	State() *raw.DatabaseState
	Size() (s uint64, present bool, err error)
}

type LevelDBFingerprintMetricIndex struct {
	p *leveldb.LevelDBPersistence
}

type LevelDBFingerprintMetricIndexOptions struct {
	leveldb.LevelDBOptions
}

func (i *LevelDBFingerprintMetricIndex) Close() {
	i.p.Close()
}

func (i *LevelDBFingerprintMetricIndex) State() *raw.DatabaseState {
	return i.p.State()
}

func (i *LevelDBFingerprintMetricIndex) Size() (uint64, bool, error) {
	s, err := i.p.Size()
	return s, true, err
}

func (i *LevelDBFingerprintMetricIndex) IndexBatch(mapping FingerprintMetricMapping) error {
	b := leveldb.NewBatch()
	defer b.Close()

	for f, m := range mapping {
		k := new(dto.Fingerprint)
		dumpFingerprint(k, &f)
		v := new(dto.Metric)
		dumpMetric(v, m)

		b.Put(k, v)
	}

	return i.p.Commit(b)
}

func (i *LevelDBFingerprintMetricIndex) Lookup(f *clientmodel.Fingerprint) (m clientmodel.Metric, ok bool, err error) {
	k := new(dto.Fingerprint)
	dumpFingerprint(k, f)
	v := new(dto.Metric)
	if ok, err := i.p.Get(k, v); !ok {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}

	m = clientmodel.Metric{}

	for _, pair := range v.LabelPair {
		m[clientmodel.LabelName(pair.GetName())] = clientmodel.LabelValue(pair.GetValue())
	}

	return m, true, nil
}

func (i *LevelDBFingerprintMetricIndex) Prune() (bool, error) {
	i.p.Prune()

	return false, nil
}

func NewLevelDBFingerprintMetricIndex(o *LevelDBFingerprintMetricIndexOptions) (*LevelDBFingerprintMetricIndex, error) {
	s, err := leveldb.NewLevelDBPersistence(&o.LevelDBOptions)
	if err != nil {
		return nil, err
	}

	return &LevelDBFingerprintMetricIndex{
		p: s,
	}, nil
}

type LabelNameFingerprintMapping map[clientmodel.LabelName]clientmodel.Fingerprints

type LabelNameFingerprintIndex interface {
	raw.Pruner

	IndexBatch(LabelNameFingerprintMapping) error
	Lookup(clientmodel.LabelName) (fps clientmodel.Fingerprints, ok bool, err error)
	Has(clientmodel.LabelName) (ok bool, err error)
	State() *raw.DatabaseState
	Size() (s uint64, present bool, err error)
}

type LevelDBLabelNameFingerprintIndex struct {
	p *leveldb.LevelDBPersistence
}

func (i *LevelDBLabelNameFingerprintIndex) IndexBatch(b LabelNameFingerprintMapping) error {
	batch := leveldb.NewBatch()
	defer batch.Close()

	for labelName, fingerprints := range b {
		sort.Sort(fingerprints)

		key := &dto.LabelName{
			Name: proto.String(string(labelName)),
		}
		value := new(dto.FingerprintCollection)
		for _, fingerprint := range fingerprints {
			f := new(dto.Fingerprint)
			dumpFingerprint(f, fingerprint)
			value.Member = append(value.Member, f)
		}

		batch.Put(key, value)
	}

	return i.p.Commit(batch)
}

func (i *LevelDBLabelNameFingerprintIndex) Lookup(l clientmodel.LabelName) (fps clientmodel.Fingerprints, ok bool, err error) {
	k := new(dto.LabelName)
	dumpLabelName(k, l)
	v := new(dto.FingerprintCollection)
	ok, err = i.p.Get(k, v)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	for _, m := range v.Member {
		fp := new(clientmodel.Fingerprint)
		loadFingerprint(fp, m)
		fps = append(fps, fp)
	}

	return fps, true, nil
}

func (i *LevelDBLabelNameFingerprintIndex) Has(l clientmodel.LabelName) (ok bool, err error) {
	return i.p.Has(&dto.LabelName{
		Name: proto.String(string(l)),
	})
}

func (i *LevelDBLabelNameFingerprintIndex) Prune() (bool, error) {
	i.p.Prune()

	return false, nil
}

func (i *LevelDBLabelNameFingerprintIndex) Close() {
	i.p.Close()
}

func (i *LevelDBLabelNameFingerprintIndex) Size() (uint64, bool, error) {
	s, err := i.p.Size()
	return s, true, err
}

func (i *LevelDBLabelNameFingerprintIndex) State() *raw.DatabaseState {
	return i.p.State()
}

type LevelDBLabelNameFingerprintIndexOptions struct {
	leveldb.LevelDBOptions
}

func NewLevelLabelNameFingerprintIndex(o *LevelDBLabelNameFingerprintIndexOptions) (*LevelDBLabelNameFingerprintIndex, error) {
	s, err := leveldb.NewLevelDBPersistence(&o.LevelDBOptions)
	if err != nil {
		return nil, err
	}

	return &LevelDBLabelNameFingerprintIndex{
		p: s,
	}, nil
}

type LabelSetFingerprintMapping map[LabelPair]clientmodel.Fingerprints

type LabelSetFingerprintIndex interface {
	raw.ForEacher
	raw.Pruner

	IndexBatch(LabelSetFingerprintMapping) error
	Lookup(*LabelPair) (m clientmodel.Fingerprints, ok bool, err error)
	Has(*LabelPair) (ok bool, err error)
	State() *raw.DatabaseState
	Size() (s uint64, present bool, err error)
}

type LevelDBLabelSetFingerprintIndex struct {
	p *leveldb.LevelDBPersistence
}

type LevelDBLabelSetFingerprintIndexOptions struct {
	leveldb.LevelDBOptions
}

func (i *LevelDBLabelSetFingerprintIndex) IndexBatch(m LabelSetFingerprintMapping) error {
	batch := leveldb.NewBatch()
	defer batch.Close()

	for pair, fps := range m {
		sort.Sort(fps)

		key := &dto.LabelPair{
			Name:  proto.String(string(pair.Name)),
			Value: proto.String(string(pair.Value)),
		}
		value := new(dto.FingerprintCollection)
		for _, fp := range fps {
			f := new(dto.Fingerprint)
			dumpFingerprint(f, fp)
			value.Member = append(value.Member, f)
		}

		batch.Put(key, value)
	}

	return i.p.Commit(batch)
}

func (i *LevelDBLabelSetFingerprintIndex) Lookup(p *LabelPair) (m clientmodel.Fingerprints, ok bool, err error) {
	k := &dto.LabelPair{
		Name:  proto.String(string(p.Name)),
		Value: proto.String(string(p.Value)),
	}
	v := new(dto.FingerprintCollection)

	ok, err = i.p.Get(k, v)

	if !ok {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	for _, pair := range v.Member {
		fp := new(clientmodel.Fingerprint)
		loadFingerprint(fp, pair)
		m = append(m, fp)
	}

	return m, true, nil
}

func (i *LevelDBLabelSetFingerprintIndex) Has(p *LabelPair) (ok bool, err error) {
	k := &dto.LabelPair{
		Name:  proto.String(string(p.Name)),
		Value: proto.String(string(p.Value)),
	}

	return i.p.Has(k)
}

func (i *LevelDBLabelSetFingerprintIndex) ForEach(d storage.RecordDecoder, f storage.RecordFilter, o storage.RecordOperator) (bool, error) {
	return i.p.ForEach(d, f, o)
}

func (i *LevelDBLabelSetFingerprintIndex) Prune() (bool, error) {
	i.p.Prune()
	return false, nil
}

func (i *LevelDBLabelSetFingerprintIndex) Close() {
	i.p.Close()
}

func (i *LevelDBLabelSetFingerprintIndex) Size() (uint64, bool, error) {
	s, err := i.p.Size()
	return s, true, err
}

func (i *LevelDBLabelSetFingerprintIndex) State() *raw.DatabaseState {
	return i.p.State()
}

func NewLevelDBLabelSetFingerprintIndex(o *LevelDBLabelSetFingerprintIndexOptions) (*LevelDBLabelSetFingerprintIndex, error) {
	s, err := leveldb.NewLevelDBPersistence(&o.LevelDBOptions)
	if err != nil {
		return nil, err
	}

	return &LevelDBLabelSetFingerprintIndex{
		p: s,
	}, nil
}

type MetricMembershipIndex interface {
	raw.Pruner

	IndexBatch([]clientmodel.Metric) error
	Has(clientmodel.Metric) (ok bool, err error)
	State() *raw.DatabaseState
	Size() (s uint64, present bool, err error)
}

type LevelDBMetricMembershipIndex struct {
	p *leveldb.LevelDBPersistence
}

var existenceIdentity = new(dto.MembershipIndexValue)

func (i *LevelDBMetricMembershipIndex) IndexBatch(ms []clientmodel.Metric) error {
	batch := leveldb.NewBatch()
	defer batch.Close()

	for _, m := range ms {
		k := new(dto.Metric)
		dumpMetric(k, m)
		batch.Put(k, existenceIdentity)
	}

	return i.p.Commit(batch)
}

func (i *LevelDBMetricMembershipIndex) Has(m clientmodel.Metric) (ok bool, err error) {
	k := new(dto.Metric)
	dumpMetric(k, m)

	return i.p.Has(k)
}

func (i *LevelDBMetricMembershipIndex) Close() {
	i.p.Close()
}

func (i *LevelDBMetricMembershipIndex) Size() (uint64, bool, error) {
	s, err := i.p.Size()
	return s, true, err
}

func (i *LevelDBMetricMembershipIndex) State() *raw.DatabaseState {
	return i.p.State()
}

func (i *LevelDBMetricMembershipIndex) Prune() (bool, error) {
	i.p.Prune()

	return false, nil
}

type LevelDBMetricMembershipIndexOptions struct {
	leveldb.LevelDBOptions
}

func NewLevelDBMetricMembershipIndex(o *LevelDBMetricMembershipIndexOptions) (*LevelDBMetricMembershipIndex, error) {
	s, err := leveldb.NewLevelDBPersistence(&o.LevelDBOptions)
	if err != nil {
		return nil, err
	}

	return &LevelDBMetricMembershipIndex{
		p: s,
	}, nil
}