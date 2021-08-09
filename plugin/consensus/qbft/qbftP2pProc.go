// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package qbft

import (
	"bytes"
	"context"
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

	maxRecvQueueSize = 1024
	maxPeersNum      = 1024
)

// P2PNode struct
type P2pNode struct {
	ID ID
	IP string

	state            *ConsensusState
	peerSet          *lru.Cache
	eventChannel     chan *tmtypes.QbftP2PMsg
	broadcastChannel chan MsgInfo
	unicastChannel   chan MsgInfo
	ctx              context.Context
}

// NewP2pNode method
func NewP2pNode(state *ConsensusState, ctx context.Context) *P2pNode {
	peerSet, err := lru.New(maxPeersNum)
	if err != nil {
		panic(err)
	}
	node := &P2pNode{
		state:            state,
		peerSet:          peerSet,
		eventChannel:     make(chan *tmtypes.QbftP2PMsg, maxRecvQueueSize),
		broadcastChannel: make(chan MsgInfo, maxRecvQueueSize),
		unicastChannel:   make(chan MsgInfo, maxRecvQueueSize),
		ctx:              ctx,
	}
	state.SetBroadcastChannel(node.broadcastChannel)
	state.SetUnicastChannel(node.unicastChannel)
	return node
}

// Start node
func (node *P2pNode) Start() {
	for {
		select {
		case <-node.ctx.Done():
			qbftlog.Info("P2pNode stop")
			return
		case msg := <-node.eventChannel:
			node.procMsg(msg)
		case msg := <-node.broadcastChannel:
			node.broadcast(msg)
		case msg := <-node.unicastChannel:
			peer, err := node.getPeer(msg.PeerID)
			if err != nil {
				qbftlog.Error("unicastChannel getPeer", "err", err)
				break
			}
			success := peer.Send(msg)
			if !success {
				qbftlog.Error("send failure in unicastChannel", "peerID", peer.ID())
			}
		}
	}
}

