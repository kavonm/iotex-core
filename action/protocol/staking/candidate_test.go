// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestClone(t *testing.T) {
	r := require.New(t)
	d := &Candidate{
		Owner:              identityset.Address(1),
		Operator:           identityset.Address(2),
		Reward:             identityset.Address(3),
		Name:               "testname1234",
		Votes:              big.NewInt(0),
		SelfStakeBucketIdx: 0,
		SelfStake:          big.NewInt(2100000000),
	}
	d2 := d.Clone()
	r.Equal(d, d2)
	d.AddVote(big.NewInt(100))
	r.NotEqual(d, d2)

	c := d.toStateCandidate()
	r.Equal(d.Owner.String(), c.Address)
	r.Equal(d.Reward.String(), c.RewardAddress)
	r.Equal(d.Votes, c.Votes)
	r.Equal(d.Name, string(c.CanName))
}

var (
	testCandidates = []struct {
		d     *Candidate
		index int
	}{
		{
			&Candidate{
				Owner:              identityset.Address(1),
				Operator:           identityset.Address(11),
				Reward:             identityset.Address(1),
				Name:               "test1",
				Votes:              big.NewInt(2),
				SelfStakeBucketIdx: 1,
				SelfStake:          unit.ConvertIotxToRau(1200000),
			},
			2,
		},
		{
			&Candidate{
				Owner:              identityset.Address(2),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(1),
				Name:               "test2",
				Votes:              big.NewInt(3),
				SelfStakeBucketIdx: 2,
				SelfStake:          unit.ConvertIotxToRau(1200000),
			},
			1,
		},
		{
			&Candidate{
				Owner:              identityset.Address(3),
				Operator:           identityset.Address(13),
				Reward:             identityset.Address(1),
				Name:               "test3",
				Votes:              big.NewInt(3),
				SelfStakeBucketIdx: 3,
				SelfStake:          unit.ConvertIotxToRau(1200000),
			},
			0,
		},
		{
			&Candidate{
				Owner:              identityset.Address(4),
				Operator:           identityset.Address(14),
				Reward:             identityset.Address(1),
				Name:               "test4",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 4,
				SelfStake:          unit.ConvertIotxToRau(1200000),
			},
			3,
		},
		{
			&Candidate{
				Owner:              identityset.Address(5),
				Operator:           identityset.Address(15),
				Reward:             identityset.Address(2),
				Name:               "test5",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 5,
				SelfStake:          unit.ConvertIotxToRau(1199999),
			},
			5,
		},
		{
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(16),
				Reward:             identityset.Address(2),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 6,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
			6,
		},
	}
)

func TestCandCenter(t *testing.T) {
	r := require.New(t)

	m := NewCandidateCenter()
	for i, v := range testCandidates {
		r.NoError(m.Upsert(testCandidates[i].d))
		r.True(m.ContainsName(v.d.Name))
		r.Equal(v.d, m.GetByName(v.d.Name))
	}
	r.Equal(len(testCandidates), m.Size())

	// test candidate that does not exist
	noName := identityset.Address(22)
	r.False(m.ContainsOwner(noName))
	m.Delete(noName)
	r.Equal(len(testCandidates), m.Size())

	// test existence
	for _, v := range testCandidates {
		r.True(m.ContainsName(v.d.Name))
		r.True(m.ContainsOwner(v.d.Owner))
		r.True(m.ContainsOperator(v.d.Operator))
		r.True(m.ContainsSelfStakingBucket(v.d.SelfStakeBucketIdx))
		r.Equal(v.d, m.GetByName(v.d.Name))
	}

	// test convert to list
	list, err := m.All()
	r.NoError(err)
	r.Equal(m.Size(), len(list))
	for _, v := range m.ownerMap {
		for i := range list {
			if list[i].Name == v.Name {
				r.Equal(v, list[i])
				break
			}
		}
	}

	// cannot insert candidate with conflicting name/operator/self-staking index
	old := testCandidates[0].d
	conflict := m.GetByName(old.Name)
	r.NotNil(conflict)
	conflict.Owner = identityset.Address(24)
	r.Equal(ErrInvalidCanName, m.Upsert(conflict))
	conflict.Name = "xxx"
	r.Equal(ErrInvalidOperator, m.Upsert(conflict))
	conflict.Operator = identityset.Address(24)
	r.Equal(ErrInvalidSelfStkIndex, m.Upsert(conflict))

	// test update candidate
	d := m.GetByName(old.Name)
	r.NotNil(d)
	d.Name = "xxx"
	d.Operator = identityset.Address(24)
	d.SelfStakeBucketIdx += 100
	r.NoError(m.Upsert(d))
	r.True(m.ContainsName(d.Name))
	r.True(m.ContainsOperator(d.Operator))
	r.True(m.ContainsSelfStakingBucket(d.SelfStakeBucketIdx))
	r.False(m.ContainsName(old.Name))
	r.False(m.ContainsOperator(old.Operator))
	r.False(m.ContainsSelfStakingBucket(old.SelfStakeBucketIdx))

	// test delete
	for i, v := range testCandidates {
		m.Delete(v.d.Owner)
		r.False(m.ContainsOwner(v.d.Owner))
		r.False(m.ContainsName(v.d.Name))
		r.False(m.ContainsOperator(v.d.Operator))
		r.False(m.ContainsSelfStakingBucket(v.d.SelfStakeBucketIdx))
		r.Equal(len(testCandidates)-i-1, m.Size())
	}
}

func TestGetPutCandidate(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)

	// put candidates and get
	for _, e := range testCandidates {
		_, err := getCandidate(sm, e.d.Owner)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
		require.NoError(putCandidate(sm, e.d))
		d1, err := getCandidate(sm, e.d.Owner)
		require.NoError(err)
		require.Equal(e.d, d1)
	}

	// delete buckets and get
	for _, e := range testCandidates {
		require.NoError(delCandidate(sm, e.d.Owner))
		_, err := getCandidate(sm, e.d.Owner)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
	}
}
