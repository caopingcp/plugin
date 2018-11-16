// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/33cn/chain33/account"
	"github.com/33cn/chain33/client"
	"github.com/33cn/chain33/common"
	dbm "github.com/33cn/chain33/common/db"
	"github.com/33cn/chain33/system/dapp"
	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
	"google.golang.org/grpc"
)

const (
	exciting = 100000 / 2
	lucky    = 1000 / 2
	happy    = 100 / 2
	notbad   = 10 / 2
)

const (
	minPurBlockNum  = 30
	minDrawBlockNum = 40
)

const (
	creatorKey = "powerball-creator"
)

const (
	ListDESC    = int32(0)
	ListASC     = int32(1)
	DefultCount = int32(20)  //默认一次取多少条记录
	MaxCount    = int32(100) //最多取100条
)

const (
	FiveStar  = 5
	ThreeStar = 3
	TwoStar   = 2
	OneStar   = 1
)

//const defaultAddrPurTimes = 10
const luckyNumMol = 100000
const decimal = 100000000 //1e8
const randMolNum = 5
const grpcRecSize int = 5 * 30 * 1024 * 1024
const blockNum = 5

type PowerballDB struct {
	pty.Powerball
}

func NewPowerballDB(powerballId string, purBlock int64, drawBlock int64,
	blockHeight int64, addr string) *PowerballDB {
	ball := &PowerballDB{}
	ball.PowerballId = powerballId
	ball.PurBlockNum = purBlock
	ball.DrawBlockNum = drawBlock
	ball.CreateHeight = blockHeight
	ball.Fund = 0
	ball.Status = pty.PowerballCreated
	ball.TotalPurchasedTxNum = 0
	ball.CreateAddr = addr
	ball.Round = 0
	ball.MissingRecords = make([]*pty.MissingRecord, 5)
	for index := range ball.MissingRecords {
		tempTimes := make([]int32, 10)
		ball.MissingRecords[index] = &pty.MissingRecord{tempTimes}
	}
	return ball
}

func (ball *PowerballDB) GetKVSet() (kvset []*types.KeyValue) {
	value := types.Encode(&ball.Powerball)
	kvset = append(kvset, &types.KeyValue{Key(ball.PowerballId), value})
	return kvset
}

func (ball *PowerballDB) Save(db dbm.KV) {
	set := ball.GetKVSet()
	for i := 0; i < len(set); i++ {
		db.Set(set[i].GetKey(), set[i].Value)
	}
}

func Key(id string) (key []byte) {
	key = append(key, []byte("mavl-"+pty.PowerballX+"-")...)
	key = append(key, []byte(id)...)
	return key
}

type Action struct {
	coinsAccount *account.DB
	db           dbm.KV
	txhash       []byte
	fromaddr     string
	blocktime    int64
	height       int64
	execaddr     string
	difficulty   uint64
	api          client.QueueProtocolAPI
	conn         *grpc.ClientConn
	grpcClient   types.Chain33Client
	index        int
}

func NewPowerballAction(l *Powerball, tx *types.Transaction, index int) *Action {
	hash := tx.Hash()
	fromaddr := tx.From()

	msgRecvOp := grpc.WithMaxMsgSize(grpcRecSize)
	conn, err := grpc.Dial(cfg.ParaRemoteGrpcClient, grpc.WithInsecure(), msgRecvOp)

	if err != nil {
		panic(err)
	}
	grpcClient := types.NewChain33Client(conn)

	return &Action{l.GetCoinsAccount(), l.GetStateDB(), hash, fromaddr, l.GetBlockTime(),
		l.GetHeight(), dapp.ExecAddress(string(tx.Execer)), l.GetDifficulty(), l.GetApi(), conn, grpcClient, index}
}

