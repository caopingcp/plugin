// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package qbft

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/33cn/chain33/types"
	ttypes "github.com/33cn/plugin/plugin/consensus/qbft/types"
	tmtypes "github.com/33cn/plugin/plugin/dapp/qbftNode/types"
	"github.com/golang/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
)

const (
	moduleName       = "consensus"
	qbftVoteTopic    = "QBFT-VOTE-TOPIC"
	qbftAggVoteTopic = "QBFT-AGGVOTE-TOPIC"
	qbftMsgTopic     = "QBFT-MSG-TOPIC"

	maxMsgQueueSize = 1024
	maxPeersNum     = 1024
	maxAliveCount   = 20
)

// P2PNode struct
type P2pNode struct {
	ID    ID
	IP    string
	valId string

	state    *ConsensusState
	peerSet  *lru.Cache
	valIdSet *lru.Cache

	eventChannel     chan *tmtypes.QbftP2PMsg
	broadcastChannel chan MsgInfo
	unicastChannel   chan MsgInfo

	cancel context.CancelFunc
	ctx    context.Context
}

// NewP2pNode method
func NewP2pNode(state *ConsensusState) *P2pNode {
	peerSet, err := lru.New(maxPeersNum)
	if err != nil {
		panic(err)
	}
	valIdSet, err := lru.New(maxPeersNum)
	if err != nil {
		panic(err)
	}
	node := &P2pNode{
		state:            state,
		peerSet:          peerSet,
		valIdSet:         valIdSet,
		eventChannel:     make(chan *tmtypes.QbftP2PMsg, maxMsgQueueSize),
		broadcastChannel: make(chan MsgInfo, maxMsgQueueSize),
		unicastChannel:   make(chan MsgInfo, maxMsgQueueSize),
	}

	state.SetBroadcastChannel(node.broadcastChannel)
	state.SetUnicastChannel(node.unicastChannel)
	return node
}

// Start node
func (node *P2pNode) Start() {
	node.ctx, node.cancel = context.WithCancel(context.Background())
	node.initSelfInfo()
	node.updatePeerSet()

	go node.StartConsensusRoutine()
	go node.recvMsgRoutine()
	go node.sendMsgRoutine()
	go node.managePeersRoutine()
}

// Stop node
func (node *P2pNode) Stop() {
	// Stop peers
	for _, id := range node.peerSet.Keys() {
		peer, _ := node.getPeer(id)
		peer.Stop()
	}
	node.peerSet.Purge()
	if node.cancel != nil {
		node.cancel()
	}
	//stop consensus
	node.state.Stop()
}

