// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package executor

import (
	"fmt"
	"strconv"

	"github.com/33cn/chain33/account"
	"github.com/33cn/chain33/common"
	dbm "github.com/33cn/chain33/common/db"
	"github.com/33cn/chain33/system/dapp"
	"github.com/33cn/chain33/types"
	pty "github.com/33cn/plugin/plugin/dapp/powerball/types"
)

// ball number and range
var (
	RedBalls = pty.RedBalls
	RedRange = pty.RedRange

	BlueBalls = pty.BlueBalls
	BlueRange = pty.BlueRange
)

// prize range
const (
	Zero = iota
	First
	Second
	Third
	Fourth
	Fifth
	Sixth

	PrizeRange = 7
)

// prize proportion, per mille
const (
	FirstRatio  = 520
	SecondRatio = 256
	ThirdRatio  = 128
	FourthRatio = 64
	FifthRatio  = 32

	SixthRatio = 2 //fixed, double ticket price
)

// allocation proportion, per mille
const (
	NextRatio = 100 //当期销售收入按一定比例进入下期奖池
)

// platform and develop account address
const (
	PlatformAddr = "1PHtChNt3UcfssR7v7trKSk3WJtAWjKjjX"
	DevelopAddr  = "1D6RFZNp2rh6QdbcZ1d7RWuBUz61We6SD7"
)

const (
	//const defaultAddrPurTimes = 10
	decimal  = 100000000 //1e8
	blockNum = 5

	minPauseBlockNum = 5

	creatorKey = "powerball-creator"
)

// search parameter
const (
	ListDESC    = int32(0)
	ListASC     = int32(1)
	DefultCount = int32(20)  //默认一次取多少条记录
	MaxCount    = int32(100) //最多取100条
)

// PowerballDB struct
type PowerballDB struct {
	pty.Powerball
}

// NewPowerballDB method
func NewPowerballDB(powerballID string, purTime string, drawTime string, ticketPrice int64,
	platformRatio int64, developRatio int64, blockHeight int64, addr string) *PowerballDB {
	ball := &PowerballDB{}
	ball.PowerballID = powerballID
	ball.PurTime = purTime
	ball.DrawTime = drawTime
	ball.TicketPrice = ticketPrice
	ball.PlatformRatio = platformRatio
	ball.DevelopRatio = developRatio
	ball.CreateHeight = blockHeight
	ball.AccuFund = 0
	ball.SaleFund = 0
	ball.Status = pty.PowerballCreated
	ball.TotalPurchasedTxNum = 0
	ball.TotalAddrNum = 0
	ball.CreateAddr = addr
	ball.Round = 0
	ball.LuckyNumber = nil
	ball.MissingRecords = make([]*pty.PowerMissingRecord, 2)
	ball.MissingRecords[0] = &pty.PowerMissingRecord{Times: make([]int64, RedRange)}
	ball.MissingRecords[1] = &pty.PowerMissingRecord{Times: make([]int64, BlueRange)}
	return ball
}

// GetKVSet method
func (ball *PowerballDB) GetKVSet() (kvset []*types.KeyValue) {
	value := types.Encode(&ball.Powerball)
	kvset = append(kvset, &types.KeyValue{Key: Key(ball.PowerballID), Value: value})
	return kvset
}

// Save method
func (ball *PowerballDB) Save(db dbm.KV) {
	set := ball.GetKVSet()
	for i := 0; i < len(set); i++ {
		db.Set(set[i].GetKey(), set[i].Value)
	}
}

// Key method
func Key(id string) (key []byte) {
	key = append(key, []byte("mavl-"+pty.PowerballX+"-")...)
	key = append(key, []byte(id)...)
	return key
}

// Action struct
type Action struct {
	coinsAccount *account.DB
	db           dbm.KV
	txhash       []byte
	fromaddr     string
	blocktime    int64
	height       int64
	execaddr     string
	difficulty   uint64
	index        int
	powerball    *Powerball
}