func (action *Action) GetReceiptLog(powerball *pty.Powerball, preStatus int32, logTy int32,
	round int64, buyNumber int64, amount int64, way int64, luckyNum int64, updateInfo *pty.PowerballUpdateBuyInfo) *types.ReceiptLog {
	log := &types.ReceiptLog{}
	l := &pty.ReceiptPowerball{}

	log.Ty = logTy

	l.PowerballId = powerball.PowerballId
	l.Status = powerball.Status
	l.PrevStatus = preStatus
	if logTy == pty.TyLogPowerballBuy {
		l.Round = round
		l.Number = buyNumber
		l.Amount = amount
		l.Addr = action.fromaddr
		l.Way = way
		l.Index = action.GetIndex()
		l.Time = action.blocktime
		l.TxHash = common.ToHex(action.txhash)
	}
	if logTy == pty.TyLogPowerballDraw {
		l.Round = round
		l.LuckyNumber = luckyNum
		l.Time = action.blocktime
		l.TxHash = common.ToHex(action.txhash)
		if len(updateInfo.BuyInfo) > 0 {
			l.UpdateInfo = updateInfo
		}
	}

	log.Log = types.Encode(l)
	return log
}

//fmt.Sprintf("%018d", action.height*types.MaxTxsPerBlock+int64(action.index))
func (action *Action) GetIndex() int64 {
	return action.height*types.MaxTxsPerBlock + int64(action.index)
}

func (action *Action) PowerballCreate(create *pty.PowerballCreate) (*types.Receipt, error) {
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	var receipt *types.Receipt

	powerballId := common.ToHex(action.txhash)

	if !isRightCreator(action.fromaddr, action.db, false) {
		return nil, pty.ErrNoPrivilege
	}

	if create.GetPurBlockNum() < minPurBlockNum {
		return nil, pty.ErrPowerballPurBlockLimit
	}

	if create.GetDrawBlockNum() < minDrawBlockNum {
		return nil, pty.ErrPowerballDrawBlockLimit
	}

	if create.GetPurBlockNum() > create.GetDrawBlockNum() {
		return nil, pty.ErrPowerballDrawBlockLimit
	}

	_, err := findPowerball(action.db, powerballId)
	if err != types.ErrNotFound {
		pblog.Error("PowerballCreate", "PowerballCreate repeated", powerballId)
		return nil, pty.ErrPowerballRepeatHash
	}

	ball := NewPowerballDB(powerballId, create.GetPurBlockNum(),
		create.GetDrawBlockNum(), action.height, action.fromaddr)

	if types.IsPara() {
		mainHeight := action.GetMainHeightByTxHash(action.txhash)
		if mainHeight < 0 {
			pblog.Error("PowerballCreate", "mainHeight", mainHeight)
			return nil, pty.ErrPowerballStatus
		}
		ball.CreateOnMain = mainHeight
	}

	pblog.Debug("PowerballCreate created", "powerballId", powerballId)

	ball.Save(action.db)
	kv = append(kv, ball.GetKVSet()...)

	receiptLog := action.GetReceiptLog(&ball.Powerball, 0, pty.TyLogPowerballCreate, 0, 0, 0, 0, 0, nil)
	logs = append(logs, receiptLog)

	receipt = &types.Receipt{types.ExecOk, kv, logs}
	return receipt, nil
}