func (node *P2pNode) initSelfInfo() {
	hint := time.NewTicker(10 * time.Second)
	defer hint.Stop()

	beg := time.Now()
	for {
		select {
		case <-node.ctx.Done():
			qbftlog.Info("P2pNode initSelfInfo quit")
			return
		case <-hint.C:
			qbftlog.Info("Still waiting P2P module up......", "cost", time.Since(beg))
		default:
			info, err := node.getSelfInfo()
			if err == nil {
				node.ID, node.IP = ID(info.Name), info.Addr
				node.state.SetOurID(node.ID)
				node.valId = node.state.privValidator.GetPubKey().KeyString()
				node.valIdSet.Add(node.valId, node.ID)
				qbftlog.Info("init self info", "id", node.ID, "ip", node.IP, "valId", node.valId)
				return
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// StartConsensusRoutine if peers reached the threshold start consensus routine
func (node *P2pNode) StartConsensusRoutine() {
	for {
		select {
		case <-node.ctx.Done():
			qbftlog.Info("P2pNode StartConsensusRoutine quit")
			return
		default:
			//TODO:the peer count need be optimized
			if node.peerSet.Len() >= 0 {
				node.state.Start()
				return
			}
			time.Sleep(1 * time.Second)
		}

	}
}

func (node *P2pNode) recvMsgRoutine() {
	for {
		select {
		case <-node.ctx.Done():
			qbftlog.Info("P2pNode recvMsgRoutine quit")
			return
		case msg := <-node.eventChannel:
			node.procMsg(msg)
		}
	}
}

func (node *P2pNode) sendMsgRoutine() {
	for {
		select {
		case <-node.ctx.Done():
			qbftlog.Info("P2pNode sendMsgRoutine quit")
			return
		case msg := <-node.broadcastChannel:
			node.broadcast(msg)
		case msg := <-node.unicastChannel:
			if string(msg.PeerID) == node.valId {
				break
			}
			id, ok := node.valIdSet.Get(string(msg.PeerID))
			if !ok {
				qbftlog.Error(fmt.Sprintf("unicastChannel not exist valId %v", msg.PeerID))
				break
			}
			peer, err := node.getPeer(id.(ID))
			if err != nil {
				qbftlog.Error("unicastChannel getPeer", "err", err)
				break
			}
			//change msg PeerID and PeerIP
			msg.PeerID = peer.ID()
			msg.PeerIP = peer.ip
			success := peer.Send(msg)
			if !success {
				qbftlog.Error("send failure in unicastChannel", "peerID", peer.ID())
			}
		}
	}
}

func (node *P2pNode) managePeersRoutine() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-node.ctx.Done():
			qbftlog.Info("P2pNode managePeersRoutine quit")
			return
		case <-ticker.C:
			node.updatePeerSet()
		}
	}
}

func (node *P2pNode) getSelfInfo() (*types.Peer, error) {
	list, err := node.state.client.SendFetchPeerList()
	if err != nil {
		return nil, err
	}
	if len(list.Peers) == 0 {
		return nil, fmt.Errorf("empty peer list")
	}
	info := list.Peers[len(list.Peers)-1]
	if !info.Self {
		return nil, fmt.Errorf("not self info")
	}
	return info, nil
}

func (node *P2pNode) updatePeerSet() {
	list, err := node.state.client.SendFetchPeerList()
	if err != nil {
		qbftlog.Error("updatePeerSet SendFetchPeerList fail", "err", err)
	}
	for _, peer := range list.GetPeers() {
		if !peer.Self {
			pp, err1 := node.getPeer(ID(peer.Name))
			if err1 == nil {
				pp.IncAliveCount()
			} else {
				newPeer := NewP2pPeer(ID(peer.Name), peer.Addr, node.state, node.valIdSet)
				err2 := node.addPeer(newPeer)
				if err2 != nil {
					qbftlog.Error("updatePeerSet addPeer fail", "err", err2)
				}
			}
		}
	}
	peerStrings := []string{}
	for _, id := range node.peerSet.Keys() {
		peer, err3 := node.getPeer(id)
		if err3 != nil {
			qbftlog.Error("updatePeerSet getPeer fail", "err", err3)
			continue
		}
		if !isContain(list.GetPeers(), string(id.(ID))) {
			peer.DecAliveCount()
			if peer.GetAliveCount() == 0 {
				err4 := node.delPeer(peer)
				if err4 != nil {
					qbftlog.Error("updatePeerSet delPeer fail", "err", err4)
				}
			}
		}
		peerStrings = append(peerStrings, peer.String())
	}
	qbftlog.Info("updatePeerSet print info", "Peers", peerStrings)
	for _, vid := range node.valIdSet.Keys() {
		pid, _ := node.valIdSet.Get(vid)
		if pid.(ID) == node.ID {
			continue
		}
		pub, _ := hex.DecodeString(vid.(string))
		pubkey, err := ttypes.ConsensusCrypto.PubKeyFromBytes(pub)
		if err != nil {
			qbftlog.Error("updatePeerSet PubKeyFromBytes fail", "err", err)
			node.valIdSet.Remove(vid)
			qbftlog.Info("delete wrong valId", "valId", vid, "pid", pid)
			continue
		}
		addr := ttypes.GenAddressByPubKey(pubkey)
		if !node.state.Validators.HasAddress(addr) {
			node.valIdSet.Remove(vid)
			qbftlog.Info("delete valId not in validator set", "valId", vid, "pid", pid)
			continue
		}
	}
}

func isContain(peers []*types.Peer, id string) bool {
	for _, peer := range peers {
		if peer.Name == id {
			return true
		}
	}
	return false
}

func (node *P2pNode) addPeer(peer *p2pPeer) error {
	err := peer.Start()
	if err != nil {
		return err
	}

	stateMsg := MsgInfo{TypeID: ttypes.NewRoundStepID, Msg: node.state.RoundStateMessage(), PeerID: peer.ID(), PeerIP: peer.ip}
	peer.Send(stateMsg)
	heartbeat, err := node.state.HeartbeatMessage(0)
	if err != nil {
		return err
	}
	heartbeatMsg := MsgInfo{TypeID: ttypes.ProposalHeartbeatID, Msg: heartbeat, PeerID: peer.ID(), PeerIP: peer.ip}
	peer.Send(heartbeatMsg)
	qbftlog.Info("Send state msg", "stateMsg", stateMsg)
	node.peerSet.Add(peer.ID(), peer)
	qbftlog.Info("add peer", "id", peer.ID(), "ip", peer.ip)
	return nil
}

func (node *P2pNode) delPeer(peer *p2pPeer) error {
	err := peer.Stop()
	if err != nil {
		return err
	}
	node.peerSet.Remove(peer.ID())
	qbftlog.Info("delete peer", "id", peer.ID(), "ip", peer.ip)
	return nil
}

func (node *P2pNode) broadcast(msg MsgInfo) chan bool {
	successChan := make(chan bool, node.peerSet.Len())
	qbftlog.Debug("broadcast msg", "msgtype", msg.TypeID)
	var wg sync.WaitGroup
	for _, id := range node.peerSet.Keys() {
		peer, err := node.getPeer(id)
		if err != nil {
			qbftlog.Error("broadcast getPeer", "err", err)
			continue
		}
		wg.Add(1)
		go func(peer *p2pPeer) {
			defer wg.Done()
			msg.PeerID = peer.ID()
			msg.PeerIP = peer.ip
			success := peer.Send(msg)
			successChan <- success
		}(peer)
	}
	go func() {
		wg.Wait()
		close(successChan)
	}()
	return successChan
}

func (node *P2pNode) procMsg(msg *tmtypes.QbftP2PMsg) {
	if ID(msg.FromId) == node.ID {
		qbftlog.Warn("receive msg from self", "msg", msg)
		return
	}
	peer, err := node.getPeer(ID(msg.FromId))
	if err != nil {
		qbftlog.Error("procMsg getPeer", "err", err)
		return
	}
	select {
	case peer.recvQueue <- msg:
		atomic.AddInt32(&peer.recvQueueSize, 1)
	case <-time.After(defaultSendTimeout):
		if byte(msg.Ty) == ttypes.ProposalBlockID {
			propBlock := msg.GetBlock()
			propBlock.Data = nil
		}
		qbftlog.Error("P2pNode send msg timeout", "peer", peer, "msg", msg)
	}
}

func (node *P2pNode) getPeer(id interface{}) (*p2pPeer, error) {
	item, ok := node.peerSet.Get(id)
	if !ok {
		return nil, fmt.Errorf("not exist peer id %v", id)
	}
	peer, ok := item.(*p2pPeer)
	if !ok {
		return nil, fmt.Errorf("not valid p2pPeer id %v", id)
	}
	return peer, nil
}

func (node *P2pNode) GetPeerIP(id ID) string {
	if id == node.ID {
		return node.IP
	}
	peer, err := node.getPeer(id)
	if err != nil {
		return ""
	}
	return peer.ip
}

// p2pPeer
type p2pPeer struct {
	id    ID
	ip    string
	valId string

	myState    *ConsensusState
	state      *PeerConnState
	valIdSet   *lru.Cache
	aliveCount int32

	recvQueue        chan *tmtypes.QbftP2PMsg
	recvQueueSize    int32
	transferChannel  chan MsgInfo
	updateStateQueue chan MsgInfo
	heartbeatQueue   chan proto.Message

	ctx    context.Context
	cancel context.CancelFunc
}

// NewP2pPeer method
func NewP2pPeer(id ID, ip string, state *ConsensusState, valIdSet *lru.Cache) *p2pPeer {
	peer := &p2pPeer{
		id:               id,
		ip:               ip,
		myState:          state,
		valIdSet:         valIdSet,
		aliveCount:       maxAliveCount,
		recvQueue:        make(chan *tmtypes.QbftP2PMsg, maxMsgQueueSize),
		updateStateQueue: make(chan MsgInfo, maxMsgQueueSize),
		heartbeatQueue:   make(chan proto.Message, 100),
	}
	peer.state = &PeerConnState{ip: net.ParseIP(ip), PeerRoundState: ttypes.PeerRoundState{
		Round:              -1,
		ProposalPOLRound:   -1,
		LastCommitRound:    -1,
		CatchupCommitRound: -1,
	}}
	peer.SetTransferChannel(state.peerMsgQueue)
	return peer
}

func (pp *p2pPeer) Start() error {
	pp.ctx, pp.cancel = context.WithCancel(context.Background())

	go pp.recvRoutine()
	go pp.gossipDataRoutine()
	go pp.gossipVotesRoutine()
	go pp.queryMaj23Routine()

	return nil
}

func (pp *p2pPeer) Stop() error {
	if pp.cancel != nil {
		pp.cancel()
	}
	return nil
}

func (pp *p2pPeer) ID() ID {
	return pp.id
}

func (pp *p2pPeer) String() string {
	if pp == nil {
		return "nil-Peer"
	}
	return fmt.Sprintf("Peer{ID:%v IP:%v VID:%v AC:%v}",
		pp.id,
		pp.ip,
		pp.valId,
		pp.aliveCount)
}

func (pp *p2pPeer) SetTransferChannel(transferChannel chan MsgInfo) {
	pp.transferChannel = transferChannel
}

func (pp *p2pPeer) GetAliveCount() int32 { return atomic.LoadInt32(&pp.aliveCount) }

func (pp *p2pPeer) IncAliveCount() {
	if atomic.LoadInt32(&pp.aliveCount) < maxAliveCount {
		atomic.AddInt32(&pp.aliveCount, 1)
	}
}

func (pp *p2pPeer) DecAliveCount() {
	if atomic.LoadInt32(&pp.aliveCount) > 0 {
		atomic.AddInt32(&pp.aliveCount, -1)
	}
}

func (pp *p2pPeer) recvRoutine() {
	for {
		select {
		case <-pp.ctx.Done():
			qbftlog.Info("recvRoutine quit")
			return
		case msg := <-pp.recvQueue:
			pp.handleMsg(msg)
		}
	}
}

func (pp *p2pPeer) transferMsg(ty byte, msg proto.Message) {
	pp.transferChannel <- MsgInfo{ty, msg, pp.id, pp.ip}
}

func (pp *p2pPeer) handleMsg(msg *tmtypes.QbftP2PMsg) {
	ps := pp.state
	ty := byte(msg.GetTy())
	switch ty {
	case ttypes.ProposalID:
		proposal := msg.GetProposal()
		pp.transferMsg(ty, proposal)
		ps.SetHasProposal(proposal)
	case ttypes.VoteID:
		pp.transferMsg(ty, msg.GetVote())
		vote := &ttypes.Vote{QbftVote: msg.GetVote()}
		ps.SetHasVote(vote)
	case ttypes.ProposalBlockID:
		pp.transferMsg(ty, msg.GetBlock())
		block := &ttypes.QbftBlock{QbftBlock: msg.GetBlock()}
		ps.SetHasProposalBlock(block)
	case ttypes.AggVoteID:
		pp.transferMsg(ty, msg.GetAggVote())
		aggVote := &ttypes.AggVote{QbftAggVote: msg.GetAggVote()}
		ps.SetHasAggVote(aggVote)
	case ttypes.NewRoundStepID:
		ps.ApplyNewRoundStepMessage(msg.GetNewRoundStep())
	case ttypes.ValidBlockID:
		ps.ApplyValidBlockMessage(msg.GetValidBlock())
	case ttypes.ExpectBlockID:
		ps.ApplyExpectBlockMessage(msg.GetExpectBlock())
	case ttypes.HasVoteID:
		ps.ApplyHasVoteMessage(msg.GetHasVote())
	case ttypes.VoteSetMaj23ID:
		maj32 := msg.GetVoteSetMaj23()
		pp.myState.SetPeerMaj23(maj32.Height, int(maj32.Round), byte(maj32.Type), string(pp.id), maj32.BlockID)
		var myVotes *ttypes.BitArray
		switch byte(maj32.Type) {
		case ttypes.VoteTypePrevote:
			myVotes = pp.myState.GetPrevotesState(maj32.Height, int(maj32.Round), maj32.BlockID)
		case ttypes.VoteTypePrecommit:
			myVotes = pp.myState.GetPrecommitsState(maj32.Height, int(maj32.Round), maj32.BlockID)
		default:
			qbftlog.Error("Bad VoteSetBitsMessage field Type", "type", byte(maj32.Type))
			return
		}
		if myVotes != nil && myVotes.QbftBitArray != nil {
			voteSetBitMsg := &tmtypes.QbftVoteSetBitsMsg{
				Height:  maj32.Height,
				Round:   maj32.Round,
				Type:    maj32.Type,
				BlockID: maj32.BlockID,
				Votes:   myVotes.QbftBitArray,
			}
			pp.Send(MsgInfo{TypeID: ttypes.VoteSetBitsID, Msg: voteSetBitMsg, PeerID: pp.id, PeerIP: pp.ip})
		}
	case ttypes.ProposalPOLID:
		ps.ApplyProposalPOLMessage(msg.GetProposalPOL())
	case ttypes.VoteSetBitsID:
		tmp := msg.GetVoteSetBits()
		if pp.myState.Height == tmp.Height {
			var myVotes *ttypes.BitArray
			switch byte(tmp.Type) {
			case ttypes.VoteTypePrevote:
				myVotes = pp.myState.GetPrevotesState(tmp.Height, int(tmp.Round), tmp.BlockID)
			case ttypes.VoteTypePrecommit:
				myVotes = pp.myState.GetPrecommitsState(tmp.Height, int(tmp.Round), tmp.BlockID)
			default:
				qbftlog.Error("Bad VoteSetBitsMessage field Type", "type", byte(tmp.Type))
				return
			}
			ps.ApplyVoteSetBitsMessage(tmp, myVotes)
		} else {
			ps.ApplyVoteSetBitsMessage(tmp, nil)
		}
	case ttypes.ProposalHeartbeatID:
		heartbeat := msg.GetHeartbeat()
		qbftlog.Debug("Received proposal heartbeat message",
			"height", heartbeat.Height, "round", heartbeat.Round, "sequence", heartbeat.Sequence,
			"valIdx", heartbeat.ValidatorIndex, "valAddr", fmt.Sprintf("%X", heartbeat.ValidatorAddress))
		pp.procHeartbeatMsg(heartbeat)
	default:
		qbftlog.Error("qbft ProcEvent not support", "ty", msg.GetTy())
	}

}

func (pp *p2pPeer) procHeartbeatMsg(heartbeat *tmtypes.QbftHeartbeat) {
	_, val := pp.myState.Validators.GetByIndex(int(heartbeat.ValidatorIndex))
	// Verify signature
	pubkey, err := ttypes.ConsensusCrypto.PubKeyFromBytes(val.PubKey)
	if err != nil {
		qbftlog.Error("procHeartbeatMsg PubKeyFromBytes", "err", err)
	}
	heartbeatTmp := &ttypes.Heartbeat{QbftHeartbeat: heartbeat}
	signature, err := ttypes.ConsensusCrypto.SignatureFromBytes(heartbeat.Signature)
	if err != nil {
		qbftlog.Error("procHeartbeatMsg SignatureFromBytes", "err", err)
	}
	if !pubkey.VerifyBytes(ttypes.SignBytes(pp.myState.state.ChainID, heartbeatTmp), signature) {
		qbftlog.Error("procHeartbeatMsg VerifyBytes", "err", err)
	}
	pp.valId = pubkey.KeyString()
	if id, ok := pp.valIdSet.Get(pp.valId); ok {
		if id.(ID) == pp.ID() {
			return
		}
		qbftlog.Info("old valId info", "valId", pp.valId, "old", id.(ID))
	}
	pp.valIdSet.Add(pp.valId, pp.ID())
	qbftlog.Info("add new valId", "valId", pp.valId, "pid", pp.ID())
}

func (pp *p2pPeer) gossipDataRoutine() {
	for {
		select {
		case <-pp.ctx.Done():
			qbftlog.Info("gossipDataRoutine quit")
			return
		default:
			rs := pp.myState.GetRoundState()
			prs := pp.state.GetRoundState()

			// If the peer is on a previous height, help catch up.
			if (0 < prs.Height) && (prs.Height < rs.Height) {
				if prs.ProposalBlockHash != nil && !prs.ProposalBlock {
					proposalBlock := pp.myState.client.LoadProposalBlock(prs.Height)
					if proposalBlock == nil {
						qbftlog.Error("load proposal block fail", "selfHeight", rs.Height,
							"blockHeight", pp.myState.client.GetCurrentHeight())
						time.Sleep(pp.myState.PeerGossipSleep())
						continue
					}
					newBlock := &ttypes.QbftBlock{QbftBlock: proposalBlock}
					if !newBlock.HashesTo(prs.ProposalBlockHash) {
						qbftlog.Error(fmt.Sprintf("Wrong proposal block hash. Expected %X, got %X", prs.ProposalBlockHash,
							newBlock.Hash()), "height", prs.Height)
						time.Sleep(pp.myState.PeerGossipSleep())
						continue
					}
					msg := MsgInfo{TypeID: ttypes.ProposalBlockID, Msg: proposalBlock, PeerID: pp.id, PeerIP: pp.ip}
					qbftlog.Info("Sending block for catchup", "peerIP", pp.ip,
						"selfHeight", rs.Height, "peer(H/R/S)", fmt.Sprintf("%v/%v/%v", prs.Height, prs.Round, prs.Step),
						"block(H/R/hash)", fmt.Sprintf("%v/%v/%X", proposalBlock.Header.Height, proposalBlock.Header.Round, newBlock.Hash()))
					if pp.Send(msg) {
						pp.state.SetHasProposalBlock(newBlock)
					}
					continue
				}
			}

			// If height and round don't match, sleep.
			if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
				time.Sleep(pp.myState.PeerGossipSleep())
				continue
			}

			// By here, height and round match.
			// Proposal block parts were already matched and sent if any were wanted.
			// (These can match on hash so the round doesn't matter)
			// Now consider sending other things, like the Proposal itself.

			// Send Proposal && ProposalPOL BitArray?
			if rs.Proposal != nil && !prs.Proposal {
				// Proposal: share the proposal metadata with peer.
				{
					msg := MsgInfo{TypeID: ttypes.ProposalID, Msg: rs.Proposal, PeerID: pp.id, PeerIP: pp.ip}
					qbftlog.Debug(fmt.Sprintf("Sending proposal. Self myState: %v/%v/%v", rs.Height, rs.Round, rs.Step),
						"peerip", pp.ip, "proposal-height", rs.Proposal.Height, "proposal-round", rs.Proposal.Round)
					if pp.Send(msg) {
						pp.state.SetHasProposal(rs.Proposal)
					}
				}
				// ProposalPOL: lets peer know which POL votes we have so far.
				// Peer must receive ttypes.ProposalMessage first.
				// rs.Proposal was validated, so rs.Proposal.POLRound <= rs.Round,
				// so we definitely have rs.Votes.Prevotes(rs.Proposal.POLRound).
				if 0 <= rs.Proposal.POLRound {
					msg := MsgInfo{TypeID: ttypes.ProposalPOLID, Msg: &tmtypes.QbftProposalPOLMsg{
						Height:           rs.Height,
						ProposalPOLRound: rs.Proposal.POLRound,
						ProposalPOL:      rs.Votes.Prevotes(int(rs.Proposal.POLRound)).BitArray().QbftBitArray,
					}, PeerID: pp.id, PeerIP: pp.ip}
					qbftlog.Debug("Sending POL", "height", prs.Height, "round", prs.Round)
					pp.Send(msg)
				}
				continue
			}

			// Send proposal block
			if rs.Proposal != nil && prs.ProposalBlockHash != nil && bytes.Equal(rs.Proposal.Blockhash, prs.ProposalBlockHash) {
				if rs.ProposalBlock != nil && !prs.ProposalBlock {
					msg := MsgInfo{TypeID: ttypes.ProposalBlockID, Msg: rs.ProposalBlock.QbftBlock, PeerID: pp.id, PeerIP: pp.ip}
					qbftlog.Debug(fmt.Sprintf("Sending proposal block. Self myState: %v/%v/%v", rs.Height, rs.Round, rs.Step),
						"peerip", pp.ip, "block-height", rs.ProposalBlock.Header.Height, "block-round", rs.ProposalBlock.Header.Round)
					if pp.Send(msg) {
						pp.state.SetHasProposalBlock(rs.ProposalBlock)
					}
					continue
				}
			}

			// Nothing to do. Sleep.
			time.Sleep(pp.myState.PeerGossipSleep())
			continue
		}
	}
}