// NewPowerballAction method
func NewPowerballAction(l *Powerball, tx *types.Transaction, index int) *Action {
	hash := tx.Hash()
	fromaddr := tx.From()
	return &Action{
		coinsAccount: l.GetCoinsAccount(), db: l.GetStateDB(),
		txhash: hash, fromaddr: fromaddr, blocktime: l.GetBlockTime(),
		height: l.GetHeight(), execaddr: dapp.ExecAddress(string(tx.Execer)),
		difficulty: l.GetDifficulty(), index: index, powerball: l}
}

// GetPowerCommonRecipt generate logs for powerball common action
func (action *Action) GetPowerCommonRecipt(powerball *pty.Powerball, preStatus int32) *pty.ReceiptPowerball {
	p := &pty.ReceiptPowerball{}
	p.PowerballID = powerball.PowerballID
	p.Status = powerball.Status
	p.PrevStatus = preStatus
	return p
}

// GetCreateReceiptLog generate logs for powerball create action
func (action *Action) GetCreateReceiptLog(powerball *pty.Powerball, preStatus int32) *types.ReceiptLog {
	log := &types.ReceiptLog{}
	log.Ty = pty.TyLogPowerballCreate

	p := action.GetPowerCommonRecipt(powerball, preStatus)
	log.Log = types.Encode(p)
	return log
}

// GetBuyReceiptLog generate logs for powerball buy action
func (action *Action) GetBuyReceiptLog(powerball *pty.Powerball, preStatus int32, round int64, buyNumber *pty.BallNumber, amount int64) *types.ReceiptLog {
	log := &types.ReceiptLog{}
	log.Ty = pty.TyLogPowerballBuy

	p := action.GetPowerCommonRecipt(powerball, preStatus)
	p.Round = round
	p.Number = buyNumber
	p.Amount = amount
	p.Addr = action.fromaddr
	p.Index = action.GetIndex()
	p.Time = action.blocktime
	p.TxHash = common.ToHex(action.txhash)
	log.Log = types.Encode(p)
	return log
}

// GetPauseReceiptLog generate logs for powerball pause action
func (action *Action) GetPauseReceiptLog(powerball *pty.Powerball, preStatus int32) *types.ReceiptLog {
	log := &types.ReceiptLog{}
	log.Ty = pty.TyLogPowerballPause

	p := action.GetPowerCommonRecipt(powerball, preStatus)
	log.Log = types.Encode(p)
	return log
}

// GetDrawReceiptLog generate logs for powerball draw action
func (action *Action) GetDrawReceiptLog(powerball *pty.Powerball, preStatus int32, round int64, luckyNum *pty.BallNumber, updateInfo *pty.PowerballUpdateBuyInfo,
	addrNumThisRound int64, buyAmountThisRound int64, gainInfos *pty.PowerballGainInfos, prizeInfo []*pty.PowerballPrizeInfo) *types.ReceiptLog {
	log := &types.ReceiptLog{}
	log.Ty = pty.TyLogPowerballDraw

	p := action.GetPowerCommonRecipt(powerball, preStatus)
	p.Round = round
	p.LuckyNumber = luckyNum
	p.Time = action.blocktime
	p.TxHash = common.ToHex(action.txhash)
	p.TotalAddrNum = addrNumThisRound
	p.SaleFund = buyAmountThisRound
	if len(updateInfo.Updates) > 0 {
		p.UpdateInfo = updateInfo
	}
	p.GainInfos = gainInfos
	p.PrizeInfo = prizeInfo
	log.Log = types.Encode(p)
	return log
}

// GetCloseReceiptLog generate logs for powerball close action
func (action *Action) GetCloseReceiptLog(powerball *pty.Powerball, preStatus int32) *types.ReceiptLog {
	log := &types.ReceiptLog{}
	log.Ty = pty.TyLogPowerballClose

	p := action.GetPowerCommonRecipt(powerball, preStatus)
	log.Log = types.Encode(p)
	return log
}

// GetIndex method
func (action *Action) GetIndex() int64 {
	return action.height*types.MaxTxsPerBlock + int64(action.index)
}