//one bty for one ticket
func (action *Action) PowerballBuy(buy *pty.PowerballBuy) (*types.Receipt, error) {
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	//var receipt *types.Receipt

	powerball, err := findPowerball(action.db, buy.PowerballId)
	if err != nil {
		pblog.Error("PowerballBuy", "PowerballId", buy.PowerballId)
		return nil, err
	}

	ball := &PowerballDB{*powerball}
	preStatus := ball.Status

	if ball.Status == pty.PowerballClosed {
		pblog.Error("PowerballBuy", "status", ball.Status)
		return nil, pty.ErrPowerballStatus
	}

	if ball.Status == pty.PowerballDrawed {
		//no problem both on main and para
		if action.height <= ball.LastTransToDrawState {
			pblog.Error("PowerballBuy", "action.heigt", action.height, "lastTransToDrawState", ball.LastTransToDrawState)
			return nil, pty.ErrPowerballStatus
		}
	}

	if ball.Status == pty.PowerballCreated || ball.Status == pty.PowerballDrawed {
		pblog.Debug("PowerballBuy switch to purchasestate")
		ball.LastTransToPurState = action.height
		ball.Status = pty.PowerballPurchase
		ball.Round += 1
		if types.IsPara() {
			mainHeight := action.GetMainHeightByTxHash(action.txhash)
			if mainHeight < 0 {
				pblog.Error("PowerballBuy", "mainHeight", mainHeight)
				return nil, pty.ErrPowerballStatus
			}
			ball.LastTransToPurStateOnMain = mainHeight
		}
	}

	if ball.Status == pty.PowerballPurchase {
		if types.IsPara() {
			mainHeight := action.GetMainHeightByTxHash(action.txhash)
			if mainHeight < 0 {
				pblog.Error("PowerballBuy", "mainHeight", mainHeight)
				return nil, pty.ErrPowerballStatus
			}
			if mainHeight-ball.LastTransToPurStateOnMain > ball.GetPurBlockNum() {
				pblog.Error("PowerballBuy", "action.height", action.height, "mainHeight", mainHeight, "LastTransToPurStateOnMain", ball.LastTransToPurStateOnMain)
				return nil, pty.ErrPowerballStatus
			}
		} else {
			if action.height-ball.LastTransToPurState > ball.GetPurBlockNum() {
				pblog.Error("PowerballBuy", "action.height", action.height, "LastTransToPurState", ball.LastTransToPurState)
				return nil, pty.ErrPowerballStatus
			}
		}
	}

	if ball.CreateAddr == action.fromaddr {
		return nil, pty.ErrPowerballCreatorBuy
	}

	if buy.GetAmount() <= 0 {
		pblog.Error("PowerballBuy", "buyAmount", buy.GetAmount())
		return nil, pty.ErrPowerballBuyAmount
	}

	if buy.GetNumber() < 0 || buy.GetNumber() >= luckyNumMol {
		pblog.Error("PowerballBuy", "buyNumber", buy.GetNumber())
		return nil, pty.ErrPowerballBuyNumber
	}

	if ball.Records == nil {
		pblog.Debug("PowerballBuy records init")
		ball.Records = make(map[string]*pty.PurchaseRecords)
	}

	newRecord := &pty.PurchaseRecord{buy.GetAmount(), buy.GetNumber(), action.GetIndex(), buy.GetWay()}
	pblog.Debug("PowerballBuy", "amount", buy.GetAmount(), "number", buy.GetNumber())

	/**********
	Once ExecTransfer succeed, ExecFrozen succeed, no roolback needed
	**********/

	receipt, err := action.coinsAccount.ExecTransfer(action.fromaddr, ball.CreateAddr, action.execaddr, buy.GetAmount()*decimal)
	if err != nil {
		pblog.Error("PowerballBuy.ExecTransfer", "addr", action.fromaddr, "execaddr", action.execaddr, "amount", buy.GetAmount())
		return nil, err
	}
	logs = append(logs, receipt.Logs...)
	kv = append(kv, receipt.KV...)

	receipt, err = action.coinsAccount.ExecFrozen(ball.CreateAddr, action.execaddr, buy.GetAmount()*decimal)

	if err != nil {
		pblog.Error("PowerballBuy.Frozen", "addr", ball.CreateAddr, "execaddr", action.execaddr, "amount", buy.GetAmount())
		return nil, err
	}
	logs = append(logs, receipt.Logs...)
	kv = append(kv, receipt.KV...)

	ball.Fund += buy.GetAmount()

	if record, ok := ball.Records[action.fromaddr]; ok {
		record.Record = append(record.Record, newRecord)
	} else {
		initrecord := &pty.PurchaseRecords{}
		initrecord.Record = append(initrecord.Record, newRecord)
		initrecord.FundWin = 0
		initrecord.AmountOneRound = 0
		ball.Records[action.fromaddr] = initrecord
	}
	ball.Records[action.fromaddr].AmountOneRound += buy.Amount
	ball.TotalPurchasedTxNum++

	ball.Save(action.db)
	kv = append(kv, ball.GetKVSet()...)

	receiptLog := action.GetReceiptLog(&ball.Powerball, preStatus, pty.TyLogPowerballBuy, ball.Round, buy.GetNumber(), buy.GetAmount(), buy.GetWay(), 0, nil)
	logs = append(logs, receiptLog)

	receipt = &types.Receipt{types.ExecOk, kv, logs}
	return receipt, nil
}