func (pp *p2pPeer) gossipVotesRoutine() {
	// Simple hack to throttle logs upon sleep.
	var sleeping = 0
	for {
		select {
		case <-pp.ctx.Done():
			qbftlog.Info("queryMaj23Routine quit")
			return
		default:
			rs := pp.myState.GetRoundState()
			prs := pp.state.GetRoundState()

			switch sleeping {
			case 1: // First sleep
				sleeping = 2
			case 2: // No more sleep
				sleeping = 0
			}

			// If height matches, then send LastCommit, Prevotes, Precommits.
			if rs.Height == prs.Height {
				if !UseAggSig() || gossipVotes.Load().(bool) {
					if pp.gossipVotesForHeight(rs, prs) {
						continue
					}
				}
			}

			// Special catchup logic.
			// If peer is lagging by height 1, send LastCommit.
			if prs.Height != 0 && rs.Height == prs.Height+1 {
				if pp.PickSendVote(rs.LastCommit) {
					qbftlog.Debug("Pick rs.LastCommit to send", "peerip", pp.ip, "height", prs.Height)
					continue
				}
			}

			// Catchup logic
			// If peer is lagging by more than 1, send Commit.
			if prs.Height != 0 && rs.Height >= prs.Height+2 {
				// Load the block commit for prs.Height,
				// which contains precommit signatures for prs.Height.
				commit := pp.myState.client.LoadBlockCommit(prs.Height + 1)
				commitObj := &ttypes.Commit{QbftCommit: commit}
				if pp.PickSendVote(commitObj) {
					qbftlog.Info("Pick Catchup commit to send",
						"commit(H/R)", fmt.Sprintf("%v/%v", commitObj.Height(), commitObj.Round()),
						"BitArray", commitObj.BitArray().String(),
						"peerip", pp.ip, "peer(H/R/S)", fmt.Sprintf("%v/%v/%v", prs.Height, prs.Round, prs.Step))
					continue
				}
			}

			if sleeping == 0 {
				// We sent nothing. Sleep...
				sleeping = 1
				qbftlog.Debug("No votes to send, sleeping", "peerip", pp.ip, "rs.Height", rs.Height, "prs.Height", prs.Height,
					"localPV", rs.Votes.Prevotes(rs.Round).BitArray(), "peerPV", prs.Prevotes,
					"localPC", rs.Votes.Precommits(rs.Round).BitArray(), "peerPC", prs.Precommits)
			} else if sleeping == 2 {
				// Continued sleep...
				sleeping = 1
			}

			time.Sleep(pp.myState.PeerGossipSleep())
			continue
		}
	}
}

