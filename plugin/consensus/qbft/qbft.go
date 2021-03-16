// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package qbft

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"sync/atomic"
	"time"

	"github.com/33cn/chain33/common/crypto"
	dbm "github.com/33cn/chain33/common/db"
	"github.com/33cn/chain33/common/log/log15"
	"github.com/33cn/chain33/queue"
	drivers "github.com/33cn/chain33/system/consensus"
	cty "github.com/33cn/chain33/system/dapp/coins/types"
	"github.com/33cn/chain33/types"
	"github.com/33cn/chain33/util"
	ttypes "github.com/33cn/plugin/plugin/consensus/qbft/types"
	tmtypes "github.com/33cn/plugin/plugin/dapp/qbftNode/types"
	"github.com/golang/protobuf/proto"
)

const (
	qbftVersion = "0.1.0"
)

var (
	qbftlog                     = log15.New("module", "qbft")
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
	peerGossipSleepDuration     int32 = 100
	peerQueryMaj23SleepDuration int32 = 2000
	zeroHash                    [32]byte
	random                      *rand.Rand
	signName                    = "ed25519"
	useAggSig                   = false
	gossipVotes                 atomic.Value
	multiBlocks                 int64 = 1
)

func init() {
	drivers.Reg("qbft", New)
	drivers.QueryData.Register("qbft", &Client{})
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// Client qbft implementation
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
	gossipVotes.Store(true)
	if subcfg.MultiBlocks > 0 {
		multiBlocks = subcfg.MultiBlocks
	}
}

// DefaultDBProvider returns a database using the DBBackend and DBDir
// specified in the ctx.Config.
func DefaultDBProvider(name string) dbm.DB {
	return dbm.NewDB(name, "leveldb", fmt.Sprintf("datadir%sqbft", string(os.PathSeparator)), 0)
}

// New ...
func New(cfg *types.Consensus, sub []byte) queue.Module {
	qbftlog.Info("Start to create qbft client")
	applyConfig(sub)
	//init rand
	ttypes.Init()

	signType, ok := ttypes.SignMap[signName]
	if !ok {
		qbftlog.Error("Invalid sign name")
		return nil
	}

	ttypes.CryptoName = types.GetSignName("", signType)
	cr, err := crypto.New(ttypes.CryptoName)
	if err != nil {
		qbftlog.Error("NewQbftClient", "err", err)
		return nil
	}
	ttypes.ConsensusCrypto = cr

	if useAggSig {
		_, err = crypto.ToAggregate(ttypes.ConsensusCrypto)
		if err != nil {
			qbftlog.Error("ConsensusCrypto not support aggregate signature", "name", ttypes.CryptoName)
			return nil
		}
	}

	genDoc, err := ttypes.GenesisDocFromFile("genesis.json")
	if err != nil {
		qbftlog.Error("NewQbftClient", "msg", "GenesisDocFromFile fail", "error", err)
		return nil
	}

	privValidator := ttypes.LoadOrGenPrivValidatorFS("priv_validator.json")
	if privValidator == nil {
		qbftlog.Error("NewQbftClient create priv_validator file fail")
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
		qbftlog.Error("GenesisState", "err", err)
		return nil
	}
	return &state
}

