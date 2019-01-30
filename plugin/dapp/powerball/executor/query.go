// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
)

// Query_GetPowerballNormalInfo get powerball normal info
func (l *Powerball) Query_GetPowerballNormalInfo(param *pty.ReqPowerballInfo) (types.Message, error) {
	powerball, err := findPowerball(l.GetStateDB(), param.GetPowerballID())
	if err != nil {
		return nil, err
	}
	return &pty.ReplyPowerballNormalInfo{
		CreateHeight:  powerball.CreateHeight,
		PurTime:       powerball.PurTime,
		DrawTime:      powerball.DrawTime,
		TicketPrice:   powerball.TicketPrice,
		PlatformRatio: powerball.PlatformRatio,
		DevelopRatio:  powerball.DevelopRatio,
		CreateAddr:    powerball.CreateAddr}, nil
}

// Query_GetPowerballPurchaseAddr get purchase address for current round
func (l *Powerball) Query_GetPowerballPurchaseAddr(param *pty.ReqPowerballInfo) (types.Message, error) {
	powerball, err := findPowerball(l.GetStateDB(), param.GetPowerballID())
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

// Query_GetPowerballCurrentInfo get powerball current info
func (l *Powerball) Query_GetPowerballCurrentInfo(param *pty.ReqPowerballInfo) (types.Message, error) {
	powerball, err := findPowerball(l.GetStateDB(), param.GetPowerballID())
	if err != nil {
		return nil, err
	}
	reply := &pty.ReplyPowerballCurrentInfo{
		Status:                     powerball.Status,
		AccuFund:                   powerball.AccuFund,
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
		TotalAddrNum:               powerball.TotalAddrNum,
	}
	return reply, nil
}

// Query_GetPowerballHistoryLuckyNumber get history lucky number
func (l *Powerball) Query_GetPowerballHistoryLuckyNumber(param *pty.ReqPowerballLuckyHistory) (types.Message, error) {
	return ListPowerballLuckyHistory(l.GetLocalDB(), l.GetStateDB(), param)
}

// Query_GetPowerballRoundLuckyNumber get lucky number at some rounds
func (l *Powerball) Query_GetPowerballRoundLuckyNumber(param *pty.ReqPowerballLuckyInfo) (types.Message, error) {
	var records []*pty.PowerballDrawRecord
	for _, round := range param.Round {
		key := calcPowerballDrawKey(param.PowerballID, round)
		record, err := l.findPowerballDrawRecord(key)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return &pty.PowerballDrawRecords{Records: records}, nil
}

// Query_GetPowerballHistoryBuyInfo get history buy number
func (l *Powerball) Query_GetPowerballHistoryBuyInfo(param *pty.ReqPowerballBuyHistory) (types.Message, error) {
	return ListPowerballBuyRecords(l.GetLocalDB(), l.GetStateDB(), param)
}

// Query_GetPowerballBuyRoundInfo get buy number for each round
func (l *Powerball) Query_GetPowerballRoundBuyInfo(param *pty.ReqPowerballBuyInfo) (types.Message, error) {
	key := calcPowerballBuyRoundPrefix(param.PowerballID, param.Addr, param.Round)
	record, err := l.findPowerballBuyRecords(key)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// Query_GetPowerballHistoryGainInfo get history gain info
func (l *Powerball) Query_GetPowerballHistoryGainInfo(param *pty.ReqPowerballGainHistory) (types.Message, error) {
	return ListPowerballGainRecords(l.GetLocalDB(), l.GetStateDB(), param)
}

// Query_GetPowerballRoundGainInfo get gain info for each round
func (l *Powerball) Query_GetPowerballRoundGainInfo(param *pty.ReqPowerballGainInfo) (types.Message, error) {
	key := calcPowerballGainKey(param.PowerballID, param.Addr, param.Round)
	record, err := l.findPowerballGainRecord(key)
	if err != nil {
		return nil, err
	}
	return record, nil
}