func (node *P2pNode) addPeer(peer *p2pPeer) error {
	err := peer.Start()
	if err != nil {
		return err
	}
	node.peerSet.Add(peer.ID(), peer)
	qbftlog.Info("add peer", "id", peer.ID(), "ip", peer.ip)
	stateMsg := MsgInfo{TypeID: ttypes.NewRoundStepID, Msg: node.state.RoundStateMessage(), PeerID: peer.ID(), PeerIP: peer.ip.String()}
	peer.Send(stateMsg)
	qbftlog.Info("Send state msg", "msg", stateMsg, "ourID", node.ID, "ourIP", node.IP)
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
			msg.PeerIP = peer.ip.String()
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
	peer, err := node.getPeer(ID(msg.PeerId))
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

// p2pPeer
type p2pPeer struct {
	ip net.IP
	id ID

	myState *ConsensusState
	state   *PeerConnState

	recvQueue        chan *tmtypes.QbftP2PMsg
	recvQueueSize    int32
	transferChannel  chan MsgInfo
	updateStateQueue chan MsgInfo
	heartbeatQueue   chan proto.Message

	ctx    context.Context
	cancel context.CancelFunc
}

// NewP2pPeer method
func NewP2pPeer(state *ConsensusState) *p2pPeer {
	peer := &p2pPeer{
		myState: state,
	}
	peer.SetTransferChannel(state.peerMsgQueue)
	return peer
}

func (pp *p2pPeer) Start() error {
	pp.recvQueue = make(chan *tmtypes.QbftP2PMsg, maxRecvQueueSize)
	//采用context来统一管理所有服务
	pp.ctx, pp.cancel = context.WithCancel(context.Background())

	go pp.recvRoutine()
	go pp.gossipDataRoutine()
	go pp.gossipVotesRoutine()
	go pp.queryMaj23Routine()

	return nil
}

func (pp *p2pPeer) Stop() {
	if pp.cancel != nil {
		pp.cancel()
	}
}

func (pp *p2pPeer) ID() ID {
	return pp.id
}

func (pp *p2pPeer) SetTransferChannel(transferChannel chan MsgInfo) {
	pp.transferChannel = transferChannel
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
	pp.transferChannel <- MsgInfo{ty, msg, pp.id, pp.ip.String()}
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
		vote := &ttypes.Vote{QbftVote: msg.GetVote()}
		pp.transferMsg(ty, vote)
		ps.SetHasVote(vote)
	case ttypes.ProposalBlockID:
		block := &ttypes.QbftBlock{QbftBlock: msg.GetBlock()}
		pp.transferMsg(ty, block)
		ps.SetHasProposalBlock(block)
	case ttypes.AggVoteID:
		aggVote := &ttypes.AggVote{QbftAggVote: msg.GetAggVote()}
		pp.transferMsg(ty, aggVote)
		ps.SetHasAggVote(aggVote)
	case ttypes.NewRoundStepID:
		ps.ApplyNewRoundStepMessage(msg.GetNewRoundStep())
	case ttypes.ValidBlockID:
		ps.ApplyValidBlockMessage(msg.GetValidBlock())
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
			pp.Send(MsgInfo{TypeID: ttypes.VoteSetBitsID, Msg: voteSetBitMsg, PeerID: pp.id, PeerIP: pp.ip.String()})
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
	default:
		qbftlog.Error("paracross ProcEvent not support", "ty", msg.GetTy())
	}

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
					msg := MsgInfo{TypeID: ttypes.ProposalBlockID, Msg: proposalBlock, PeerID: pp.id, PeerIP: pp.ip.String()}
					qbftlog.Info("Sending block for catchup", "peerIP", pp.ip.String(),
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
					msg := MsgInfo{TypeID: ttypes.ProposalID, Msg: rs.Proposal, PeerID: pp.id, PeerIP: pp.ip.String()}
					qbftlog.Debug(fmt.Sprintf("Sending proposal. Self myState: %v/%v/%v", rs.Height, rs.Round, rs.Step),
						"peerip", pp.ip.String(), "proposal-height", rs.Proposal.Height, "proposal-round", rs.Proposal.Round)
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
					}, PeerID: pp.id, PeerIP: pp.ip.String()}
					qbftlog.Debug("Sending POL", "height", prs.Height, "round", prs.Round)
					pp.Send(msg)
				}
				continue
			}

			// Send proposal block
			if rs.Proposal != nil && prs.ProposalBlockHash != nil && bytes.Equal(rs.Proposal.Blockhash, prs.ProposalBlockHash) {
				if rs.ProposalBlock != nil && !prs.ProposalBlock {
					msg := MsgInfo{TypeID: ttypes.ProposalBlockID, Msg: rs.ProposalBlock.QbftBlock, PeerID: pp.id, PeerIP: pp.ip.String()}
					qbftlog.Debug(fmt.Sprintf("Sending proposal block. Self myState: %v/%v/%v", rs.Height, rs.Round, rs.Step),
						"peerip", pp.ip.String(), "block-height", rs.ProposalBlock.Header.Height, "block-round", rs.ProposalBlock.Header.Round)
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
					qbftlog.Debug("Pick rs.LastCommit to send", "peerip", pp.ip.String(), "height", prs.Height)
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
						"peerip", pp.ip.String(), "peer(H/R/S)", fmt.Sprintf("%v/%v/%v", prs.Height, prs.Round, prs.Step))
					continue
				}
			}

			if sleeping == 0 {
				// We sent nothing. Sleep...
				sleeping = 1
				qbftlog.Debug("No votes to send, sleeping", "peerip", pp.ip.String(), "rs.Height", rs.Height, "prs.Height", prs.Height,
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
			qbftlog.Debug("Pick rs.LastCommit to send", "peerip", pp.ip.String(),
				"peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round))
			return true
		}
	}
	// If there are POL prevotes to send...
	if prs.Step <= ttypes.RoundStepPropose && prs.Round != -1 && prs.Round <= rs.Round && prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if pp.PickSendVote(polPrevotes) {
				qbftlog.Debug("Pick rs.Prevotes(prs.ProposalPOLRound) to send",
					"peerip", pp.ip.String(), "peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round),
					"POLRound", prs.ProposalPOLRound)
				return true
			}
		}
	}
	// If there are prevotes to send...
	if prs.Step <= ttypes.RoundStepPrevoteWait && prs.Round != -1 && prs.Round <= rs.Round {
		if pp.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
			qbftlog.Debug("Pick rs.Prevotes(prs.Round) to send",
				"peerip", pp.ip.String(), "peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round))
			return true
		}
	}
	// If there are precommits to send...
	if prs.Step <= ttypes.RoundStepPrecommitWait && prs.Round != -1 && prs.Round <= rs.Round {
		if pp.PickSendVote(rs.Votes.Precommits(prs.Round)) {
			qbftlog.Debug("Pick rs.Precommits(prs.Round) to send",
				"peerip", pp.ip.String(), "peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round))
			return true
		}
	}
	// If there are prevotes to send...Needed because of validBlock mechanism
	if prs.Round != -1 && prs.Round <= rs.Round {
		if pp.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
			qbftlog.Debug("Pick rs.Prevotes(prs.Round) to send",
				"peerip", pp.ip.String(), "peer(H/R)", fmt.Sprintf("%v/%v", prs.Height, prs.Round))
			return true
		}
	}
	// If there are POLPrevotes to send...
	if prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if pp.PickSendVote(polPrevotes) {
				qbftlog.Debug("Pick rs.Prevotes(prs.ProposalPOLRound) to send",
					"peerip", pp.ip.String(), "round", prs.ProposalPOLRound)
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
						}, PeerID: pp.id, PeerIP: pp.ip.String(),
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
						}, PeerID: pp.id, PeerIP: pp.ip.String(),
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
						}, PeerID: pp.id, PeerIP: pp.ip.String(),
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
						}, PeerID: pp.id, PeerIP: pp.ip.String(),
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
				msg := MsgInfo{TypeID: ttypes.AggVoteID, Msg: aggVote.QbftAggVote, PeerID: pp.id, PeerIP: pp.ip.String()}
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
		msg := MsgInfo{TypeID: ttypes.VoteID, Msg: vote.QbftVote, PeerID: pp.id, PeerIP: pp.ip.String()}
		qbftlog.Debug("Sending vote message", "msg", msg)
		if pp.Send(msg) {
			pp.state.SetHasVote(vote)
			return true
		}
	}
	return false
}

func (pp *p2pPeer) Send(msg MsgInfo) bool {
	p2pMsg := convertToP2pMsg(msg)
	if p2pMsg == nil {
		return false
	}
	bytes, err := proto.Marshal(p2pMsg)
	if err != nil {
		qbftlog.Error("p2pPeer marshal msg fail", "err", err)
		return false
	}
	err = pp.myState.client.SendPeerMsg(qbftMsgTopic, string(msg.PeerID), bytes)
	if err != nil {
		qbftlog.Error("p2pPeer send msg fail", "err", err)
		return false
	}
	return true
}

func convertToP2pMsg(msg MsgInfo) *tmtypes.QbftP2PMsg {
	p2pMsg := &tmtypes.QbftP2PMsg{Ty: int32(msg.TypeID), PeerId: string(msg.PeerID)}
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
	default:
		qbftlog.Error("unknown message type", "type", msg.TypeID)
		return nil
	}
	return p2pMsg
}

// p2p订阅消息
func (client *Client) SendPeerMsg(topic string, pid string, msg []byte) error {
	data := &types.PeerMessage{Topic: topic, Module: moduleName, PeerId: pid, Msg: msg}
	_, err := client.sendP2PMsg(types.EventPeerMessage, data)
	return err
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