// PowerballCreate create powerball
func (action *Action) PowerballCreate(create *pty.PowerballCreate) (*types.Receipt, error) {
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	var receipt *types.Receipt

	powerballID := common.ToHex(action.txhash)

	if !isRightCreator(action.fromaddr, action.db, false) {
		return nil, pty.ErrNoPrivilege
	}

	_, err := findPowerball(action.db, powerballID)
	if err != types.ErrNotFound {
		pblog.Error("PowerballCreate", "PowerballCreate repeated", powerballID)
		return nil, pty.ErrPowerballRepeatHash
	}

	ball := NewPowerballDB(powerballID, create.GetPurTime(),
		create.GetDrawTime(), create.GetTicketPrice(), create.GetPlatformRatio(), create.GetDevelopRatio(), action.height, action.fromaddr)

	if types.IsPara() {
		mainHeight := action.powerball.GetMainHeight()
		if mainHeight < 0 {
			pblog.Error("PowerballCreate", "mainHeight", mainHeight)
			return nil, pty.ErrPowerballStatus
		}
		ball.CreateOnMain = mainHeight
	}

	pblog.Debug("PowerballCreate created", "powerballId", powerballID)

	ball.Save(action.db)
	kv = append(kv, ball.GetKVSet()...)

	receiptLog := action.GetCreateReceiptLog(&ball.Powerball, pty.PowerballNil)
	logs = append(logs, receiptLog)

	receipt = &types.Receipt{Ty: types.ExecOk, KV: kv, Logs: logs}
	return receipt, nil
}

// PowerballBuy buy powerball
func (action *Action) PowerballBuy(buy *pty.PowerballBuy) (*types.Receipt, error) {
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue

	powerball, err := findPowerball(action.db, buy.PowerballID)
	if err != nil {
		pblog.Error("PowerballBuy", "PowerballId", buy.PowerballID)
		return nil, err
	}

	ball := &PowerballDB{*powerball}
	preStatus := ball.Status

	if ball.Status == pty.PowerballPaused || ball.Status == pty.PowerballClosed {
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
		pblog.Debug("PowerballBuy switch to purchase state")
		ball.LastTransToPurState = action.height
		ball.Status = pty.PowerballPurchase
		ball.Round++
		ball.LuckyNumber = nil
		if types.IsPara() {
			mainHeight := action.powerball.GetMainHeight()
			if mainHeight < 0 {
				pblog.Error("PowerballBuy", "mainHeight", mainHeight)
				return nil, pty.ErrPowerballStatus
			}
			ball.LastTransToPurStateOnMain = mainHeight
		}
	}

	if ball.CreateAddr == action.fromaddr {
		return nil, pty.ErrPowerballCreatorBuy
	}

	if buy.GetAmount() <= 0 {
		pblog.Error("PowerballBuy", "buyAmount", buy.GetAmount())
		return nil, pty.ErrPowerballBuyAmount
	}

	if buy.GetNumber() == nil {
		return nil, pty.ErrPowerballBuyNumber
	}

	if ball.PurInfos == nil {
		pblog.Debug("PowerballBuy records init")
		ball.PurInfos = make([]*pty.PurchaseInfo, 0, RedBalls)
	}

	newRecord := &pty.PowerPurchaseRecord{Amount: buy.GetAmount(), Number: buy.GetNumber(), Index: action.GetIndex()}
	pblog.Debug("PowerballBuy", "amount", buy.GetAmount(), "number", buy.GetNumber())

	/**********
	Once ExecTransfer succeed, ExecFrozen succeed, no roolback needed
	**********/

	receipt, err := action.coinsAccount.ExecTransfer(action.fromaddr, ball.CreateAddr, action.execaddr, buy.GetAmount()*ball.TicketPrice*decimal)
	if err != nil {
		pblog.Error("PowerballBuy.ExecTransfer", "addr", action.fromaddr, "execaddr", action.execaddr, "amount", buy.GetAmount())
		return nil, err
	}
	logs = append(logs, receipt.Logs...)
	kv = append(kv, receipt.KV...)

	receipt, err = action.coinsAccount.ExecFrozen(ball.CreateAddr, action.execaddr, buy.GetAmount()*ball.TicketPrice*decimal)
	if err != nil {
		pblog.Error("PowerballBuy.Frozen", "addr", ball.CreateAddr, "execaddr", action.execaddr, "amount", buy.GetAmount())
		return nil, err
	}
	logs = append(logs, receipt.Logs...)
	kv = append(kv, receipt.KV...)

	ball.SaleFund += buy.GetAmount() * ball.TicketPrice * decimal

	exist := false
	for _, info := range ball.PurInfos {
		if info.Addr == action.fromaddr {
			info.Records = append(info.Records, newRecord)
			info.AmountOneRound += buy.Amount
			exist = true
			break
		}
	}
	if !exist {
		initInfo := &pty.PurchaseInfo{}
		initInfo.Addr = action.fromaddr
		initInfo.Records = append(initInfo.Records, newRecord)
		initInfo.FundWin = 0
		initInfo.AmountOneRound = buy.Amount
		initInfo.PrizeOneRound = make([]int64, PrizeRange)
		ball.PurInfos = append(ball.PurInfos, initInfo)
		ball.TotalAddrNum++
	}
	ball.TotalPurchasedTxNum++

	ball.Save(action.db)
	kv = append(kv, ball.GetKVSet()...)

	receiptLog := action.GetBuyReceiptLog(&ball.Powerball, preStatus, ball.Round, buy.GetNumber(), buy.GetAmount())
	logs = append(logs, receiptLog)

	receipt = &types.Receipt{Ty: types.ExecOk, KV: kv, Logs: logs}
	return receipt, nil
}