func (pp *p2pPeer) gossipVotesForHeight(rs *ttypes.RoundState, prs *ttypes.PeerRoundState) bool {
	// If there are lastCommits to send...
	if prs.Step == ttypes.RoundStepNewHeight {
		if pp.PickSendVote(rs.LastCommit) {
			qbftlog.Debug("Pick rs.LastCommit to send", "peerip", pp.ip,
				"peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round))
			return true
		}
	}
	// If there are POL prevotes to send...
	if prs.Step <= ttypes.RoundStepPropose && prs.Round != -1 && prs.Round <= rs.Round && prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if pp.PickSendVote(polPrevotes) {
				qbftlog.Debug("Pick rs.Prevotes(prs.ProposalPOLRound) to send",
					"peerip", pp.ip, "peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round),
					"POLRound", prs.ProposalPOLRound)
				return true
			}
		}
	}
	// If there are prevotes to send...
	if prs.Step <= ttypes.RoundStepPrevoteWait && prs.Round != -1 && prs.Round <= rs.Round {
		if pp.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
			qbftlog.Debug("Pick rs.Prevotes(prs.Round) to send",
				"peerip", pp.ip, "peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round))
			return true
		}
	}
	// If there are precommits to send...
	if prs.Step <= ttypes.RoundStepPrecommitWait && prs.Round != -1 && prs.Round <= rs.Round {
		if pp.PickSendVote(rs.Votes.Precommits(prs.Round)) {
			qbftlog.Debug("Pick rs.Precommits(prs.Round) to send",
				"peerip", pp.ip, "peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round))
			return true
		}
	}
	// If there are prevotes to send...Needed because of validBlock mechanism
	if prs.Round != -1 && prs.Round <= rs.Round {
		if pp.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
			qbftlog.Debug("Pick rs.Prevotes(prs.Round) to send",
				"peerip", pp.ip, "peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round))
			return true
		}
	}
	// If there are POLPrevotes to send...
	if prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if pp.PickSendVote(polPrevotes) {
				qbftlog.Debug("Pick rs.Prevotes(prs.ProposalPOLRound) to send",
					"peerip", pp.ip, "round", prs.ProposalPOLRound)
				return true
			}
		}
	}
	return false
}

