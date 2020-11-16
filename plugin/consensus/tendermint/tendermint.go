// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tendermint

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/33cn/chain33/common/crypto"
	dbm "github.com/33cn/chain33/common/db"
	"github.com/33cn/chain33/common/log/log15"
	"github.com/33cn/chain33/common/merkle"
	"github.com/33cn/chain33/queue"
	drivers "github.com/33cn/chain33/system/consensus"
	cty "github.com/33cn/chain33/system/dapp/coins/types"
	"github.com/33cn/chain33/types"
	"github.com/33cn/chain33/util"
	ttypes "github.com/33cn/plugin/plugin/consensus/tendermint/types"
	tmtypes "github.com/33cn/plugin/plugin/dapp/valnode/types"
	"github.com/golang/protobuf/proto"
)

const (
	tendermintVersion = "0.1.0"
)

var (
	tendermintlog               = log15.New("module", "tendermint")
	genesis                     string
	genesisAmount               int64 = 1e8
	genesisBlockTime            int64
	timeoutTxAvail              int32 = 1000
	timeoutPropose              int32 = 3000 // millisecond
	timeoutProposeDelta         int32 = 500
	timeoutPrevote              int32 = 1000
	timeoutPrevoteDelta         int32 = 500
	timeoutPrecommit            int32 = 1000
	timeoutPrecommitDelta       int32 = 500
	timeoutCommit               int32 = 1000
	skipTimeoutCommit                 = false
	createEmptyBlocks                 = false
	fastSync                          = false
	preExec                           = false
	createEmptyBlocksInterval   int32 // second
	validatorNodes                    = []string{"127.0.0.1:46656"}
	peerGossipSleepDuration     int32 = 200
	peerQueryMaj23SleepDuration int32 = 2000
	zeroHash                    [32]byte
	random                      *rand.Rand
	signName                          = "ed25519"
	useAggSig                         = false
	multiBlocks                 int64 = 1
	timeoutChangeProposer       int64 = 5000
)