//1.Anyone who buy a ticket
//2.Creator
func (action *Action) PowerballDraw(draw *pty.PowerballDraw) (*types.Receipt, error) {
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	var receipt *types.Receipt

	powerball, err := findPowerball(action.db, draw.PowerballId)
	if err != nil {
		pblog.Error("PowerballBuy", "PowerballId", draw.PowerballId)
		return nil, err
	}

	ball := &PowerballDB{*powerball}

	preStatus := ball.Status

	if ball.Status != pty.PowerballPurchase {
		pblog.Error("PowerballDraw", "ball.Status", ball.Status)
		return nil, pty.ErrPowerballStatus
	}

	if types.IsPara() {
		mainHeight := action.GetMainHeightByTxHash(action.txhash)
		if mainHeight < 0 {
			pblog.Error("PowerballBuy", "mainHeight", mainHeight)
			return nil, pty.ErrPowerballStatus
		}
		if mainHeight-ball.GetLastTransToPurStateOnMain() < ball.GetDrawBlockNum() {
			pblog.Error("PowerballDraw", "action.height", action.height, "mainHeight", mainHeight, "GetLastTransToPurStateOnMain", ball.GetLastTransToPurState())
			return nil, pty.ErrPowerballStatus
		}
	} else {
		if action.height-ball.GetLastTransToPurState() < ball.GetDrawBlockNum() {
			pblog.Error("PowerballDraw", "action.height", action.height, "GetLastTransToPurState", ball.GetLastTransToPurState())
			return nil, pty.ErrPowerballStatus
		}
	}

	if action.fromaddr != ball.GetCreateAddr() {
		if _, ok := ball.Records[action.fromaddr]; !ok {
			pblog.Error("PowerballDraw", "action.fromaddr", action.fromaddr)
			return nil, pty.ErrPowerballDrawActionInvalid
		}
	}

	rec, updateInfo, err := action.checkDraw(ball)
	if err != nil {
		return nil, err
	}
	kv = append(kv, rec.KV...)
	logs = append(logs, rec.Logs...)

	ball.Save(action.db)
	kv = append(kv, ball.GetKVSet()...)

	receiptLog := action.GetReceiptLog(&ball.Powerball, preStatus, pty.TyLogPowerballDraw, ball.Round, 0, 0, 0, ball.LuckyNumber, updateInfo)
	logs = append(logs, receiptLog)

	receipt = &types.Receipt{types.ExecOk, kv, logs}
	return receipt, nil
}

func (action *Action) PowerballClose(draw *pty.PowerballClose) (*types.Receipt, error) {
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	//var receipt *types.Receipt

	if !isEableToClose() {
		return nil, pty.ErrPowerballErrUnableClose
	}

	powerball, err := findPowerball(action.db, draw.PowerballId)
	if err != nil {
		pblog.Error("PowerballBuy", "PowerballId", draw.PowerballId)
		return nil, err
	}

	ball := &PowerballDB{*powerball}
	preStatus := ball.Status

	if action.fromaddr != ball.CreateAddr {
		return nil, pty.ErrPowerballErrCloser
	}

	if ball.Status == pty.PowerballClosed {
		return nil, pty.ErrPowerballStatus
	}

	addrkeys := make([]string, len(ball.Records))
	i := 0
	var totalReturn int64 = 0
	for addr := range ball.Records {
		totalReturn += ball.Records[addr].AmountOneRound
		addrkeys[i] = addr
		i++
	}
	pblog.Debug("PowerballClose", "totalReturn", totalReturn)

	if totalReturn > 0 {

		if !action.CheckExecAccount(ball.CreateAddr, decimal*totalReturn, true) {
			return nil, pty.ErrPowerballFundNotEnough
		}

		sort.Strings(addrkeys)

		for _, addr := range addrkeys {
			if ball.Records[addr].AmountOneRound > 0 {
				receipt, err := action.coinsAccount.ExecTransferFrozen(ball.CreateAddr, addr, action.execaddr,
					decimal*ball.Records[addr].AmountOneRound)
				if err != nil {
					return nil, err
				}

				kv = append(kv, receipt.KV...)
				logs = append(logs, receipt.Logs...)
			}
		}
	}

	for addr := range ball.Records {
		ball.Records[addr].Record = ball.Records[addr].Record[0:0]
		delete(ball.Records, addr)
	}

	ball.TotalPurchasedTxNum = 0
	pblog.Debug("PowerballClose switch to closestate")
	ball.Status = pty.PowerballClosed

	ball.Save(action.db)
	kv = append(kv, ball.GetKVSet()...)

	receiptLog := action.GetReceiptLog(&ball.Powerball, preStatus, pty.TyLogPowerballClose, 0, 0, 0, 0, 0, nil)
	logs = append(logs, receiptLog)

	return &types.Receipt{types.ExecOk, kv, logs}, nil
}

