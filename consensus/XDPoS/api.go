// Copyright (c) 2018 XDPoSChain
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package XDPoS

import (
	"encoding/base64"
	"math/big"

	"github.com/XinFinOrg/XDC-Subnet/common"
	"github.com/XinFinOrg/XDC-Subnet/consensus"
	"github.com/XinFinOrg/XDC-Subnet/consensus/XDPoS/utils"
	"github.com/XinFinOrg/XDC-Subnet/core/types"
	"github.com/XinFinOrg/XDC-Subnet/rlp"
	"github.com/XinFinOrg/XDC-Subnet/rpc"
)

// API is a user facing RPC API to allow controlling the signer and voting
// mechanisms of the proof-of-authority scheme.
type API struct {
	chain consensus.ChainReader
	XDPoS *XDPoS
}

type V2BlockInfo struct {
	Hash       common.Hash
	Round      types.Round
	Number     *big.Int
	ParentHash common.Hash
	Committed  bool
	Miner      common.Hash
	Timestamp  *big.Int
	EncodedRLP string
	Error      string
}

type NetworkInformation struct {
	NetworkId                  *big.Int
	XDCValidatorAddress        common.Address
	RelayerRegistrationAddress common.Address
	Denom                      string
	NetworkName                string
}

type SignerTypes struct {
	CurrentNumber  int
	CurrentSigners []common.Address
	MissingSigners []common.Address
}

type MasternodesStatus struct {
	Number          uint64
	Round           types.Round
	MasternodesLen  int
	Masternodes     []common.Address
	PenaltyLen      int
	Penalty         []common.Address
	StandbynodesLen int
	Standbynodes    []common.Address
	Error           error
}

type MessageStatus map[string]map[string]SignerTypes

// GetSnapshot retrieves the state snapshot at a given block.
func (api *API) GetSnapshot(number *rpc.BlockNumber) (*utils.PublicApiSnapshot, error) {
	// Retrieve the requested block number (or current if none requested)
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// Ensure we have an actually valid block and return its snapshot
	if header == nil {
		return nil, utils.ErrUnknownBlock
	}
	return api.XDPoS.GetSnapshot(api.chain, header)
}

// GetSnapshotAtHash retrieves the state snapshot at a given block.
func (api *API) GetSnapshotAtHash(hash common.Hash) (*utils.PublicApiSnapshot, error) {
	header := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, utils.ErrUnknownBlock
	}
	return api.XDPoS.GetSnapshot(api.chain, header)
}

// GetSigners retrieves the list of authorized signers at the specified block.
func (api *API) GetSigners(number *rpc.BlockNumber) ([]common.Address, error) {
	// Retrieve the requested block number (or current if none requested)
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// Ensure we have an actually valid block and return the signers from its snapshot
	if header == nil {
		return nil, utils.ErrUnknownBlock
	}

	return api.XDPoS.GetAuthorisedSignersFromSnapshot(api.chain, header)
}

// GetSignersAtHash retrieves the state snapshot at a given block.
func (api *API) GetSignersAtHash(hash common.Hash) ([]common.Address, error) {
	header := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, utils.ErrUnknownBlock
	}
	return api.XDPoS.GetAuthorisedSignersFromSnapshot(api.chain, header)
}

func (api *API) GetMasternodesByNumber(number *rpc.BlockNumber) MasternodesStatus {
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else if *number == rpc.CommittedBlockNumber {
		hash := api.XDPoS.EngineV2.GetLatestCommittedBlockInfo().Hash
		header = api.chain.GetHeaderByHash(hash)
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}

	round, err := api.XDPoS.EngineV2.GetRoundNumber(header)
	if err != nil {
		return MasternodesStatus{
			Error: err,
		}
	}

	masterNodes := api.XDPoS.EngineV2.GetMasternodes(api.chain, header)
	penalties := api.XDPoS.EngineV2.GetPenalties(api.chain, header)
	standbynodes := api.XDPoS.EngineV2.GetStandbynodes(api.chain, header)

	info := MasternodesStatus{
		Number:          header.Number.Uint64(),
		Round:           round,
		MasternodesLen:  len(masterNodes),
		Masternodes:     masterNodes,
		PenaltyLen:      len(penalties),
		Penalty:         penalties,
		StandbynodesLen: len(standbynodes),
		Standbynodes:    standbynodes,
	}
	return info
}

