// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
)

func (l *Powerball) Query_GetPowerballNormalInfo(param *pty.ReqPowerballInfo) (types.Message, error) {
	powerball, err := findPowerball(l.GetStateDB(), param.GetPowerballId())
	if err != nil {
		return nil, err
	}
	return &pty.ReplyPowerballNormalInfo{
		powerball.CreateHeight,
		powerball.PurTime,
		powerball.DrawTime,
		powerball.TicketPrice,
		powerball.CreateAddr}, nil
}

func (l *Powerball) Query_GetPowerballPurchaseAddr(param *pty.ReqPowerballInfo) (types.Message, error) {
	powerball, err := findPowerball(l.GetStateDB(), param.GetPowerballId())
	if err != nil {
		return nil, err
	}
	reply := &pty.ReplyPowerballPurchaseAddr{}
	for _, info := range powerball.PurInfos {
		reply.Address = append(reply.Address, info.Addr)
	}
	//powerball.Records
	return reply, nil
}

func (l *Powerball) Query_GetPowerballCurrentInfo(param *pty.ReqPowerballInfo) (types.Message, error) {
	powerball, err := findPowerball(l.GetStateDB(), param.GetPowerballId())
	if err != nil {
		return nil, err
	}
	reply := &pty.ReplyPowerballCurrentInfo{
		Status:                     powerball.Status,
		TotalFund:                  powerball.TotalFund,
		SaleFund:                   powerball.SaleFund,
		LastTransToPurState:        powerball.LastTransToPurState,
		LastTransToDrawState:       powerball.LastTransToDrawState,
		TotalPurchasedTxNum:        powerball.TotalPurchasedTxNum,
		Round:                      powerball.Round,
		LuckyNumber:                powerball.LuckyNumber,
		LastTransToPurStateOnMain:  powerball.LastTransToPurStateOnMain,
		LastTransToDrawStateOnMain: powerball.LastTransToDrawStateOnMain,
		PurTime:                    powerball.PurTime,
		DrawTime:                   powerball.DrawTime,
		MissingRecords:             powerball.MissingRecords,
	}
	return reply, nil
}

func (l *Powerball) Query_GetPowerballHistoryLuckyNumber(param *pty.ReqPowerballLuckyHistory) (types.Message, error) {
	return ListPowerballLuckyHistory(l.GetLocalDB(), l.GetStateDB(), param)
}

func (l *Powerball) Query_GetPowerballRoundLuckyNumber(param *pty.ReqPowerballLuckyInfo) (types.Message, error) {
	//	var req pty.ReqPowerballLuckyInfo
	var records []*pty.PowerballDrawRecord
	//	err := types.Decode(param, &req)
	//if err != nil {
	//	return nil, err
	//}
	for _, round := range param.Round {
		key := calcPowerballDrawKey(param.PowerballId, round)
		record, err := l.findPowerballDrawRecord(key)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return &pty.PowerballDrawRecords{Records: records}, nil
}

func (l *Powerball) Query_GetPowerballHistoryBuyInfo(param *pty.ReqPowerballBuyHistory) (types.Message, error) {
	return ListPowerballBuyRecords(l.GetLocalDB(), l.GetStateDB(), param)
}

func (l *Powerball) Query_GetPowerballBuyRoundInfo(param *pty.ReqPowerballBuyInfo) (types.Message, error) {
	key := calcPowerballBuyRoundPrefix(param.PowerballId, param.Addr, param.Round)
	record, err := l.findPowerballBuyRecords(key)
	if err != nil {
		return nil, err
	}
	return record, nil
}