// PowerballPause pause powerball
func (action *Action) PowerballPause(pause *pty.PowerballPause) (*types.Receipt, error) {
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue

	powerball, err := findPowerball(action.db, pause.PowerballID)
	if err != nil {
		pblog.Error("PowerballPause", "PowerballId", pause.PowerballID)
		return nil, err
	}

	ball := &PowerballDB{*powerball}
	preStatus := ball.Status
	if ball.Status != pty.PowerballPurchase {
		pblog.Error("PowerballPause", "ball.Status", ball.Status)
		return nil, pty.ErrPowerballStatus
	}

	if action.fromaddr != ball.GetCreateAddr() {
		pblog.Error("PowerballPause", "action.fromaddr", action.fromaddr)
		return nil, pty.ErrPowerballPauseActionInvalid
	}

	if types.IsPara() {
		mainHeight := action.powerball.GetMainHeight()
		if mainHeight < 0 {
			pblog.Error("PowerballPause", "mainHeight", mainHeight)
			return nil, pty.ErrPowerballStatus
		}
		if mainHeight-ball.GetLastTransToPurStateOnMain() < minPauseBlockNum {
			pblog.Error("PowerballPause", "action.height", action.height, "mainHeight", mainHeight,
				"GetLastTransToPurStateOnMain", ball.GetLastTransToPurState())
			return nil, pty.ErrPowerballStatus
		}
	} else {
		if action.height-ball.GetLastTransToPurState() < minPauseBlockNum {
			pblog.Error("PowerballPause", "action.height", action.height, "GetLastTransToPurState", ball.GetLastTransToPurState())
			return nil, pty.ErrPowerballStatus
		}
	}

	pblog.Debug("Powerball enter pause state", "PowerballId", pause.PowerballID)
	ball.Status = pty.PowerballPaused

	ball.Save(action.db)
	kv = append(kv, ball.GetKVSet()...)

	receiptLog := action.GetPauseReceiptLog(&ball.Powerball, preStatus)
	logs = append(logs, receiptLog)

	return &types.Receipt{Ty: types.ExecOk, KV: kv, Logs: logs}, nil
}