func init() {
	drivers.Reg("tendermint", New)
	drivers.QueryData.Register("tendermint", &Client{})
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// Client Tendermint implementation
type Client struct {
	//config
	*drivers.BaseClient
	genesisDoc    *ttypes.GenesisDoc // initial validator set
	privValidator ttypes.PrivValidator
	privKey       crypto.PrivKey // local node's p2p key
	pubKey        string
	csState       *ConsensusState
	csStore       *ConsensusStore // save consensus state
	node          *Node
	txsAvailable  chan int64
	stopC         chan struct{}
}

type subConfig struct {
	Genesis                   string   `json:"genesis"`
	GenesisAmount             int64    `json:"genesisAmount"`
	GenesisBlockTime          int64    `json:"genesisBlockTime"`
	TimeoutTxAvail            int32    `json:"timeoutTxAvail"`
	TimeoutPropose            int32    `json:"timeoutPropose"`
	TimeoutProposeDelta       int32    `json:"timeoutProposeDelta"`
	TimeoutPrevote            int32    `json:"timeoutPrevote"`
	TimeoutPrevoteDelta       int32    `json:"timeoutPrevoteDelta"`
	TimeoutPrecommit          int32    `json:"timeoutPrecommit"`
	TimeoutPrecommitDelta     int32    `json:"timeoutPrecommitDelta"`
	TimeoutCommit             int32    `json:"timeoutCommit"`
	SkipTimeoutCommit         bool     `json:"skipTimeoutCommit"`
	CreateEmptyBlocks         bool     `json:"createEmptyBlocks"`
	CreateEmptyBlocksInterval int32    `json:"createEmptyBlocksInterval"`
	ValidatorNodes            []string `json:"validatorNodes"`
	FastSync                  bool     `json:"fastSync"`
	PreExec                   bool     `json:"preExec"`
	SignName                  string   `json:"signName"`
	UseAggregateSignature     bool     `json:"useAggregateSignature"`
	MultiBlocks               int64    `json:"multiBlocks"`
	TimeoutChangeProposer     int64    `json:"timeoutChangeProposer"`
}

func applyConfig(sub []byte) {
	var subcfg subConfig
	if sub != nil {
		types.MustDecode(sub, &subcfg)
	}
	if subcfg.Genesis != "" {
		genesis = subcfg.Genesis
	}
	if subcfg.GenesisAmount > 0 {
		genesisAmount = subcfg.GenesisAmount
	}
	if subcfg.GenesisBlockTime > 0 {
		genesisBlockTime = subcfg.GenesisBlockTime
	}
	if subcfg.TimeoutTxAvail > 0 {
		timeoutTxAvail = subcfg.TimeoutTxAvail
	}
	if subcfg.TimeoutPropose > 0 {
		timeoutPropose = subcfg.TimeoutPropose
	}
	if subcfg.TimeoutProposeDelta > 0 {
		timeoutProposeDelta = subcfg.TimeoutProposeDelta
	}
	if subcfg.TimeoutPrevote > 0 {
		timeoutPrevote = subcfg.TimeoutPrevote
	}
	if subcfg.TimeoutPrevoteDelta > 0 {
		timeoutPrevoteDelta = subcfg.TimeoutPrevoteDelta
	}
	if subcfg.TimeoutPrecommit > 0 {
		timeoutPrecommit = subcfg.TimeoutPrecommit
	}
	if subcfg.TimeoutPrecommitDelta > 0 {
		timeoutPrecommitDelta = subcfg.TimeoutPrecommitDelta
	}
	if subcfg.TimeoutCommit > 0 {
		timeoutCommit = subcfg.TimeoutCommit
	}
	skipTimeoutCommit = subcfg.SkipTimeoutCommit
	createEmptyBlocks = subcfg.CreateEmptyBlocks
	if subcfg.CreateEmptyBlocksInterval > 0 {
		createEmptyBlocksInterval = subcfg.CreateEmptyBlocksInterval
	}
	if len(subcfg.ValidatorNodes) > 0 {
		validatorNodes = subcfg.ValidatorNodes
	}
	fastSync = subcfg.FastSync
	preExec = subcfg.PreExec
	if subcfg.SignName != "" {
		signName = subcfg.SignName
	}
	useAggSig = subcfg.UseAggregateSignature
	if subcfg.MultiBlocks > 0 {
		multiBlocks = subcfg.MultiBlocks
	}
	if subcfg.TimeoutChangeProposer > 0 {
		timeoutChangeProposer = subcfg.TimeoutChangeProposer
	}
}

// DefaultDBProvider returns a database using the DBBackend and DBDir
// specified in the ctx.Config.
func DefaultDBProvider(name string) dbm.DB {
	return dbm.NewDB(name, "leveldb", fmt.Sprintf("datadir%stendermint", string(os.PathSeparator)), 0)
}

// New ...
func New(cfg *types.Consensus, sub []byte) queue.Module {
	tendermintlog.Info("Start to create tendermint client")
	applyConfig(sub)
	//init rand
	ttypes.Init()

	signType, ok := ttypes.SignMap[signName]
	if !ok {
		tendermintlog.Error("Invalid sign name")
		return nil
	}

	ttypes.CryptoName = types.GetSignName("", signType)
	cr, err := crypto.New(ttypes.CryptoName)
	if err != nil {
		tendermintlog.Error("NewTendermintClient", "err", err)
		return nil
	}
	ttypes.ConsensusCrypto = cr

	if useAggSig {
		_, err = crypto.ToAggregate(ttypes.ConsensusCrypto)
		if err != nil {
			tendermintlog.Error("ConsensusCrypto not support aggregate signature", "name", ttypes.CryptoName)
			return nil
		}
	}

	genDoc, err := ttypes.GenesisDocFromFile("genesis.json")
	if err != nil {
		tendermintlog.Error("NewTendermintClient", "msg", "GenesisDocFromFile fail", "error", err)
		return nil
	}

	privValidator := ttypes.LoadOrGenPrivValidatorFS("priv_validator.json")
	if privValidator == nil {
		tendermintlog.Error("NewTendermintClient create priv_validator file fail")
		return nil
	}

	ttypes.InitMessageMap()

	priv := privValidator.PrivKey
	pubkey := privValidator.GetPubKey().KeyString()
	c := drivers.NewBaseClient(cfg)
	client := &Client{
		BaseClient:    c,
		genesisDoc:    genDoc,
		privValidator: privValidator,
		privKey:       priv,
		pubKey:        pubkey,
		csStore:       NewConsensusStore(),
		txsAvailable:  make(chan int64, 1),
		stopC:         make(chan struct{}, 1),
	}
	c.SetChild(client)
	return client
}

// GenesisDoc returns the Node's GenesisDoc.
func (client *Client) GenesisDoc() *ttypes.GenesisDoc {
	return client.genesisDoc
}

// GenesisState returns the Node's GenesisState.
func (client *Client) GenesisState() *State {
	state, err := MakeGenesisState(client.genesisDoc)
	if err != nil {
		tendermintlog.Error("GenesisState", "err", err)
		return nil
	}
	return &state
}

// Close TODO:may need optimize
func (client *Client) Close() {
	client.BaseClient.Close()
	client.node.Stop()
	client.stopC <- struct{}{}
	tendermintlog.Info("consensus tendermint closed")
}

// SetQueueClient ...
func (client *Client) SetQueueClient(q queue.Client) {
	client.InitClient(q, func() {
		//call init block
		client.InitBlock()
	})

	go client.EventLoop()
	go client.StartConsensus()
}

// StartConsensus a routine that make the consensus start
func (client *Client) StartConsensus() {
	//进入共识前先同步到最大高度
	hint := time.NewTicker(5 * time.Second)
	beg := time.Now()
OuterLoop:
	for fastSync {
		select {
		case <-hint.C:
			tendermintlog.Info("Still catching up max height......", "Height", client.GetCurrentHeight(), "cost", time.Since(beg))
		default:
			if client.IsCaughtUp() {
				tendermintlog.Info("This node has caught up max height")
				break OuterLoop
			}
			time.Sleep(time.Second)
		}
	}
	hint.Stop()

	// load state
	var state State
	if client.GetCurrentHeight() == 0 {
		genState := client.GenesisState()
		if genState == nil {
			panic("StartConsensus GenesisState fail")
		}
		state = genState.Copy()
	} else if client.GetConsensusHeight() <= client.csStore.LoadStateHeight() {
		stoState := client.csStore.LoadStateFromStore()
		if stoState == nil {
			panic("StartConsensus LoadStateFromStore fail")
		}
		state = LoadState(stoState)
		tendermintlog.Info("Load state from store")
	} else {
		height := client.GetConsensusHeight()
		blkState := client.LoadBlockState(height)
		if blkState == nil {
			panic("StartConsensus LoadBlockState fail")
		}
		state = LoadState(blkState)
		tendermintlog.Info("Load state from block")
		//save initial state in store
		blkCommit := client.LoadBlockCommit(height)
		if blkCommit == nil {
			panic("StartConsensus LoadBlockCommit fail")
		}
		err := client.csStore.SaveConsensusState(height-1, blkState, blkCommit)
		if err != nil {
			panic(fmt.Sprintf("StartConsensus SaveConsensusState fail: %v", err))
		}
		tendermintlog.Info("Save state from block")
	}
	tendermintlog.Debug("Load state finish", "state", state)

	// start
	tendermintlog.Info("StartConsensus",
		"privValidator", fmt.Sprintf("%X", ttypes.Fingerprint(client.privValidator.GetAddress())),
		"Validators", state.Validators.String())
	// Log whether this node is a validator or an observer
	if state.Validators.HasAddress(client.privValidator.GetAddress()) {
		tendermintlog.Info("This node is a validator")
	} else {
		tendermintlog.Info("This node is not a validator")
	}

	stateDB := NewStateDB(client, state)

	// make block executor for consensus and blockchain reactors to execute blocks
	blockExec := NewBlockExecutor(stateDB)

	// Make ConsensusReactor
	csState := NewConsensusState(client, state, blockExec)
	// reset height, round, state begin at newheigt,0,0
	client.privValidator.ResetLastHeight(state.LastHeight)
	csState.SetPrivValidator(client.privValidator)

	client.csState = csState

	// Create & add listener
	protocol, listeningAddress := "tcp", "0.0.0.0:46656"
	node := NewNode(validatorNodes, protocol, listeningAddress, client.privKey, state.ChainID, tendermintVersion, csState)

	client.node = node
	node.Start()

	go client.CreateBlock()
}

// GetGenesisBlockTime ...
func (client *Client) GetGenesisBlockTime() int64 {
	return genesisBlockTime
}

// CreateGenesisTx ...
func (client *Client) CreateGenesisTx() (ret []*types.Transaction) {
	var tx types.Transaction
	tx.Execer = []byte("coins")
	tx.To = genesis
	//gen payload
	g := &cty.CoinsAction_Genesis{}
	g.Genesis = &types.AssetsGenesis{}
	g.Genesis.Amount = genesisAmount * types.Coin
	tx.Payload = types.Encode(&cty.CoinsAction{Value: g, Ty: cty.CoinsActionGenesis})
	ret = append(ret, &tx)
	return
}

func (client *Client) GetConsensusHeight() int64 {
	if multiBlocks > 1 {
		info, err := client.getLastBlockInfo()
		if err != nil {
			return -1
		}
		return info.Block.Header.Height
	}
	return client.GetCurrentHeight()
}

func (client *Client) getBlockInfoTx(current *types.Block) (*tmtypes.ValNodeAction, error) {
	//检查第一笔交易
	if len(current.Txs) == 0 {
		return nil, types.ErrEmptyTx
	}
	baseTx := current.Txs[0]

	var valAction tmtypes.ValNodeAction
	err := types.Decode(baseTx.GetPayload(), &valAction)
	if err != nil {
		return nil, err
	}
	//检查交易类型
	if valAction.GetTy() != tmtypes.ValNodeActionBlockInfo {
		return nil, ttypes.ErrBaseTxType
	}
	//检查交易内容
	if valAction.GetBlockInfo() == nil {
		return nil, ttypes.ErrBlockInfoTx
	}
	return &valAction, nil
}

// CheckBlock 检查区块
func (client *Client) CheckBlock(parent *types.Block, current *types.BlockDetail) error {
	cfg := client.GetAPI().GetConfig()
	if current.Block.Difficulty != cfg.GetP(0).PowLimitBits {
		return types.ErrBlockHeaderDifficulty
	}
	if parent.Height+1 != current.Block.Height {
		return types.ErrBlockHeight
	}
	//判断exec 是否成功
	if current.Receipts[0].Ty != types.ExecOk {
		return ttypes.ErrBaseExecErr
	}
	valAction, err := client.getBlockInfoTx(current.Block)
	if err != nil {
		return err
	}
	info := valAction.GetBlockInfo()
	if current.Block.Height != info.Block.Header.BlockHeight {
		return ttypes.ErrBlockInfoHeight
	}
	if current.Block.Height > 1 {
		lastValAction, err := client.getBlockInfoTx(parent)
		if err != nil {
			return err
		}
		lastInfo := lastValAction.GetBlockInfo()
		if multiBlocks > 1 {
			if lastInfo.Block.Header.Height == info.Block.Header.Height {
				if lastInfo.Block.Header.BlockSequence+1 != info.Block.Header.BlockSequence {
					return ttypes.ErrBlockSequence
				} else {
					return nil
				}
			}
		}
		//切换提议节点进行检查
		lastProposalBlock := &ttypes.TendermintBlock{TendermintBlock: lastInfo.GetBlock()}
		if !lastProposalBlock.HashesTo(info.Block.Header.LastBlockID.Hash) {
			return ttypes.ErrLastBlockID
		}
	}
	return nil
}

// ProcEvent reply not support action err
func (client *Client) ProcEvent(msg *queue.Message) bool {
	msg.ReplyErr("Client", types.ErrActionNotSupport)
	return true
}

// CreateBlock a routine monitor whether some transactions available and tell client by available channel
func (client *Client) CreateBlock() {
	cfg := client.GetQueueClient().GetConfig()
	for {
		if client.IsClosed() {
			tendermintlog.Info("CreateBlock quit")
			break
		}
		if !client.csState.IsRunning() {
			tendermintlog.Info("consensus not running")
			time.Sleep(500 * time.Millisecond)
			continue
		}

		height, err := client.getLastHeight()
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if !client.CheckTxsAvailable(height) {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if height > 0 && multiBlocks > 1 {
			info, err := client.getLastBlockInfo()
			if err != nil {
				continue
			}
			csHeight := info.Block.Header.Height
			seq := info.Block.Header.BlockSequence
			proposerAddr := info.Block.Header.ProposerAddr
			if seq+1 < multiBlocks {
				if bytes.Equal(proposerAddr, client.privValidator.GetAddress()) {
					block := client.BuildBlock()
					tmtBlock := info.Block
					tmtBlock.Header.BlockHeight = block.Height
					tmtBlock.Header.BlockSequence = seq + 1
					baseTx := CreateBlockInfoTx(client.pubKey, info.State, tmtBlock)
					block.Txs[0] = baseTx
					if cfg.IsFork(block.Height, "ForkRootHash") {
						block.Txs = types.TransactionSort(block.Txs)
					}
					block.TxHash = merkle.CalcMerkleRoot(cfg, block.Height, block.Txs)
					err = client.WriteBlock(nil, block)
					if err != nil {
						tendermintlog.Error("WriteBlock fail", "err", err)
					}
					time.Sleep(time.Duration(timeoutTxAvail) * time.Millisecond)
					continue
				} else {
					if client.WaitBlockSequence(csHeight, seq+1, timeoutChangeProposer) {
						continue
					}
					tendermintlog.Info("Will change proposer", "height", csHeight+1)
				}
			}
			//client.WaitMajorPeerHeight(height)
			height = csHeight
		}

		client.txsAvailable <- height + 1
		time.Sleep(time.Duration(timeoutTxAvail) * time.Millisecond)
	}
}

func (client *Client) getLastHeight() (int64, error) {
	lastBlock, err := client.RequestLastBlock()
	if err != nil {
		return -1, err
	}
	return lastBlock.Height, nil
}

func (client *Client) getLastBlockInfo() (*tmtypes.TendermintBlockInfo, error) {
	lastBlock, err := client.RequestLastBlock()
	if err != nil {
		return nil, err
	}
	if lastBlock.Height == 0 {
		return nil, nil
	}
	valAction, err := client.getBlockInfoTx(lastBlock)
	if err != nil {
		return nil, err
	}
	return valAction.GetBlockInfo(), nil
}

// WaitBlock by height
func (client *Client) WaitBlockSequence(height int64, seq int64, interval int64) bool {
	retry := int64(0)
	newHeight := int64(0)
	newSeq := int64(0)
	for {
		info, err := client.getLastBlockInfo()
		if err == nil {
			newHeight = info.Block.Header.Height
			newSeq = info.Block.Header.BlockSequence
			if newHeight == height && newSeq >= seq {
				return true
			}
		}
		retry++
		time.Sleep(100 * time.Millisecond)
		if retry >= interval/100 {
			tendermintlog.Error("Wait block sequence fail", "height", height, "seq", seq,
				"currentHeight", newHeight, "currentSeq", newSeq)
			return false
		}
	}
}

// WaitMajorPeerHeight wait majority peer height
func (client *Client) WaitMajorPeerHeight(height int64) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	beg := time.Now()
OuterLoop:
	for {
		select {
		case <-ticker.C:
			tendermintlog.Info("Still waiting majority peers......", "height", height, "cost", time.Since(beg))
		default:
			peerlist, err := client.fetchPeerList()
			if err == nil {
				total := len(peerlist.Peers)
				count := 0
				for _, peer := range peerlist.Peers {
					if peer == nil {
						continue
					} else if peer.Header.Height >= height {
						count++
						if count >= total*2/3+1 {
							tendermintlog.Info("Majority peers catch up", "height", height)
							break OuterLoop
						}
					}
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (client *Client) fetchPeerList() (*types.PeerList, error) {
	msg := client.GetQueueClient().NewMessage("p2p", types.EventPeerInfo, nil)
	err := client.GetQueueClient().SendTimeout(msg, true, 5*time.Second)
	if err != nil {
		tendermintlog.Error("fetchPeerList", "client.Send err:", err)
		return nil, err
	}
	resp, err := client.GetQueueClient().WaitTimeout(msg, 10*time.Second)
	if err != nil {
		tendermintlog.Error("fetchPeerList", "client.Wait err:", err)
		return nil, err
	}
	peerlist := resp.GetData().(*types.PeerList)
	if peerlist == nil {
		return nil, types.ErrNoPeer
	}
	return peerlist, nil
}

// TxsAvailable check available channel
func (client *Client) TxsAvailable() <-chan int64 {
	return client.txsAvailable
}

// StopC stop client
func (client *Client) StopC() <-chan struct{} {
	return client.stopC
}

// GetMempoolSize get tx num in mempool
func (client *Client) GetMempoolSize() int64 {
	msg := client.GetQueueClient().NewMessage("mempool", types.EventGetMempoolSize, nil)
	err := client.GetQueueClient().Send(msg, true)
	if err != nil {
		tendermintlog.Error("GetMempoolSize send", "err", err)
		return 0
	}
	resp, err := client.GetQueueClient().Wait(msg)
	if err != nil {
		tendermintlog.Error("GetMempoolSize result", "err", err)
		return 0
	}
	return resp.GetData().(*types.MempoolSize).GetSize()
}

// CheckTxsAvailable check whether some new transactions arriving
func (client *Client) CheckTxsAvailable(height int64) bool {
	num := client.GetMempoolSize()
	if num == 0 {
		return false
	}
	txs := client.RequestTx(int(num), nil)
	txs = client.CheckTxDup(txs, height)
	return len(txs) != 0
}

// CheckTxDup check transactions that duplicate
func (client *Client) CheckTxDup(txs []*types.Transaction, height int64) (transactions []*types.Transaction) {
	cacheTxs := types.TxsToCache(txs)
	var err error
	cacheTxs, err = util.CheckTxDup(client.GetQueueClient(), cacheTxs, height)
	if err != nil {
		return txs
	}
	return types.CacheToTxs(cacheTxs)
}

// BuildBlock build a new block
func (client *Client) BuildBlock() *types.Block {
	lastBlock, err := client.RequestLastBlock()
	if err != nil {
		tendermintlog.Error("BuildBlock fail", "err", err)
		return nil
	}
	cfg := client.GetAPI().GetConfig()
	txs := client.RequestTx(int(cfg.GetP(lastBlock.Height+1).MaxTxNumber)-1, nil)
	// placeholder
	tx0 := &types.Transaction{}
	txs = append([]*types.Transaction{tx0}, txs...)

	var newblock types.Block
	newblock.ParentHash = lastBlock.Hash(cfg)
	newblock.Height = lastBlock.Height + 1
	client.AddTxsToBlock(&newblock, txs)
	//固定难度
	newblock.Difficulty = cfg.GetP(0).PowLimitBits
	newblock.BlockTime = types.Now().Unix()
	if lastBlock.BlockTime >= newblock.BlockTime {
		newblock.BlockTime = lastBlock.BlockTime + 1
	}
	return &newblock
}

// CommitBlock call WriteBlock to commit to chain
func (client *Client) CommitBlock(block *types.Block) error {
	cfg := client.GetAPI().GetConfig()
	retry := 0
	for {
		retErr := client.WriteBlock(nil, block)
		if retErr == nil {
			return nil
		}
		tendermintlog.Info("Write block fail", "tries", retry, "err", retErr)
		if client.WaitBlock(block.Height) {
			if !preExec {
				return nil
			}
			curBlock, err := client.RequestBlock(block.Height)
			if err == nil {
				if bytes.Equal(curBlock.Hash(cfg), block.Hash(cfg)) {
					tendermintlog.Info("already has block")
					return nil
				}
				tendermintlog.Info("block is different", "block", block, "curBlock", curBlock)
				if bytes.Equal(curBlock.Txs[0].Hash(), block.Txs[0].Hash()) {
					tendermintlog.Warn("base tx is same, origin maybe same")
					return nil
				}
			}
		}
		retry++
		if retry >= 5 {
			tendermintlog.Error("Commit block fail", "height", block.Height)
			return retErr
		}
	}
}

// WaitBlock by height
func (client *Client) WaitBlock(height int64) bool {
	retry := 0
	for {
		newHeight, err := client.getLastHeight()
		if err == nil && newHeight >= height {
			return true
		}
		retry++
		time.Sleep(200 * time.Millisecond)
		if retry >= 10 {
			tendermintlog.Info("Wait block fail", "height", height, "CurrentHeight", newHeight)
			return false
		}
	}
}

// QueryValidatorsByHeight ...
func (client *Client) QueryValidatorsByHeight(height int64) (*tmtypes.ValNodes, error) {
	if height < 1 {
		return nil, ttypes.ErrHeightLessThanOne
	}
	req := &tmtypes.ReqNodeInfo{Height: height}
	param, err := proto.Marshal(req)
	if err != nil {
		tendermintlog.Error("QueryValidatorsByHeight marshal", "err", err)
		return nil, types.ErrInvalidParam
	}
	msg := client.GetQueueClient().NewMessage("execs", types.EventBlockChainQuery,
		&types.ChainExecutor{Driver: "valnode", FuncName: "GetValNodeByHeight", StateHash: zeroHash[:], Param: param})
	err = client.GetQueueClient().SendTimeout(msg, true, 5*time.Second)
	if err != nil {
		tendermintlog.Error("QueryValidatorsByHeight send", "err", err)
		return nil, err
	}
	msg, err = client.GetQueueClient().WaitTimeout(msg, 10*time.Second)
	if err != nil {
		tendermintlog.Info("QueryValidatorsByHeight result", "err", err)
		return nil, err
	}
	return msg.GetData().(types.Message).(*tmtypes.ValNodes), nil
}

// QueryBlockInfoByHeight get blockInfo and block by height
func (client *Client) QueryBlockInfoByHeight(height int64) (*tmtypes.TendermintBlockInfo, *types.Block, error) {
	if height < 1 {
		return nil, nil, ttypes.ErrHeightLessThanOne
	}
	if multiBlocks > 1 {
		req := &tmtypes.ReqBlockInfo{Height: height}
		param, err := proto.Marshal(req)
		if err != nil {
			tendermintlog.Error("QueryBlockInfoByHeight marshal", "err", err)
			return nil, nil, types.ErrInvalidParam
		}
		msg := client.GetQueueClient().NewMessage("execs", types.EventBlockChainQuery,
			&types.ChainExecutor{Driver: "valnode", FuncName: "GetBlockInfoByHeight", StateHash: zeroHash[:], Param: param})
		err = client.GetQueueClient().SendTimeout(msg, true, 5*time.Second)
		if err != nil {
			tendermintlog.Error("QueryBlockInfoByHeight send", "err", err)
			return nil, nil, err
		}
		msg, err = client.GetQueueClient().WaitTimeout(msg, 10*time.Second)
		if err != nil {
			tendermintlog.Info("QueryBlockInfoByHeight result", "err", err)
			return nil, nil, err
		}
		info := msg.GetData().(types.Message).(*tmtypes.TendermintBlockInfo)
		height = info.Block.Header.BlockHeight
	}
	block, err := client.RequestBlock(height)
	if err != nil {
		return nil, nil, err
	}
	valAction, err := client.getBlockInfoTx(block)
	if err != nil {
		return nil, nil, err
	}
	return valAction.GetBlockInfo(), block, nil
}

// LoadBlockCommit by height
func (client *Client) LoadBlockCommit(height int64) *tmtypes.TendermintCommit {
	blockInfo, _, err := client.QueryBlockInfoByHeight(height)
	if err != nil {
		tendermintlog.Error("LoadBlockCommit GetBlockInfo fail", "err", err)
		return nil
	}
	return blockInfo.GetBlock().GetLastCommit()
}

// LoadBlockState by height
func (client *Client) LoadBlockState(height int64) *tmtypes.State {
	blockInfo, _, err := client.QueryBlockInfoByHeight(height)
	if err != nil {
		tendermintlog.Error("LoadBlockState GetBlockInfo fail", "err", err)
		return nil
	}
	return blockInfo.GetState()
}

// LoadProposalBlock by height
func (client *Client) LoadProposalBlock(height int64) *tmtypes.TendermintBlock {
	blockInfo, block, err := client.QueryBlockInfoByHeight(height)
	if err != nil {
		tendermintlog.Error("LoadProposal GetBlockInfo fail", "err", err)
		return nil
	}
	proposalBlock := blockInfo.GetBlock()
	proposalBlock.Data = block
	return proposalBlock
}

// Query_IsHealthy query whether consensus is sync
func (client *Client) Query_IsHealthy(req *types.ReqNil) (types.Message, error) {
	isHealthy := false
	if client.IsCaughtUp() && client.GetCurrentHeight() <= client.csState.GetRoundState().Height+1 {
		isHealthy = true
	}
	return &tmtypes.IsHealthy{IsHealthy: isHealthy}, nil
}

// Query_NodeInfo query validator node info
func (client *Client) Query_NodeInfo(req *types.ReqNil) (types.Message, error) {
	nodes := client.csState.GetRoundState().Validators.Validators
	validators := make([]*tmtypes.Validator, 0)
	for _, node := range nodes {
		if node == nil {
			validators = append(validators, &tmtypes.Validator{})
		} else {
			item := &tmtypes.Validator{
				Address:     node.Address,
				PubKey:      node.PubKey,
				VotingPower: node.VotingPower,
				Accum:       node.Accum,
			}
			validators = append(validators, item)
		}
	}
	return &tmtypes.ValidatorSet{Validators: validators, Proposer: &tmtypes.Validator{}}, nil
}

// CmpBestBlock 比较newBlock是不是最优区块
func (client *Client) CmpBestBlock(newBlock *types.Block, cmpBlock *types.Block) bool {
	return false
}
