// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	log "github.com/33cn/chain33/common/log/log15"
	drivers "github.com/33cn/chain33/system/dapp"
	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
)

var pblog = log.New("module", "execs.powerball")
var driverName = pty.PowerballX

func init() {
	ety := types.LoadExecutorType(driverName)
	ety.InitFuncList(types.ListMethod(&Powerball{}))
}

type subConfig struct {
}

var cfg subConfig

// Init powerball
func Init(name string, sub []byte) {
	driverName := GetName()
	if name != driverName {
		panic("system dapp can't be rename")
	}
	if sub != nil {
		types.MustDecode(sub, &cfg)
	}
	drivers.Register(driverName, newPowerball, types.GetDappFork(driverName, "Enable"))
}

// GetName for powerball
func GetName() string {
	return newPowerball().GetName()
}

// Powerball driver
type Powerball struct {
	drivers.DriverBase
}

func newPowerball() drivers.Driver {
	p := &Powerball{}
	p.SetChild(p)
	p.SetExecutorType(types.LoadExecutorType(driverName))
	return p
}

// GetDriverName for powerball
func (ball *Powerball) GetDriverName() string {
	return pty.PowerballX
}

func (ball *Powerball) findPowerballBuyRecords(key []byte) (*pty.PowerballBuyRecords, error) {
	count := ball.GetLocalDB().PrefixCount(key)
	pblog.Info("findPowerballBuyRecords", "count", count)
	values, err := ball.GetLocalDB().List(key, nil, int32(count), 0)
	if err != nil {
		return nil, err
	}

	var records pty.PowerballBuyRecords
	for _, value := range values {
		var record pty.PowerballBuyRecord
		err := types.Decode(value, &record)
		if err != nil {
			continue
		}
		records.Records = append(records.Records, &record)
	}
	return &records, nil
}

func (ball *Powerball) findPowerballBuyRecord(key []byte) (*pty.PowerballBuyRecord, error) {
	value, err := ball.GetLocalDB().Get(key)
	if err != nil && err != types.ErrNotFound {
		pblog.Error("findPowerballBuyRecord", "err", err)
		return nil, err
	}
	if err == types.ErrNotFound {
		return nil, nil
	}

	var record pty.PowerballBuyRecord
	err = types.Decode(value, &record)
	if err != nil {
		pblog.Error("findPowerballBuyRecord", "err", err)
		return nil, err
	}
	return &record, nil
}

func (ball *Powerball) findPowerballDrawRecord(key []byte) (*pty.PowerballDrawRecord, error) {
	value, err := ball.GetLocalDB().Get(key)
	if err != nil && err != types.ErrNotFound {
		pblog.Error("findPowerballDrawRecord", "err", err)
		return nil, err
	}
	if err == types.ErrNotFound {
		return nil, nil
	}

	var record pty.PowerballDrawRecord
	err = types.Decode(value, &record)
	if err != nil {
		pblog.Error("findPowerballDrawRecord", "err", err)
		return nil, err
	}
	return &record, nil
}

func (ball *Powerball) findPowerballGainRecord(key []byte) (*pty.PowerballGainRecord, error) {
	value, err := ball.GetLocalDB().Get(key)
	if err != nil {
		pblog.Error("findPowerballGainRecord", "err", err)
		return nil, err
	}

	var record pty.PowerballGainRecord
	err = types.Decode(value, &record)
	if err != nil {
		pblog.Error("findPowerballGainRecord", "err", err)
		return nil, err
	}
	return &record, nil
}

func (ball *Powerball) savePowerballBuy(powerlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	key := calcPowerballBuyKey(powerlog.PowerballID, powerlog.Addr, powerlog.Round, powerlog.Index)
	record := &pty.PowerballBuyRecord{Number: powerlog.Number, Amount: powerlog.Amount, Round: powerlog.Round, Type: Zero,
		Index: powerlog.Index, Time: powerlog.Time, TxHash: powerlog.TxHash}
	kv := &types.KeyValue{Key: key, Value: types.Encode(record)}
	kvs = append(kvs, kv)
	return
}