func (action *Action) GetModify(beg, end int64, randMolNum int64) ([]byte, error) {
	//通过某个区间计算modify
	timeSource := int64(0)
	total := int64(0)
	//last := []byte("last")
	newmodify := ""
	for i := beg; i < end; i += randMolNum {
		req := &types.ReqBlocks{i, i, false, []string{""}}
		blocks, err := action.api.GetBlocks(req)
		if err != nil {
			return []byte{}, err
		}
		block := blocks.Items[0].Block
		timeSource += block.BlockTime
		total += block.BlockTime
	}

	//for main chain, 5 latest block
	//for para chain, 5 latest block -- 5 sequence main block
	txActions, err := action.getTxActions(end, blockNum)
	if err != nil {
		return nil, err
	}

	//modify, bits, id
	var modifies []byte
	var bits uint32
	var ticketIds string

	for _, ticketAction := range txActions {
		pblog.Debug("GetModify", "modify", ticketAction.GetMiner().GetModify(), "bits", ticketAction.GetMiner().GetBits(), "ticketId", ticketAction.GetMiner().GetTicketId())
		modifies = append(modifies, ticketAction.GetMiner().GetModify()...)
		bits += ticketAction.GetMiner().GetBits()
		ticketIds += ticketAction.GetMiner().GetTicketId()
	}

	newmodify = fmt.Sprintf("%s:%s:%d:%d", string(modifies), ticketIds, total, bits)

	modify := common.Sha256([]byte(newmodify))
	return modify, nil
}

//random used for verfication in solo
func (action *Action) findLuckyNum(isSolo bool, ball *PowerballDB) int64 {
	var num int64 = 0
	if isSolo {
		//used for internal verfication
		num = 12345
	} else {
		randMolNum := (ball.TotalPurchasedTxNum+action.height-ball.LastTransToPurState)%3 + 2 //3~5

		modify, err := action.GetModify(ball.LastTransToPurState, action.height-1, randMolNum)
		pblog.Error("findLuckyNum", "begin", ball.LastTransToPurState, "end", action.height-1, "randMolNum", randMolNum)

		if err != nil {
			pblog.Error("findLuckyNum", "err", err)
			return -1
		}

		baseNum, err := strconv.ParseUint(common.ToHex(modify[0:4]), 0, 64)
		if err != nil {
			pblog.Error("findLuckyNum", "err", err)
			return -1
		}

		num = int64(baseNum) % luckyNumMol
	}
	return num
}

func checkFundAmount(luckynum int64, guessnum int64, way int64) (int64, int64) {
	if way == FiveStar && luckynum == guessnum {
		return exciting, FiveStar
	} else if way == ThreeStar && luckynum%1000 == guessnum%1000 {
		return lucky, ThreeStar
	} else if way == TwoStar && luckynum%100 == guessnum%100 {
		return happy, TwoStar
	} else if way == OneStar && luckynum%10 == guessnum%10 {
		return notbad, OneStar
	} else {
		return 0, 0
	}
}

