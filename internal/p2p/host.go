package p2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const TopicName = "erst-simulation-state"

type Host struct {
	node   host.Host
	ps     *pubsub.PubSub
	topic  *pubsub.Topic
	sub    *pubsub.Subscription
	ctx    context.Context
	cancel context.CancelFunc
}

func NewHost(listenPort int) (*Host, error) {
	ctx, cancel := context.WithCancel(context.Background())

	addr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)
	node, err := libp2p.New(libp2p.ListenAddrStrings(addr))
	if err != nil {
		cancel()
		return nil, err
	}

	ps, err := pubsub.NewGossipSub(ctx, node)
	if err != nil {
		node.Close()
		cancel()
		return nil, err
	}

	topic, err := ps.Join(TopicName)
	if err != nil {
		node.Close()
		cancel()
		return nil, err
	}

	return &Host{
		node:   node,
		ps:     ps,
		topic:  topic,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (h *Host) FullAddrs() []string {
	var addrs []string
	idStr := fmt.Sprintf("/p2p/%s", h.node.ID())
	for _, a := range h.node.Addrs() {
		addrs = append(addrs, a.String()+idStr)
	}
	return addrs
}

func (h *Host) Connect(peerAddr string) error {
	maddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return err
	}

	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return err
	}

	return h.node.Connect(h.ctx, *info)
}

func (h *Host) Broadcast(data []byte) error {
	return h.topic.Publish(h.ctx, data)
}

func (h *Host) Subscribe() (<-chan []byte, error) {
	if h.sub == nil {
		sub, err := h.topic.Subscribe()
		if err != nil {
			return nil, err
		}
		h.sub = sub
	}

	ch := make(chan []byte)
	go func() {
		defer close(ch)
		for {
			msg, err := h.sub.Next(h.ctx)
			if err != nil {
				return
			}
			if msg.ReceivedFrom != h.node.ID() {
				ch <- msg.Data
			}
		}
	}()

	return ch, nil
}

func (h *Host) Close() error {
	h.cancel()
	if h.sub != nil {
		h.sub.Cancel()
	}
	if h.topic != nil {
		h.topic.Close()
	}
	return h.node.Close()
}