// PowerballDraw draw powerball
func (action *Action) PowerballDraw(draw *pty.PowerballDraw) (*types.Receipt, error) {
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	var receipt *types.Receipt

	powerball, err := findPowerball(action.db, draw.PowerballID)
	if err != nil {
		pblog.Error("PowerballBuy", "PowerballId", draw.PowerballID)
		return nil, err
	}

	ball := &PowerballDB{*powerball}
	preStatus := ball.Status
	if ball.Status != pty.PowerballPaused {
		pblog.Error("PowerballDraw", "ball.Status", ball.Status)
		return nil, pty.ErrPowerballStatus
	}

	if action.fromaddr != ball.GetCreateAddr() {
		pblog.Error("PowerballDraw", "action.fromaddr", action.fromaddr)
		return nil, pty.ErrPowerballDrawActionInvalid
	}

	rec, updateInfo, gainInfos, prizeInfo, err := action.checkDraw(ball)
	if err != nil {
		return nil, err
	}
	kv = append(kv, rec.KV...)
	logs = append(logs, rec.Logs...)

	ball.Save(action.db)
	kv = append(kv, ball.GetKVSet()...)

	receiptLog := action.GetDrawReceiptLog(&ball.Powerball, preStatus, ball.Round, ball.LuckyNumber, updateInfo, ball.TotalAddrNum, ball.SaleFund, gainInfos, prizeInfo)
	logs = append(logs, receiptLog)

	receipt = &types.Receipt{Ty: types.ExecOk, KV: kv, Logs: logs}
	return receipt, nil
}