func (action *Action) checkDraw(ball *PowerballDB) (*types.Receipt, *pty.PowerballUpdateBuyInfo, error) {
	pblog.Debug("checkDraw")

	luckynum := action.findLuckyNum(false, ball)
	if luckynum < 0 || luckynum >= luckyNumMol {
		return nil, nil, pty.ErrPowerballErrLuckyNum
	}

	pblog.Error("checkDraw", "luckynum", luckynum)

	//var receipt *types.Receipt
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue

	//calculate fund for all participant showed their number
	var updateInfo pty.PowerballUpdateBuyInfo
	updateInfo.BuyInfo = make(map[string]*pty.PowerballUpdateRecs)
	var tempFund int64 = 0
	var totalFund int64 = 0
	addrkeys := make([]string, len(ball.Records))
	i := 0
	for addr := range ball.Records {
		addrkeys[i] = addr
		i++
		for _, rec := range ball.Records[addr].Record {
			fund, fundType := checkFundAmount(luckynum, rec.Number, rec.Way)
			if fund != 0 {
				newUpdateRec := &pty.PowerballUpdateRec{rec.Index, fundType}
				if update, ok := updateInfo.BuyInfo[addr]; ok {
					update.Records = append(update.Records, newUpdateRec)
				} else {
					initrecord := &pty.PowerballUpdateRecs{}
					initrecord.Records = append(initrecord.Records, newUpdateRec)
					updateInfo.BuyInfo[addr] = initrecord
				}
			}
			tempFund = fund * rec.Amount
			ball.Records[addr].FundWin += tempFund
			totalFund += tempFund
		}
	}
	pblog.Debug("checkDraw", "lenofupdate", len(updateInfo.BuyInfo))
	pblog.Debug("checkDraw", "update", updateInfo.BuyInfo)
	var factor float64 = 0
	if totalFund > ball.GetFund()/2 {
		pblog.Debug("checkDraw ajust fund", "ball.Fund", ball.Fund, "totalFund", totalFund)
		factor = (float64)(ball.GetFund()) / 2 / (float64)(totalFund)
		ball.Fund = ball.Fund / 2
	} else {
		factor = 1.0
		ball.Fund -= totalFund
	}

	pblog.Debug("checkDraw", "factor", factor, "totalFund", totalFund)

	//protection for rollback
	if factor == 1.0 {
		if !action.CheckExecAccount(ball.CreateAddr, totalFund, true) {
			return nil, nil, pty.ErrPowerballFundNotEnough
		}
	} else {
		if !action.CheckExecAccount(ball.CreateAddr, decimal*ball.Fund/2+1, true) {
			return nil, nil, pty.ErrPowerballFundNotEnough
		}
	}

	sort.Strings(addrkeys)

	for _, addr := range addrkeys {
		fund := (ball.Records[addr].FundWin * int64(factor*exciting)) * decimal / exciting //any problem when too little?
		pblog.Debug("checkDraw", "fund", fund)
		if fund > 0 {
			receipt, err := action.coinsAccount.ExecTransferFrozen(ball.CreateAddr, addr, action.execaddr, fund)
			if err != nil {
				return nil, nil, err
			}

			kv = append(kv, receipt.KV...)
			logs = append(logs, receipt.Logs...)
		}
	}

	for addr := range ball.Records {
		ball.Records[addr].Record = ball.Records[addr].Record[0:0]
		delete(ball.Records, addr)
	}

	pblog.Debug("checkDraw powerball switch to drawed")
	ball.LastTransToDrawState = action.height
	ball.Status = pty.PowerballDrawed
	ball.TotalPurchasedTxNum = 0
	ball.LuckyNumber = luckynum
	action.recordMissing(ball)

	if types.IsPara() {
		mainHeight := action.GetMainHeightByTxHash(action.txhash)
		if mainHeight < 0 {
			pblog.Error("PowerballBuy", "mainHeight", mainHeight)
			return nil, nil, pty.ErrPowerballStatus
		}
		ball.LastTransToDrawStateOnMain = mainHeight
	}

	return &types.Receipt{types.ExecOk, kv, logs}, &updateInfo, nil
}
func (action *Action) recordMissing(ball *PowerballDB) {
	temp := int32(ball.LuckyNumber)
	initNum := int32(10000)
	sample := [10]int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	var eachNum [5]int32
	for i := 0; i < 5; i++ {
		eachNum[i] = temp / initNum
		temp -= eachNum[i] * initNum
		initNum = initNum / 10
	}
	for i := 0; i < 5; i++ {
		for j := 0; j < 10; j++ {
			if eachNum[i] != sample[j] {
				ball.MissingRecords[i].Times[j] += 1
			}
		}
	}
}