// Get current vote pool and timeout pool content and missing messages
func (api *API) GetLatestPoolStatus() MessageStatus {
	header := api.chain.CurrentHeader()
	masternodes := api.XDPoS.EngineV2.GetMasternodes(api.chain, header)

	receivedVotes := api.XDPoS.EngineV2.ReceivedVotes()
	receivedTimeouts := api.XDPoS.EngineV2.ReceivedTimeouts()
	info := make(MessageStatus)
	info["vote"] = make(map[string]SignerTypes)
	info["timeout"] = make(map[string]SignerTypes)

	calculateSigners(info["vote"], receivedVotes, masternodes)
	calculateSigners(info["timeout"], receivedTimeouts, masternodes)

	return info
}

func (api *API) GetV2BlockByHeader(header *types.Header, uncle bool) *V2BlockInfo {
	committed := false
	latestCommittedBlock := api.XDPoS.EngineV2.GetLatestCommittedBlockInfo()
	if latestCommittedBlock == nil {
		return &V2BlockInfo{
			Hash:  header.Hash(),
			Error: "can not find latest committed block from consensus",
		}
	}
	if header.Number.Uint64() <= latestCommittedBlock.Number.Uint64() {
		committed = true && !uncle
	}

	round, err := api.XDPoS.EngineV2.GetRoundNumber(header)

	if err != nil {
		return &V2BlockInfo{
			Hash:  header.Hash(),
			Error: err.Error(),
		}
	}

	encodeBytes, err := rlp.EncodeToBytes(header)
	if err != nil {
		return &V2BlockInfo{
			Hash:  header.Hash(),
			Error: err.Error(),
		}
	}

	block := &V2BlockInfo{
		Hash:       header.Hash(),
		ParentHash: header.ParentHash,
		Number:     header.Number,
		Round:      round,
		Committed:  committed,
		Miner:      header.Coinbase.Hash(),
		Timestamp:  header.Time,
		EncodedRLP: base64.StdEncoding.EncodeToString(encodeBytes),
	}
	return block
}

func (api *API) GetV2BlockByNumber(number *rpc.BlockNumber) *V2BlockInfo {
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else if *number == rpc.CommittedBlockNumber {
		latestCommittedBlock := api.XDPoS.EngineV2.GetLatestCommittedBlockInfo()
		if latestCommittedBlock == nil {
			return &V2BlockInfo{
				Hash:  header.Hash(),
				Error: "can not find latest committed block from consensus",
			}
		}
		header = api.chain.GetHeaderByHash(latestCommittedBlock.Hash)
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}

	if header == nil {
		return &V2BlockInfo{
			Number: big.NewInt(number.Int64()),
			Error:  "can not find block from this number",
		}
	}

	uncle := false
	return api.GetV2BlockByHeader(header, uncle)
}

// Confirm V2 Block Committed Status
func (api *API) GetV2BlockByHash(blockHash common.Hash) *V2BlockInfo {
	header := api.chain.GetHeaderByHash(blockHash)
	if header == nil {
		return &V2BlockInfo{
			Hash:  blockHash,
			Error: "can not find block from this hash",
		}
	}

	// confirm this is on the main chain
	chainHeader := api.chain.GetHeaderByNumber(header.Number.Uint64())
	uncle := false
	if header.Hash() != chainHeader.Hash() {
		uncle = true
	}

	return api.GetV2BlockByHeader(header, uncle)
}

func (api *API) NetworkInformation() NetworkInformation {
	info := NetworkInformation{}
	info.NetworkId = api.chain.Config().ChainId
	info.XDCValidatorAddress = common.HexToAddress(common.MasternodeVotingSMC)
	info.Denom = api.chain.Config().XDPoS.Denom
	info.NetworkName = api.chain.Config().XDPoS.NetworkName
	return info
}

func calculateSigners(message map[string]SignerTypes, pool map[string]map[common.Hash]utils.PoolObj, masternodes []common.Address) {
	for name, objs := range pool {
		var currentSigners []common.Address
		missingSigners := make([]common.Address, len(masternodes))
		copy(missingSigners, masternodes)

		num := len(objs)
		for _, obj := range objs {
			signer := obj.GetSigner()
			currentSigners = append(currentSigners, signer)
			for i, mn := range missingSigners {
				if mn == signer {
					missingSigners = append(missingSigners[:i], missingSigners[i+1:]...)
					break
				}
			}
		}
		message[name] = SignerTypes{
			CurrentNumber:  num,
			CurrentSigners: currentSigners,
			MissingSigners: missingSigners,
		}
	}
}