func (ball *Powerball) deletePowerballBuy(powerlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	key := calcPowerballBuyKey(powerlog.PowerballID, powerlog.Addr, powerlog.Round, powerlog.Index)
	kv := &types.KeyValue{Key: key, Value: nil}
	kvs = append(kvs, kv)
	return
}

func (ball *Powerball) updatePowerballBuy(powerlog *pty.ReceiptPowerball, isAdd bool) (kvs []*types.KeyValue) {
	if powerlog.UpdateInfo != nil {
		pblog.Debug("updatePowerballBuy")
		//update old record
		for _, update := range powerlog.UpdateInfo.Updates {
			for _, updateRec := range update.Records {
				//find addr, index
				key := calcPowerballBuyKey(powerlog.PowerballID, update.Addr, powerlog.Round, updateRec.Index)
				record, err := ball.findPowerballBuyRecord(key)
				if err != nil || record == nil {
					return
				}

				if isAdd {
					pblog.Debug("updatePowerballBuy update key")
					record.Type = updateRec.Type
				} else {
					record.Type = Zero
				}

				kv := &types.KeyValue{Key: key, Value: types.Encode(record)}
				kvs = append(kvs, kv)
			}
		}
	}
	return
}

func (ball *Powerball) savePowerballDraw(powerlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	key := calcPowerballDrawKey(powerlog.PowerballID, powerlog.Round)
	record := &pty.PowerballDrawRecord{Number: powerlog.LuckyNumber, Round: powerlog.Round, Time: powerlog.Time, TxHash: powerlog.TxHash, Info: powerlog.PrizeInfo}
	kv := &types.KeyValue{Key: key, Value: types.Encode(record)}
	kvs = append(kvs, kv)
	return
}

func (ball *Powerball) deletePowerballDraw(powerlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	key := calcPowerballDrawKey(powerlog.PowerballID, powerlog.Round)
	kv := &types.KeyValue{Key: key, Value: nil}
	kvs = append(kvs, kv)
	return
}

func (ball *Powerball) savePowerball(powerlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	if powerlog.PrevStatus > 0 {
		kv := delpowerball(powerlog.PowerballID, powerlog.PrevStatus)
		kvs = append(kvs, kv)
	}
	kvs = append(kvs, addpowerball(powerlog.PowerballID, powerlog.Status))
	return
}

func (ball *Powerball) deletePowerball(powerlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	if powerlog.PrevStatus > 0 {
		kv := addpowerball(powerlog.PowerballID, powerlog.PrevStatus)
		kvs = append(kvs, kv)
	}
	kvs = append(kvs, delpowerball(powerlog.PowerballID, powerlog.Status))
	return
}

func addpowerball(powerballID string, status int32) *types.KeyValue {
	kv := &types.KeyValue{}
	kv.Key = calcPowerballKey(powerballID, status)
	kv.Value = []byte(powerballID)
	return kv
}

func delpowerball(powerballID string, status int32) *types.KeyValue {
	kv := &types.KeyValue{}
	kv.Key = calcPowerballKey(powerballID, status)
	kv.Value = nil
	return kv
}

func (ball *Powerball) savePowerballGain(powerlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	for _, gain := range powerlog.GainInfos.Gains {
		key := calcPowerballGainKey(powerlog.PowerballID, gain.Addr, powerlog.Round)
		record := &pty.PowerballGainRecord{Addr: gain.Addr, BuyAmount: gain.BuyAmount, FundAmount: gain.FundAmount, Round: powerlog.Round}
		kv := &types.KeyValue{Key: key, Value: types.Encode(record)}
		kvs = append(kvs, kv)
	}
	return kvs
}

func (ball *Powerball) deletePowerballGain(powerlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	for _, gain := range powerlog.GainInfos.Gains {
		kv := &types.KeyValue{}
		kv.Key = calcPowerballGainKey(powerlog.PowerballID, gain.Addr, powerlog.Round)
		kv.Value = nil
		kvs = append(kvs, kv)
	}
	return kvs
}

// GetPayloadValue for powerball
func (ball *Powerball) GetPayloadValue() types.Message {
	return &pty.PowerballAction{}
}

// CheckReceiptExecOk return true to check if receipt ty is ok
func (ball *Powerball) CheckReceiptExecOk() bool {
	return true
}