func getManageKey(key string, db dbm.KV) ([]byte, error) {
	manageKey := types.ManageKey(key)
	value, err := db.Get([]byte(manageKey))
	if err != nil {
		return nil, err
	}
	return value, nil
}

func isRightCreator(addr string, db dbm.KV, isSolo bool) bool {
	if isSolo {
		return true
	} else {
		value, err := getManageKey(creatorKey, db)
		if err != nil {
			pblog.Error("PowerballCreate", "creatorKey", creatorKey)
			return false
		}
		if value == nil {
			pblog.Error("PowerballCreate found nil value")
			return false
		}

		var item types.ConfigItem
		err = types.Decode(value, &item)
		if err != nil {
			pblog.Error("PowerballCreate", "Decode", value)
			return false
		}

		for _, op := range item.GetArr().Value {
			if op == addr {
				return true
			}
		}
		return false
	}
}

func isEableToClose() bool {
	return true
}

func findPowerball(db dbm.KV, powerballId string) (*pty.Powerball, error) {
	data, err := db.Get(Key(powerballId))
	if err != nil {
		pblog.Debug("findPowerball", "get", err)
		return nil, err
	}
	var ball pty.Powerball
	//decode
	err = types.Decode(data, &ball)
	if err != nil {
		pblog.Debug("findPowerball", "decode", err)
		return nil, err
	}
	return &ball, nil
}

func (action *Action) CheckExecAccount(addr string, amount int64, isFrozen bool) bool {
	acc := action.coinsAccount.LoadExecAccount(addr, action.execaddr)
	if isFrozen {
		if acc.GetFrozen() >= amount {
			return true
		}
	} else {
		if acc.GetBalance() >= amount {
			return true
		}
	}

	return false
}

func ListPowerballLuckyHistory(db dbm.Lister, stateDB dbm.KV, param *pty.ReqPowerballLuckyHistory) (types.Message, error) {
	direction := ListDESC
	if param.GetDirection() == ListASC {
		direction = ListASC
	}
	count := DefultCount
	if 0 < param.GetCount() && param.GetCount() <= MaxCount {
		count = param.GetCount()
	}
	var prefix []byte
	var key []byte
	var values [][]byte
	var err error

	prefix = calcPowerballDrawPrefix(param.PowerballId)
	key = calcPowerballDrawKey(param.PowerballId, param.GetRound())

	if param.GetRound() == 0 { //第一次查询
		values, err = db.List(prefix, nil, count, direction)
	} else {
		values, err = db.List(prefix, key, count, direction)
	}
	if err != nil {
		return nil, err
	}

	var records pty.PowerballDrawRecords
	for _, value := range values {
		var record pty.PowerballDrawRecord
		err := types.Decode(value, &record)
		if err != nil {
			continue
		}
		records.Records = append(records.Records, &record)
	}

	return &records, nil
}

func ListPowerballBuyRecords(db dbm.Lister, stateDB dbm.KV, param *pty.ReqPowerballBuyHistory) (types.Message, error) {
	direction := ListDESC
	if param.GetDirection() == ListASC {
		direction = ListASC
	}
	count := DefultCount
	if 0 < param.GetCount() && param.GetCount() <= MaxCount {
		count = param.GetCount()
	}
	var prefix []byte
	var key []byte
	var values [][]byte
	var err error

	prefix = calcPowerballBuyPrefix(param.PowerballId, param.Addr)
	key = calcPowerballBuyKey(param.PowerballId, param.Addr, param.GetRound(), param.GetIndex())

	if param.GetRound() == 0 { //第一次查询
		values, err = db.List(prefix, nil, count, direction)
	} else {
		values, err = db.List(prefix, key, count, direction)
	}

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
