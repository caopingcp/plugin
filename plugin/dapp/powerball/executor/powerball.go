// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	"sort"

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
	ParaRemoteGrpcClient string `json:"paraRemoteGrpcClient"`
}

var cfg subConfig

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

func GetName() string {
	return newPowerball().GetName()
}

type Powerball struct {
	drivers.DriverBase
}

func newPowerball() drivers.Driver {
	p := &Powerball{}
	p.SetChild(p)
	p.SetExecutorType(types.LoadExecutorType(driverName))
	return p
}

func (p *Powerball) GetDriverName() string {
	return pty.PowerballX
}

func (ball *Powerball) findPowerballBuyRecords(key []byte) (*pty.PowerballBuyRecords, error) {

	count := ball.GetLocalDB().PrefixCount(key)
	pblog.Error("findPowerballBuyRecords", "count", count)

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

func (ball *Powerball) savePowerballBuy(powerballlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	key := calcPowerballBuyKey(powerballlog.PowerballId, powerballlog.Addr, powerballlog.Round, powerballlog.Index)
	kv := &types.KeyValue{}
	record := &pty.PowerballBuyRecord{powerballlog.Number, powerballlog.Amount, powerballlog.Round, 0, powerballlog.Way, powerballlog.Index, powerballlog.Time, powerballlog.TxHash}
	kv = &types.KeyValue{key, types.Encode(record)}

	kvs = append(kvs, kv)
	return kvs
}

func (ball *Powerball) deletePowerballBuy(powerballlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	key := calcPowerballBuyKey(powerballlog.PowerballId, powerballlog.Addr, powerballlog.Round, powerballlog.Index)

	kv := &types.KeyValue{key, nil}
	kvs = append(kvs, kv)
	return kvs
}

func (ball *Powerball) updatePowerballBuy(powerballlog *pty.ReceiptPowerball, isAdd bool) (kvs []*types.KeyValue) {
	if powerballlog.UpdateInfo != nil {
		pblog.Debug("updatePowerballBuy")
		buyInfo := powerballlog.UpdateInfo.BuyInfo
		//sort for map
		addrkeys := make([]string, len(buyInfo))
		i := 0

		for addr := range buyInfo {
			addrkeys[i] = addr
			i++
		}
		sort.Strings(addrkeys)
		//update old record
		for _, addr := range addrkeys {
			for _, updateRec := range buyInfo[addr].Records {
				//find addr, index
				key := calcPowerballBuyKey(powerballlog.PowerballId, addr, powerballlog.Round, updateRec.Index)
				record, err := ball.findPowerballBuyRecord(key)
				if err != nil || record == nil {
					return kvs
				}
				kv := &types.KeyValue{}

				if isAdd {
					pblog.Debug("updatePowerballBuy update key")
					record.Type = updateRec.Type
				} else {
					record.Type = 0
				}

				kv = &types.KeyValue{key, types.Encode(record)}
				kvs = append(kvs, kv)
			}
		}
		return kvs
	}
	return kvs
}

func (ball *Powerball) savePowerballDraw(powerballlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	key := calcPowerballDrawKey(powerballlog.PowerballId, powerballlog.Round)
	kv := &types.KeyValue{}
	record := &pty.PowerballDrawRecord{powerballlog.LuckyNumber, powerballlog.Round, powerballlog.Time, powerballlog.TxHash}
	kv = &types.KeyValue{key, types.Encode(record)}
	kvs = append(kvs, kv)
	return kvs
}

func (ball *Powerball) deletePowerballDraw(powerballlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	key := calcPowerballDrawKey(powerballlog.PowerballId, powerballlog.Round)
	kv := &types.KeyValue{key, nil}
	kvs = append(kvs, kv)
	return kvs
}

func (ball *Powerball) savePowerball(powerballlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	if powerballlog.PrevStatus > 0 {
		kv := delpowerball(powerballlog.PowerballId, powerballlog.PrevStatus)
		kvs = append(kvs, kv)
	}
	kvs = append(kvs, addpowerball(powerballlog.PowerballId, powerballlog.Status))
	return kvs
}

func (ball *Powerball) deletePowerball(powerballlog *pty.ReceiptPowerball) (kvs []*types.KeyValue) {
	if powerballlog.PrevStatus > 0 {
		kv := addpowerball(powerballlog.PowerballId, powerballlog.PrevStatus)
		kvs = append(kvs, kv)
	}
	kvs = append(kvs, delpowerball(powerballlog.PowerballId, powerballlog.Status))
	return kvs
}

func addpowerball(powerballId string, status int32) *types.KeyValue {
	kv := &types.KeyValue{}
	kv.Key = calcPowerballKey(powerballId, status)
	kv.Value = []byte(powerballId)
	return kv
}

func delpowerball(powerballId string, status int32) *types.KeyValue {
	kv := &types.KeyValue{}
	kv.Key = calcPowerballKey(powerballId, status)
	kv.Value = nil
	return kv
}

func (ball *Powerball) GetPayloadValue() types.Message {
	return &pty.PowerballAction{}
}