func (pp *p2pPeer) queryMaj23Routine() {
	for {
		select {
		case <-pp.ctx.Done():
			qbftlog.Info("gossipVotesRoutine quit")
			return
		default:
			// Maybe send Height/Round/Prevotes
			{
				rs := pp.myState.GetRoundState()
				prs := pp.state.GetRoundState()
				if rs.Height == prs.Height {
					if maj23, ok := rs.Votes.Prevotes(prs.Round).TwoThirdsMajority(); ok {
						msg := MsgInfo{TypeID: ttypes.VoteSetMaj23ID, Msg: &tmtypes.QbftVoteSetMaj23Msg{
							Height:  prs.Height,
							Round:   int32(prs.Round),
							Type:    int32(ttypes.VoteTypePrevote),
							BlockID: &maj23,
						}, PeerID: pp.id, PeerIP: pp.ip,
						}
						pp.Send(msg)
						time.Sleep(pp.myState.PeerQueryMaj23Sleep())
					}
				}
			}

			// Maybe send Height/Round/Precommits
			{
				rs := pp.myState.GetRoundState()
				prs := pp.state.GetRoundState()
				if rs.Height == prs.Height {
					if maj23, ok := rs.Votes.Precommits(prs.Round).TwoThirdsMajority(); ok {
						msg := MsgInfo{TypeID: ttypes.VoteSetMaj23ID, Msg: &tmtypes.QbftVoteSetMaj23Msg{
							Height:  prs.Height,
							Round:   int32(prs.Round),
							Type:    int32(ttypes.VoteTypePrecommit),
							BlockID: &maj23,
						}, PeerID: pp.id, PeerIP: pp.ip,
						}
						pp.Send(msg)
						time.Sleep(pp.myState.PeerQueryMaj23Sleep())
					}
				}
			}

			// Maybe send Height/Round/ProposalPOL
			{
				rs := pp.myState.GetRoundState()
				prs := pp.state.GetRoundState()
				if rs.Height == prs.Height && prs.ProposalPOLRound >= 0 {
					if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
						msg := MsgInfo{TypeID: ttypes.VoteSetMaj23ID, Msg: &tmtypes.QbftVoteSetMaj23Msg{
							Height:  prs.Height,
							Round:   int32(prs.ProposalPOLRound),
							Type:    int32(ttypes.VoteTypePrevote),
							BlockID: &maj23,
						}, PeerID: pp.id, PeerIP: pp.ip,
						}
						pp.Send(msg)
						time.Sleep(pp.myState.PeerQueryMaj23Sleep())
					}
				}
			}

			// Little point sending LastCommitRound/LastCommit,
			// These are fleeting and non-blocking.

			// Maybe send Height/CatchupCommitRound/CatchupCommit.
			{
				prs := pp.state.GetRoundState()
				if prs.CatchupCommitRound != -1 && 0 < prs.Height && prs.Height <= pp.myState.client.csStore.LoadStateHeight() {
					commit := pp.myState.LoadCommit(prs.Height)
					if commit.VoteType == uint32(ttypes.VoteTypePrecommit) {
						commitTmp := ttypes.Commit{QbftCommit: commit}
						msg := MsgInfo{TypeID: ttypes.VoteSetMaj23ID, Msg: &tmtypes.QbftVoteSetMaj23Msg{
							Height:  prs.Height,
							Round:   int32(commitTmp.Round()),
							Type:    int32(ttypes.VoteTypePrecommit),
							BlockID: commit.BlockID,
						}, PeerID: pp.id, PeerIP: pp.ip,
						}
						pp.Send(msg)
						time.Sleep(pp.myState.PeerQueryMaj23Sleep())
					}
				}
			}

			time.Sleep(pp.myState.PeerQueryMaj23Sleep())
			continue
		}
	}
}