// Close TODO:may need optimize
func (client *Client) Close() {
	client.BaseClient.Close()
	client.node.Stop()
	client.stopC <- struct{}{}
	qbftlog.Info("consensus qbft closed")
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
	defer hint.Stop()

	beg := time.Now()
OuterLoop:
	for fastSync {
		if client.IsClosed() {
			qbftlog.Info("StartConsensus quit")
			return
		}
		select {
		case <-hint.C:
			qbftlog.Info("Still catching up max height......", "Height", client.GetCurrentHeight(), "cost", time.Since(beg))
		default:
			if client.IsCaughtUp() {
				qbftlog.Info("This node has caught up max height")
				break OuterLoop
			}
			time.Sleep(time.Second)
		}
	}

	// load state
	var state State
	if client.GetCurrentHeight() == 0 {
		genState := client.GenesisState()
		if genState == nil {
			panic("StartConsensus GenesisState fail")
		}
		state = genState.Copy()
	} else if client.GetCurrentHeight() <= client.csStore.LoadStateHeight() {
		stoState := client.csStore.LoadStateFromStore()
		if stoState == nil {
			panic("StartConsensus LoadStateFromStore fail")
		}
		state = LoadState(stoState)
		qbftlog.Info("Load state from store")
	} else {
		height := client.GetCurrentHeight()
		blkState := client.LoadBlockState(height)
		if blkState == nil {
			panic("StartConsensus LoadBlockState fail")
		}
		state = LoadState(blkState)
		qbftlog.Info("Load state from block")
		//save initial state in store
		blkCommit := client.LoadBlockCommit(height)
		if blkCommit == nil {
			panic("StartConsensus LoadBlockCommit fail")
		}
		err := client.csStore.SaveConsensusState(height-1, blkState, blkCommit)
		if err != nil {
			panic(fmt.Sprintf("StartConsensus SaveConsensusState fail: %v", err))
		}
		qbftlog.Info("Save state from block")
	}

	// start
	qbftlog.Info("StartConsensus",
		"privValidator", fmt.Sprintf("%X", ttypes.Fingerprint(client.privValidator.GetAddress())),
		"state", state)
	// Log whether this node is a validator or an observer
	if state.Validators.HasAddress(client.privValidator.GetAddress()) {
		qbftlog.Info("This node is a validator")
	} else {
		qbftlog.Info("This node is not a validator")
	}

	stateDB := NewStateDB(client, state)

	// make block executor for consensus and blockchain reactors to execute blocks
	blockExec := NewBlockExecutor(stateDB)

	// Make ConsensusReactor
	csState := NewConsensusState(client, state, blockExec)
	// reset height, round, state begin at newheigt,0,0
	client.privValidator.ResetLastHeight(state.LastBlockHeight)
	csState.SetPrivValidator(client.privValidator)

	client.csState = csState

	// Create & add listener
	protocol, listeningAddress := "tcp", "0.0.0.0:46656"
	node := NewNode(validatorNodes, protocol, listeningAddress, client.privKey, state.ChainID, qbftVersion, csState)

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

func (client *Client) getBlockInfoTx(current *types.Block) (*tmtypes.QbftNodeAction, error) {
	//检查第一笔交易
	if len(current.Txs) == 0 {
		return nil, types.ErrEmptyTx
	}
	baseTx := current.Txs[0]

	var valAction tmtypes.QbftNodeAction
	err := types.Decode(baseTx.GetPayload(), &valAction)
	if err != nil {
		return nil, err
	}
	//检查交易类型
	if valAction.GetTy() != tmtypes.QbftNodeActionBlockInfo {
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
	valAction, err := client.getBlockInfoTx(current.Block)
	if err != nil {
		return err
	}
	if parent.Height+1 != current.Block.Height {
		return types.ErrBlockHeight
	}
	//判断exec 是否成功
	if current.Receipts[0].Ty != types.ExecOk {
		return ttypes.ErrBaseExecErr
	}
	info := valAction.GetBlockInfo()
	if current.Block.Height > 1 {
		lastValAction, err := client.getBlockInfoTx(parent)
		if err != nil {
			return err
		}
		lastInfo := lastValAction.GetBlockInfo()
		lastProposalBlock := &ttypes.QbftBlock{QbftBlock: lastInfo.GetBlock()}
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
	for {
		if client.IsClosed() {
			qbftlog.Info("CreateBlock quit")
			break
		}
		if !client.csState.IsRunning() {
			qbftlog.Info("consensus not running")
			time.Sleep(500 * time.Millisecond)
			continue
		}

		height, err := client.getLastHeight()
		if err != nil {
			continue
		}
		if !client.CheckTxsAvailable(height) {
			time.Sleep(1000 * time.Millisecond)
			continue
		}

		if height+1 == client.csState.GetRoundState().Height {
			client.txsAvailable <- height + 1
		}
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

// TxsAvailable check available channel
func (client *Client) TxsAvailable() <-chan int64 {
	return client.txsAvailable
}

// StopC stop client
func (client *Client) StopC() <-chan struct{} {
	return client.stopC
}

// CheckTxsAvailable check whether some new transactions arriving
func (client *Client) CheckTxsAvailable(height int64) bool {
	txs := client.RequestTx(1, nil)
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
		qbftlog.Error("BuildBlock fail", "err", err)
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
	retErr := client.WriteBlock(nil, block)
	if retErr != nil {
		qbftlog.Info("CommitBlock fail", "err", retErr)
		if client.WaitBlock(block.Height) {
			if !preExec {
				return nil
			}
			curBlock, err := client.RequestBlock(block.Height)
			if err == nil {
				if bytes.Equal(curBlock.Hash(cfg), block.Hash(cfg)) {
					qbftlog.Info("already has block")
					return nil
				}
				qbftlog.Info("block is different", "block", block, "curBlock", curBlock)
				if bytes.Equal(curBlock.Txs[0].Hash(), block.Txs[0].Hash()) {
					qbftlog.Warn("base tx is same, origin maybe same")
					return nil
				}
			}
		}
		return retErr
	}
	return nil
}

// WaitBlock by height
func (client *Client) WaitBlock(height int64) bool {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	beg := time.Now()
	for {
		if client.IsClosed() {
			qbftlog.Info("WaitBlock quit")
			return false
		}
		select {
		case <-ticker.C:
			qbftlog.Info("Still waiting block......", "height", height, "cost", time.Since(beg))
		default:
			newHeight, err := client.getLastHeight()
			if err == nil && newHeight >= height {
				return true
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// QueryValidatorsByHeight ...
func (client *Client) QueryValidatorsByHeight(height int64) (*tmtypes.QbftNodes, error) {
	if height < 1 {
		return nil, ttypes.ErrHeightLessThanOne
	}
	req := &tmtypes.ReqQbftNodes{Height: height}
	param, err := proto.Marshal(req)
	if err != nil {
		qbftlog.Error("QueryValidatorsByHeight marshal", "err", err)
		return nil, types.ErrInvalidParam
	}
	msg := client.GetQueueClient().NewMessage("execs", types.EventBlockChainQuery,
		&types.ChainExecutor{Driver: "qbftNode", FuncName: "GetQbftNodeByHeight", StateHash: zeroHash[:], Param: param})
	err = client.GetQueueClient().Send(msg, true)
	if err != nil {
		qbftlog.Error("QueryValidatorsByHeight send", "err", err)
		return nil, err
	}
	msg, err = client.GetQueueClient().Wait(msg)
	if err != nil {
		qbftlog.Info("QueryValidatorsByHeight result", "err", err)
		return nil, err
	}
	return msg.GetData().(types.Message).(*tmtypes.QbftNodes), nil
}

// QueryBlockInfoByHeight get blockInfo and block by height
func (client *Client) QueryBlockInfoByHeight(height int64) (*tmtypes.QbftBlockInfo, *types.Block, error) {
	if height < 1 {
		return nil, nil, ttypes.ErrHeightLessThanOne
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
func (client *Client) LoadBlockCommit(height int64) *tmtypes.QbftCommit {
	blockInfo, _, err := client.QueryBlockInfoByHeight(height)
	if err != nil {
		qbftlog.Error("LoadBlockCommit GetBlockInfo fail", "err", err)
		return nil
	}
	seq, voteType := blockInfo.State.LastSequence, blockInfo.Block.LastCommit.VoteType
	if (seq == 0 && voteType != uint32(ttypes.VoteTypePrecommit)) ||
		(seq > 0 && voteType != uint32(ttypes.VoteTypePrevote)) {
		qbftlog.Error("LoadBlockCommit wrong VoteType", "seq", seq, "voteType", voteType)
		return nil
	}
	return blockInfo.GetBlock().GetLastCommit()
}

// LoadBlockState by height
func (client *Client) LoadBlockState(height int64) *tmtypes.QbftState {
	blockInfo, _, err := client.QueryBlockInfoByHeight(height)
	if err != nil {
		qbftlog.Error("LoadBlockState GetBlockInfo fail", "err", err)
		return nil
	}
	return blockInfo.GetState()
}

// LoadProposalBlock by height
func (client *Client) LoadProposalBlock(height int64) *tmtypes.QbftBlock {
	blockInfo, block, err := client.QueryBlockInfoByHeight(height)
	if err != nil {
		qbftlog.Error("LoadProposal GetBlockInfo fail", "err", err)
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
	return &tmtypes.QbftIsHealthy{IsHealthy: isHealthy}, nil
}

// Query_NodeInfo query validator node info
func (client *Client) Query_NodeInfo(req *types.ReqNil) (types.Message, error) {
	vals := client.csState.GetRoundState().Validators.Validators
	nodes := make([]*tmtypes.QbftNodeInfo, 0)
	for _, val := range vals {
		if val == nil {
			nodes = append(nodes, &tmtypes.QbftNodeInfo{})
		} else {
			ipstr, idstr := "UNKOWN", "UNKOWN"
			pub, err := ttypes.ConsensusCrypto.PubKeyFromBytes(val.PubKey)
			if err != nil {
				qbftlog.Error("Query_NodeInfo invalid pubkey", "err", err)
			} else {
				id := GenIDByPubKey(pub)
				idstr = string(id)
				if id == client.node.ID {
					ipstr = client.node.IP
				} else {
					ip := client.node.peerSet.GetIP(id)
					if ip == nil {
						qbftlog.Error("Query_NodeInfo nil ip", "id", idstr)
					} else {
						ipstr = ip.String()
					}
				}
			}

			item := &tmtypes.QbftNodeInfo{
				NodeIP:      ipstr,
				NodeID:      idstr,
				Address:     fmt.Sprintf("%X", val.Address),
				PubKey:      fmt.Sprintf("%X", val.PubKey),
				VotingPower: val.VotingPower,
				Accum:       val.Accum,
			}
			nodes = append(nodes, item)
		}
	}
	return &tmtypes.QbftNodeInfoSet{Nodes: nodes}, nil
}

// CmpBestBlock 比较newBlock是不是最优区块
func (client *Client) CmpBestBlock(newBlock *types.Block, cmpBlock *types.Block) bool {
	return false
}
