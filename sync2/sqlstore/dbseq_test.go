package sqlstore_test

import (
	"encoding/hex"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spacemeshos/go-spacemesh/sync2/rangesync"
	"github.com/spacemeshos/go-spacemesh/sync2/sqlstore"
)

func TestDBRangeIterator(t *testing.T) {
	for _, tc := range []struct {
		items []rangesync.KeyBytes
		from  rangesync.KeyBytes
		fromN int
	}{
		{
			items: nil,
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x00},
		},
		{
			items: nil,
			from:  rangesync.KeyBytes{0x80, 0x00, 0x00, 0x00},
		},
		{
			items: nil,
			from:  rangesync.KeyBytes{0xff, 0xff, 0xff, 0xff},
		},
		{
			items: []rangesync.KeyBytes{
				{0x00, 0x00, 0x00, 0x00},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x00},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				{0x00, 0x00, 0x00, 0x00},
			},
			from:  rangesync.KeyBytes{0x01, 0x00, 0x00, 0x00},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				{0x00, 0x00, 0x00, 0x00},
			},
			from:  rangesync.KeyBytes{0xff, 0xff, 0xff, 0xff},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				{0x01, 0x02, 0x03, 0x04},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x00},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				{0x01, 0x02, 0x03, 0x04},
			},
			from:  rangesync.KeyBytes{0x01, 0x00, 0x00, 0x00},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				{0x01, 0x02, 0x03, 0x04},
			},
			from:  rangesync.KeyBytes{0xff, 0xff, 0xff, 0xff},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				{0xff, 0xff, 0xff, 0xff},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x00},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				{0xff, 0xff, 0xff, 0xff},
			},
			from:  rangesync.KeyBytes{0x01, 0x00, 0x00, 0x00},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				{0xff, 0xff, 0xff, 0xff},
			},
			from:  rangesync.KeyBytes{0xff, 0xff, 0xff, 0xff},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				0: {0x00, 0x00, 0x00, 0x01},
				1: {0x00, 0x00, 0x00, 0x03},
				2: {0x00, 0x00, 0x00, 0x05},
				3: {0x00, 0x00, 0x00, 0x07},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x00},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				0: {0x00, 0x00, 0x00, 0x01},
				1: {0x00, 0x00, 0x00, 0x03},
				2: {0x00, 0x00, 0x00, 0x05},
				3: {0x00, 0x00, 0x00, 0x07},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x01},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				0: {0x00, 0x00, 0x00, 0x01},
				1: {0x00, 0x00, 0x00, 0x03},
				2: {0x00, 0x00, 0x00, 0x05},
				3: {0x00, 0x00, 0x00, 0x07},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x02},
			fromN: 1,
		},
		{
			items: []rangesync.KeyBytes{
				0: {0x00, 0x00, 0x00, 0x01},
				1: {0x00, 0x00, 0x00, 0x03},
				2: {0x00, 0x00, 0x00, 0x05},
				3: {0x00, 0x00, 0x00, 0x07},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x03},
			fromN: 1,
		},
		{
			items: []rangesync.KeyBytes{
				0: {0x00, 0x00, 0x00, 0x01},
				1: {0x00, 0x00, 0x00, 0x03},
				2: {0x00, 0x00, 0x00, 0x05},
				3: {0x00, 0x00, 0x00, 0x07},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x05},
			fromN: 2,
		},
		{
			items: []rangesync.KeyBytes{
				0: {0x00, 0x00, 0x00, 0x01},
				1: {0x00, 0x00, 0x00, 0x03},
				2: {0x00, 0x00, 0x00, 0x05},
				3: {0x00, 0x00, 0x00, 0x07},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x00, 0x07},
			fromN: 3,
		},
		{
			items: []rangesync.KeyBytes{
				0: {0x00, 0x00, 0x00, 0x01},
				1: {0x00, 0x00, 0x00, 0x03},
				2: {0x00, 0x00, 0x00, 0x05},
				3: {0x00, 0x00, 0x00, 0x07},
			},
			from:  rangesync.KeyBytes{0xff, 0xff, 0xff, 0xff},
			fromN: 0,
		},
		{
			items: []rangesync.KeyBytes{
				0:  {0x00, 0x00, 0x00, 0x01},
				1:  {0x00, 0x00, 0x00, 0x03},
				2:  {0x00, 0x00, 0x00, 0x05},
				3:  {0x00, 0x00, 0x00, 0x07},
				4:  {0x00, 0x00, 0x01, 0x00},
				5:  {0x00, 0x00, 0x03, 0x00},
				6:  {0x00, 0x01, 0x00, 0x00},
				7:  {0x00, 0x05, 0x00, 0x00},
				8:  {0x03, 0x05, 0x00, 0x00},
				9:  {0x09, 0x05, 0x00, 0x00},
				10: {0x0a, 0x05, 0x00, 0x00},
				11: {0xff, 0xff, 0xff, 0xff},
			},
			from:  rangesync.KeyBytes{0x00, 0x00, 0x03, 0x01},
			fromN: 6,
		},
		{
			items: []rangesync.KeyBytes{
				0:  {0x00, 0x00, 0x00, 0x01},
				1:  {0x00, 0x00, 0x00, 0x03},
				2:  {0x00, 0x00, 0x00, 0x05},
				3:  {0x00, 0x00, 0x00, 0x07},
				4:  {0x00, 0x00, 0x01, 0x00},
				5:  {0x00, 0x00, 0x03, 0x00},
				6:  {0x00, 0x01, 0x00, 0x00},
				7:  {0x00, 0x05, 0x00, 0x00},
				8:  {0x03, 0x05, 0x00, 0x00},
				9:  {0x09, 0x05, 0x00, 0x00},
				10: {0x0a, 0x05, 0x00, 0x00},
				11: {0xff, 0xff, 0xff, 0xff},
			},
			from:  rangesync.KeyBytes{0x00, 0x01, 0x00, 0x00},
			fromN: 6,
		},
		{
			items: []rangesync.KeyBytes{
				0:  {0x00, 0x00, 0x00, 0x01},
				1:  {0x00, 0x00, 0x00, 0x03},
				2:  {0x00, 0x00, 0x00, 0x05},
				3:  {0x00, 0x00, 0x00, 0x07},
				4:  {0x00, 0x00, 0x01, 0x00},
				5:  {0x00, 0x00, 0x03, 0x00},
				6:  {0x00, 0x01, 0x00, 0x00},
				7:  {0x00, 0x05, 0x00, 0x00},
				8:  {0x03, 0x05, 0x00, 0x00},
				9:  {0x09, 0x05, 0x00, 0x00},
				10: {0x0a, 0x05, 0x00, 0x00},
				11: {0xff, 0xff, 0xff, 0xff},
			},
			from:  rangesync.KeyBytes{0xff, 0xff, 0xff, 0xff},
			fromN: 11,
		},
	} {
		t.Run("", func(t *testing.T) {
			db := sqlstore.CreateDB(t, 4)
			sqlstore.InsertDBItems(t, db, tc.items)
			st := &sqlstore.SyncedTable{
				TableName: "foo",
				IDColumn:  "id",
			}
			sts, err := st.Snapshot(db)
			require.NoError(t, err)
			for startChunkSize := 1; startChunkSize < 12; startChunkSize++ {
				for maxChunkSize := 1; maxChunkSize < 12; maxChunkSize++ {
					sr := sqlstore.IDSFromTable(db, sts, tc.from, -1,
						startChunkSize, maxChunkSize)
					// when there are no items, errEmptySet is returned
					for range 3 { // make sure the sequence is reusable
						var collected []rangesync.KeyBytes
						var firstK rangesync.KeyBytes
						for k := range sr.Seq {
							if firstK == nil {
								firstK = k
							} else if k.Compare(firstK) == 0 {
								break
							}
							collected = append(collected, k)
							require.NoError(t, err)
						}
						require.NoError(t, sr.Error())
						expected := slices.Concat(
							tc.items[tc.fromN:], tc.items[:tc.fromN])
						require.Equal(
							t, expected, collected,
							"count=%d from=%s maxChunkSize=%d",
							len(tc.items), hex.EncodeToString(tc.from),
							maxChunkSize)
					}
				}
			}
		})
	}
}