// PickSendVote picks a vote and sends it to the peer.
// Returns true if vote was sent.
func (pp *p2pPeer) PickSendVote(votes ttypes.VoteSetReader) bool {
	if UseAggSig() {
		if votes.GetAggVote() != nil {
			if aggVote, ok := pp.state.PickAggVoteToSend(votes); ok {
				msg := MsgInfo{TypeID: ttypes.AggVoteID, Msg: aggVote.QbftAggVote, PeerID: pp.id, PeerIP: pp.ip}
				qbftlog.Debug("Sending aggregate vote message", "msg", msg)
				if pp.Send(msg) {
					pp.state.SetHasAggVote(aggVote)
					return true
				}
			}
			return false
		}
	}
	if vote, ok := pp.state.PickVoteToSend(votes); ok {
		if vote == nil {
			qbftlog.Warn("Pick nil vote", "aggVote", votes.GetAggVote(), "vote", votes)
			return false
		}
		msg := MsgInfo{TypeID: ttypes.VoteID, Msg: vote.QbftVote, PeerID: pp.id, PeerIP: pp.ip}
		qbftlog.Debug("Sending vote message", "msg", msg)
		if pp.Send(msg) {
			pp.state.SetHasVote(vote)
			return true
		}
	}
	return false
}

func (pp *p2pPeer) Send(msg MsgInfo) bool {
	fromId := pp.myState.GetOurID()
	p2pMsg := convertToP2pMsg(msg, fromId)
	if p2pMsg == nil {
		return false
	}
	bytes, err := proto.Marshal(p2pMsg)
	if err != nil {
		qbftlog.Error("p2pPeer marshal msg fail", "err", err)
		return false
	}
	err = pp.myState.client.SendPeerMsg(qbftMsgTopic, string(fromId), string(msg.PeerID), bytes)
	if err != nil {
		qbftlog.Error("p2pPeer send msg fail", "err", err)
		return false
	}
	return true
}

