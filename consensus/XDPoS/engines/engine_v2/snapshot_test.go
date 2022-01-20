package engine_v2

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/XinFinOrg/XDPoSChain/common"
	"github.com/XinFinOrg/XDPoSChain/consensus/XDPoS/utils"
	"github.com/XinFinOrg/XDPoSChain/core/rawdb"
	"github.com/XinFinOrg/XDPoSChain/ethdb/leveldb"
)

func TestGetMasterNodes(t *testing.T) {
	masterNodes := []common.Address{{0x4}, {0x3}, {0x2}, {0x1}}
	snap := newSnapshot(1, common.Hash{}, utils.Round(1), nil, masterNodes)

	for _, address := range masterNodes {
		if _, ok := snap.GetMappedMasterNodes()[address]; !ok {
			t.Error("should get master node from map", address.Hex(), snap.GetMappedMasterNodes())
			return
		}
	}
}

func TestStoreLoadSnapshot(t *testing.T) {
	snap := newSnapshot(1, common.Hash{0x1}, utils.Round(1), nil, nil)
	dir, err := ioutil.TempDir("", "snapshot-test")
	if err != nil {
		panic(fmt.Sprintf("can't create temporary directory: %v", err))
	}
	db, err := leveldb.New(dir, 256, 0, "")
	if err != nil {
		panic(fmt.Sprintf("can't create temporary database: %v", err))
	}
	lddb := rawdb.NewDatabase(db)

	err = storeSnapshot(snap, lddb)
	if err != nil {
		t.Error("store snapshot failed", err)
	}

	restoredSnapshot, err := loadSnapshot(nil, lddb, snap.Hash)
	if err != nil || restoredSnapshot.Hash != snap.Hash {
		t.Error("load snapshot failed", err)
	}
}
