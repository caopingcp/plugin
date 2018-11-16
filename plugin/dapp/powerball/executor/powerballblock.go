// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	"context"
	"time"

	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
	tickettypes "github.com/33cn/plugin/plugin/dapp/ticket/types"
)

const retryNum = 10

//different impl on main chain and parachain
func (action *Action) getTxActions(height int64, blockNum int64) ([]*tickettypes.TicketAction, error) {
	var txActions []*tickettypes.TicketAction
	pblog.Error("getTxActions", "height", height, "blockNum", blockNum)
	if !types.IsPara() {
		req := &types.ReqBlocks{height - blockNum + 1, height, false, []string{""}}

		blockDetails, err := action.api.GetBlocks(req)
		if err != nil {
			pblog.Error("getTxActions", "height", height, "blockNum", blockNum, "err", err)
			return txActions, err
		}
		for _, block := range blockDetails.Items {
			pblog.Debug("getTxActions", "blockHeight", block.Block.Height, "blockhash", block.Block.Hash())
			ticketAction, err := action.getMinerTx(block.Block)
			if err != nil {
				return txActions, err
			}
			txActions = append(txActions, ticketAction)
		}
		return txActions, nil
	} else {
		//block height on main
		mainHeight := action.GetMainHeightByTxHash(action.txhash)
		if mainHeight < 0 {
			pblog.Error("PowerballCreate", "mainHeight", mainHeight)
			return nil, pty.ErrPowerballStatus
		}

		blockDetails, err := action.GetBlocksOnMain(mainHeight-blockNum, mainHeight-1)
		if err != nil {
			pblog.Error("PowerballCreate", "mainHeight", mainHeight)
			return nil, pty.ErrPowerballStatus
		}

		for _, block := range blockDetails.Items {
			ticketAction, err := action.getMinerTx(block.Block)
			if err != nil {
				return txActions, err
			}
			txActions = append(txActions, ticketAction)
		}
		return txActions, nil
	}
}

//TransactionDetail
func (action *Action) GetMainHeightByTxHash(txHash []byte) int64 {
	for i := 0; i < retryNum; i++ {
		req := &types.ReqHash{txHash}
		txDetail, err := action.grpcClient.QueryTransaction(context.Background(), req)
		if err != nil {
			time.Sleep(time.Second)
		} else {
			return txDetail.GetHeight()
		}
	}

	return -1
}

func (action *Action) GetBlocksOnMain(start int64, end int64) (*types.BlockDetails, error) {
	req := &types.ReqBlocks{start, end, false, []string{""}}
	getBlockSucc := false
	var reply *types.Reply
	var err error

	for i := 0; i < retryNum; i++ {
		reply, err = action.grpcClient.GetBlocks(context.Background(), req)
		if err != nil {
			pblog.Error("GetBlocksOnMain", "start", start, "end", end, "err", err)
			time.Sleep(time.Second)
		} else {
			getBlockSucc = true
			break
		}
	}

	if !getBlockSucc {
		return nil, err
	}

	var blockDetails types.BlockDetails

	err = types.Decode(reply.Msg, &blockDetails)
	if err != nil {
		pblog.Error("GetBlocksOnMain", "err", err)
		return nil, err
	}

	return &blockDetails, nil
}

func (action *Action) getMinerTx(current *types.Block) (*tickettypes.TicketAction, error) {
	//检查第一个笔交易的execs, 以及执行状态
	if len(current.Txs) == 0 {
		return nil, types.ErrEmptyTx
	}
	baseTx := current.Txs[0]
	//判断交易类型和执行情况
	var ticketAction tickettypes.TicketAction
	err := types.Decode(baseTx.GetPayload(), &ticketAction)
	if err != nil {
		return nil, err
	}
	if ticketAction.GetTy() != tickettypes.TicketActionMiner {
		return nil, types.ErrCoinBaseTxType
	}
	//判断交易执行是否OK
	if ticketAction.GetMiner() == nil {
		return nil, tickettypes.ErrEmptyMinerTx
	}
	return &ticketAction, nil
}