func convertToP2pMsg(msg MsgInfo, from ID) *tmtypes.QbftP2PMsg {
	p2pMsg := &tmtypes.QbftP2PMsg{Ty: int32(msg.TypeID), PeerId: string(msg.PeerID), FromId: string(from)}
	switch msg.TypeID {
	case ttypes.NewRoundStepID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_NewRoundStep{NewRoundStep: msg.Msg.(*tmtypes.QbftNewRoundStepMsg)}
	case ttypes.ProposalID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_Proposal{Proposal: msg.Msg.(*tmtypes.QbftProposal)}
	case ttypes.ProposalPOLID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_ProposalPOL{ProposalPOL: msg.Msg.(*tmtypes.QbftProposalPOLMsg)}
	case ttypes.VoteID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_Vote{Vote: msg.Msg.(*tmtypes.QbftVote)}
	case ttypes.HasVoteID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_HasVote{HasVote: msg.Msg.(*tmtypes.QbftHasVoteMsg)}
	case ttypes.VoteSetMaj23ID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_VoteSetMaj23{VoteSetMaj23: msg.Msg.(*tmtypes.QbftVoteSetMaj23Msg)}
	case ttypes.VoteSetBitsID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_VoteSetBits{VoteSetBits: msg.Msg.(*tmtypes.QbftVoteSetBitsMsg)}
	case ttypes.ProposalHeartbeatID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_Heartbeat{Heartbeat: msg.Msg.(*tmtypes.QbftHeartbeat)}
	case ttypes.ProposalBlockID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_Block{Block: msg.Msg.(*tmtypes.QbftBlock)}
	case ttypes.ValidBlockID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_ValidBlock{ValidBlock: msg.Msg.(*tmtypes.QbftValidBlockMsg)}
	case ttypes.AggVoteID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_AggVote{AggVote: msg.Msg.(*tmtypes.QbftAggVote)}
	case ttypes.ExpectBlockID:
		p2pMsg.Value = &tmtypes.QbftP2PMsg_ExpectBlock{ExpectBlock: msg.Msg.(*tmtypes.QbftExpectBlockMsg)}
	default:
		qbftlog.Error("unknown message type", "type", msg.TypeID)
		return nil
	}
	return p2pMsg
}