// PowerballClose close powerball
func (action *Action) PowerballClose(draw *pty.PowerballClose) (*types.Receipt, error) {
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	//var receipt *types.Receipt

	if !isEnableToClose() {
		return nil, pty.ErrPowerballErrUnableClose
	}

	powerball, err := findPowerball(action.db, draw.PowerballID)
	if err != nil {
		pblog.Error("PowerballBuy", "PowerballId", draw.PowerballID)
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

	var totalReturn int64
	for _, item := range ball.PurInfos {
		totalReturn += item.AmountOneRound
	}
	pblog.Debug("PowerballClose", "totalReturn", totalReturn)

	if totalReturn > 0 {
		if !action.CheckExecAccount(ball.CreateAddr, decimal*totalReturn, true) {
			return nil, pty.ErrPowerballFundNotEnough
		}

		for _, info := range ball.PurInfos {
			if info.AmountOneRound > 0 {
				receipt, err := action.coinsAccount.ExecTransferFrozen(ball.CreateAddr, info.Addr, action.execaddr, decimal*info.AmountOneRound)
				if err != nil {
					return nil, err
				}

				kv = append(kv, receipt.KV...)
				logs = append(logs, receipt.Logs...)
			}
		}
	}

	pblog.Debug("Powerball enter close state")
	ball.Status = pty.PowerballClosed
	ball.PurInfos = nil
	ball.TotalPurchasedTxNum = 0
	ball.TotalAddrNum = 0

	ball.Save(action.db)
	kv = append(kv, ball.GetKVSet()...)

	receiptLog := action.GetCloseReceiptLog(&ball.Powerball, preStatus)
	logs = append(logs, receiptLog)

	return &types.Receipt{Ty: types.ExecOk, KV: kv, Logs: logs}, nil
}

func (action *Action) findLuckyNum(isSolo bool, ball *PowerballDB) *pty.BallNumber {
	var numStr []string
	if isSolo {
		//used for internal verfiy
		numStr = []string{"01", "02", "03", "04", "05", "06", "07"}
	} else {
		param := &types.ReqRandHash{
			ExecName: "ticket",
			BlockNum: blockNum,
			Hash:     action.powerball.GetLastHash(),
		}
		modify, err := action.powerball.GetExecutorAPI().GetRandNum(param)
		if err != nil {
			pblog.Error("findLuckyNum", "err", err)
			return nil
		}
		pblog.Info("findLuckyNum", "modify", common.ToHex(modify))

		seeds, err := genSeeds(modify, RedBalls+BlueBalls)
		if err != nil {
			pblog.Error("findLuckyNum", "err", err)
			return nil
		}
		pblog.Info("findLuckyNum", "seeds", seeds)

		redStr := genRandSet(RedRange, seeds[:RedBalls])
		blueStr := genRandSet(BlueRange, seeds[RedBalls:])
		numStr = append(numStr, redStr...)
		numStr = append(numStr, blueStr...)
	}
	return &pty.BallNumber{Balls: numStr}
}

func genSeeds(modify []byte, count int) ([]uint64, error) {
	step := 4
	seeds := make([]uint64, count)
	for i := 0; i < count; i++ {
		seed, err := strconv.ParseUint(common.ToHex(modify[i*step:(i+1)*step]), 0, 64)
		if err != nil {
			return nil, err
		}
		seeds[i] = seed
	}
	return seeds, nil
}

func genRandSet(total int, seeds []uint64) []string {
	set := make([]string, len(seeds))
	pool := make([]int, total)
	for i := 0; i < total; i++ {
		pool[i] = i + 1
	}
	for j := 0; j < len(seeds); j++ {
		seq := int(seeds[j] % uint64(total))
		set[j] = fmt.Sprintf("%02d", pool[seq])
		total--
		pool = append(pool[:seq], pool[seq+1:]...)
	}
	return set
}

// base on 6+1
func checkPrizeLevel(luckynum *pty.BallNumber, guessnum *pty.BallNumber) int {
	redScore := 0
	blueScore := 0

	for _, guess := range guessnum.Balls[:RedBalls] {
		for _, luck := range luckynum.Balls[:RedBalls] {
			if guess == luck {
				redScore++
			}
		}
	}
	if guessnum.Balls[RedBalls] == luckynum.Balls[RedBalls] {
		blueScore++
	}

	if redScore == RedBalls && blueScore == 1 {
		return First
	} else if redScore == RedBalls {
		return Second
	} else if redScore == RedBalls-1 && blueScore == 1 {
		return Third
	} else if redScore == RedBalls-1 || (redScore == RedBalls-2 && blueScore == 1) {
		return Fourth
	} else if redScore == RedBalls-2 || (redScore == RedBalls-3 && blueScore == 1) {
		return Fifth
	} else if redScore == RedBalls-3 || blueScore == 1 {
		return Sixth
	}
	return Zero
}

func (action *Action) checkDraw(ball *PowerballDB) (*types.Receipt, *pty.PowerballUpdateBuyInfo, *pty.PowerballGainInfos, []*pty.PowerballPrizeInfo, error) {
	luckynum := action.findLuckyNum(false, ball)
	if luckynum == nil {
		return nil, nil, nil, nil, pty.ErrPowerballErrLuckyNum
	}
	pblog.Info("checkDraw", "luckynum", luckynum.Balls)

	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	var updateInfo pty.PowerballUpdateBuyInfo
	var gainInfos pty.PowerballGainInfos
	var prizeInfo []*pty.PowerballPrizeInfo
	totalPrizeCnt := make([]int64, PrizeRange)

	for _, info := range ball.PurInfos {
		for _, rec := range info.Records {
			level := checkPrizeLevel(luckynum, rec.Number)
			pblog.Debug("checkDraw", "guessnum", rec.Number.Balls, "level", level, "amount", rec.Amount)
			info.PrizeOneRound[level] += rec.Amount
			totalPrizeCnt[level] += rec.Amount

			if level > 0 {
				newUpdateRec := &pty.PowerballUpdateRec{Index: rec.Index, Type: int32(level)}
				exist := false
				for _, update := range updateInfo.Updates {
					if update.Addr == info.Addr {
						update.Records = append(update.Records, newUpdateRec)
						exist = true
						break
					}
				}
				if !exist {
					newUpdate := &pty.PowerballUpdateRecs{Addr: info.Addr}
					newUpdate.Records = append(newUpdate.Records, newUpdateRec)
					updateInfo.Updates = append(updateInfo.Updates, newUpdate)
				}
			}
		}
	}
	pblog.Debug("checkDraw", "lenofupdate", len(updateInfo.Updates))

	//当期奖池包括：一定比例的当期销售额，累计奖池
	currentRatio := 1000 - NextRatio - ball.PlatformRatio - ball.DevelopRatio
	currentFund := ball.SaleFund*currentRatio/1000 + ball.AccuFund
	platformFund := ball.SaleFund * ball.PlatformRatio / 1000
	developFund := ball.SaleFund * ball.DevelopRatio / 1000

	sixthPrize := ball.TicketPrice * SixthRatio * decimal
	lowPrizeFund := totalPrizeCnt[Sixth] * sixthPrize
	//当期奖池不够支付六等奖的总奖金时，调整六等奖的数额
	if currentFund <= lowPrizeFund {
		pblog.Info("checkDraw need adjust sixthPrize", "currentFund", currentFund, "lowPrizeFund", lowPrizeFund, "sixthPrize", sixthPrize)
		for currentFund <= lowPrizeFund {
			//每次递减90%
			sixthPrize = sixthPrize * 9 / 10
			lowPrizeFund = totalPrizeCnt[Sixth] * sixthPrize
		}
		pblog.Info("checkDraw finish adjust sixthPrize", "sixthPrize", sixthPrize)
	}
	highPrizeFund := currentFund - lowPrizeFund
	pblog.Debug("checkDraw", "currentFund", currentFund, "lowPrizeFund", lowPrizeFund, "highPrizeFund", highPrizeFund)

	adjustPrizeCnt := make([]int64, PrizeRange)
	for i := First; i < Fifth+1; i++ {
		adjustPrizeCnt[i] = totalPrizeCnt[i]
		if adjustPrizeCnt[i] == 0 {
			adjustPrizeCnt[i] = 1
		}
	}
	firstPrize := highPrizeFund * FirstRatio / 1000 / adjustPrizeCnt[First]
	secondPrize := highPrizeFund * SecondRatio / 1000 / adjustPrizeCnt[Second]
	thirdPrize := highPrizeFund * ThirdRatio / 1000 / adjustPrizeCnt[Third]
	fourthPrize := highPrizeFund * FourthRatio / 1000 / adjustPrizeCnt[Fourth]
	fifthPrize := highPrizeFund * FifthRatio / 1000 / adjustPrizeCnt[Fifth]
	pblog.Debug("checkDraw", "firstPrize", firstPrize, "secondPrize", secondPrize, "thirdPrize", thirdPrize,
		"fourthPrize", fourthPrize, "fifthPrize", fifthPrize)

	prizeInfo = append(prizeInfo, &pty.PowerballPrizeInfo{Count: totalPrizeCnt[First], Amount: firstPrize})
	prizeInfo = append(prizeInfo, &pty.PowerballPrizeInfo{Count: totalPrizeCnt[Second], Amount: secondPrize})
	prizeInfo = append(prizeInfo, &pty.PowerballPrizeInfo{Count: totalPrizeCnt[Third], Amount: thirdPrize})
	prizeInfo = append(prizeInfo, &pty.PowerballPrizeInfo{Count: totalPrizeCnt[Fourth], Amount: fourthPrize})
	prizeInfo = append(prizeInfo, &pty.PowerballPrizeInfo{Count: totalPrizeCnt[Fifth], Amount: fifthPrize})
	prizeInfo = append(prizeInfo, &pty.PowerballPrizeInfo{Count: totalPrizeCnt[Sixth], Amount: sixthPrize})

	totalPrizeFund := int64(0)
	for _, info := range ball.PurInfos {
		info.FundWin = info.PrizeOneRound[First]*firstPrize + info.PrizeOneRound[Second]*secondPrize + info.PrizeOneRound[Third]*thirdPrize +
			info.PrizeOneRound[Fourth]*fourthPrize + info.PrizeOneRound[Fifth]*fifthPrize + info.PrizeOneRound[Sixth]*sixthPrize
		gain := &pty.PowerballGainInfo{Addr: info.Addr, BuyAmount: info.AmountOneRound, FundAmount: info.FundWin}
		gainInfos.Gains = append(gainInfos.Gains, gain)
		totalPrizeFund += info.FundWin
	}
	remainFund := currentFund - totalPrizeFund
	pblog.Debug("checkDraw", "round", ball.Round, "currentFund", currentFund, "totalPrizeFund", totalPrizeFund, "remainFund", remainFund)

	pblog.Debug("checkDraw transfer to platform", "platformFund", platformFund)
	receipt1, err := action.coinsAccount.ExecTransferFrozen(ball.CreateAddr, PlatformAddr, action.execaddr, platformFund)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	kv = append(kv, receipt1.KV...)
	logs = append(logs, receipt1.Logs...)

	pblog.Debug("checkDraw transfer to develop", "developFund", developFund)
	receipt2, err := action.coinsAccount.ExecTransferFrozen(ball.CreateAddr, DevelopAddr, action.execaddr, developFund)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	kv = append(kv, receipt2.KV...)
	logs = append(logs, receipt2.Logs...)

	for _, info := range ball.PurInfos {
		if info.FundWin > 0 {
			pblog.Debug("checkDraw pay bonus", "addr", info.Addr, "bonus", info.FundWin)
			receipt, err := action.coinsAccount.ExecTransferFrozen(ball.CreateAddr, info.Addr, action.execaddr, info.FundWin)
			if err != nil {
				return nil, nil, nil, nil, err
			}

			kv = append(kv, receipt.KV...)
			logs = append(logs, receipt.Logs...)
		}
	}

	pblog.Debug("checkDraw powerball enter draw state")
	ball.Status = pty.PowerballDrawed
	ball.PurInfos = nil
	ball.TotalPurchasedTxNum = 0
	ball.TotalAddrNum = 0
	ball.LastTransToDrawState = action.height
	ball.LuckyNumber = luckynum
	//累计奖池包括：一定比例的销售额，未中的高等级奖金
	ball.AccuFund = ball.SaleFund*NextRatio/1000 + remainFund
	ball.SaleFund = 0
	action.recordMissing(ball)

	if types.IsPara() {
		mainHeight := action.powerball.GetMainHeight()
		if mainHeight < 0 {
			pblog.Error("PowerballBuy", "mainHeight", mainHeight)
			return nil, nil, nil, nil, pty.ErrPowerballStatus
		}
		ball.LastTransToDrawStateOnMain = mainHeight
	}

	return &types.Receipt{Ty: types.ExecOk, KV: kv, Logs: logs}, &updateInfo, &gainInfos, prizeInfo, nil
}

func (action *Action) recordMissing(ball *PowerballDB) {
	for i := 0; i < RedRange; i++ {
		redStr := fmt.Sprintf("%02d", i+1)
		exist := false
		for _, luck := range ball.LuckyNumber.Balls[:RedBalls] {
			if luck == redStr {
				exist = true
				break
			}
		}
		if !exist {
			ball.MissingRecords[0].Times[i]++
		}
	}

	for i := 0; i < BlueRange; i++ {
		blueStr := fmt.Sprintf("%02d", i+1)
		if blueStr != ball.LuckyNumber.Balls[RedBalls] {
			ball.MissingRecords[1].Times[i]++
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
	}

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

func isEnableToClose() bool {
	return true
}

func findPowerball(db dbm.KV, powerballID string) (*pty.Powerball, error) {
	data, err := db.Get(Key(powerballID))
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

// CheckExecAccount method
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

// ListPowerballLuckyHistory method
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
	prefix = calcPowerballDrawPrefix(param.PowerballID)
	key = calcPowerballDrawKey(param.PowerballID, param.GetRound())

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

// ListPowerballBuyRecords method
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
	prefix = calcPowerballBuyPrefix(param.PowerballID, param.Addr)
	key = calcPowerballBuyKey(param.PowerballID, param.Addr, param.GetRound(), param.GetIndex())

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

// ListPowerballGainRecords for addr
func ListPowerballGainRecords(db dbm.Lister, stateDB dbm.KV, param *pty.ReqPowerballGainHistory) (types.Message, error) {
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
	prefix = calcPowerballGainPrefix(param.PowerballID, param.Addr)
	key = calcPowerballGainKey(param.PowerballID, param.Addr, param.GetRound())

	if param.GetRound() == 0 { //第一次查询
		values, err = db.List(prefix, nil, count, direction)
	} else {
		values, err = db.List(prefix, key, count, direction)
	}

	if err != nil {
		return nil, err
	}

	var records pty.PowerballGainRecords
	for _, value := range values {
		var record pty.PowerballGainRecord
		err := types.Decode(value, &record)
		if err != nil {
			continue
		}
		records.Records = append(records.Records, &record)
	}

	return &records, nil

}