// p2p订阅消息
func (client *Client) SendPeerMsg(topic string, from string, pid string, msg []byte) error {
	data := &types.PeerMessage{Topic: topic, Module: moduleName, From: from, PeerId: pid, Msg: msg}
	_, err := client.sendP2PMsg(types.EventPeerMessage, data)
	return err
}

func (client *Client) SendFetchPeerList() (*types.PeerList, error) {
	msg := client.GetQueueClient().NewMessage("p2p", types.EventPeerInfo, nil)
	err := client.GetQueueClient().Send(msg, true)
	if err != nil {
		return nil, err
	}
	resp, err := client.GetQueueClient().Wait(msg)
	if err != nil {
		return nil, err
	}
	list, ok := resp.GetData().(*types.PeerList)
	if !ok {
		return nil, fmt.Errorf("not PeerList")
	}
	return list, err
}

func (client *Client) SendPubP2PMsg(topic string, msg []byte) error {
	data := &types.PublishTopicMsg{Topic: topic, Msg: msg}
	_, err := client.sendP2PMsg(types.EventPubTopicMsg, data)
	return err
}

func (client *Client) SendSubP2PTopic(topic string) error {
	data := &types.SubTopic{Topic: topic, Module: moduleName}
	_, err := client.sendP2PMsg(types.EventSubTopic, data)
	return err
}

func (client *Client) SendRmvP2PTopic(topic string) error {
	data := &types.RemoveTopic{Topic: topic, Module: moduleName}
	_, err := client.sendP2PMsg(types.EventRemoveTopic, data)
	return err

}

func (client *Client) SendFetchP2PTopic() (*types.TopicList, error) {
	data := &types.FetchTopicList{Module: moduleName}
	msg, err := client.sendP2PMsg(types.EventFetchTopics, data)
	if err != nil {
		return nil, errors.Wrap(err, "reply fail")
	}
	var reply types.TopicList
	err = types.Decode(msg, &reply)
	if err != nil {
		return nil, errors.Wrap(err, "decode fail")
	}
	return &reply, err
}

func (client *Client) sendP2PMsg(ty int64, data interface{}) ([]byte, error) {
	msg := client.GetQueueClient().NewMessage("p2p", ty, data)
	err := client.GetQueueClient().Send(msg, true)
	if err != nil {
		return nil, errors.Wrapf(err, "ty=%d", ty)
	}
	resp, err := client.GetQueueClient().Wait(msg)
	if err != nil {
		return nil, errors.Wrapf(err, "wait ty=%d", ty)
	}

	if resp.GetData().(*types.Reply).IsOk {
		return resp.GetData().(*types.Reply).Msg, nil
	}
	return nil, errors.Wrapf(types.ErrInvalidParam, "resp msg=%s", string(resp.GetData().(*types.Reply).GetMsg()))
}
