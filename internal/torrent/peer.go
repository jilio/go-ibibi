package torrent

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

type Peer struct {
	Address   string
	conn      net.Conn
	sendCh    chan sendRequest
	choke     bool
	chokeCond *sync.Cond
	blockCh   chan Block
}

type sendRequest struct {
	msgType peerMessageType
	payload []byte
}

type peerMessageType byte

const (
	Choke peerMessageType = iota
	Unchoke
	Interested
	NotInterested
	Have
	Bitfield
	Request
	Piece
	Cancel
	Port

	None // incorrect, for tests
)

func NewPeer(address string, blockCh chan Block) *Peer {
	m := &sync.Mutex{}

	p := &Peer{
		Address:   address,
		sendCh:    make(chan sendRequest),
		chokeCond: sync.NewCond(m),
		blockCh:   blockCh,
	}

	go p.messageSender()

	return p
}

func (p *Peer) messageSender() {
	for req := range p.sendCh {
		p.chokeCond.L.Lock()
		for p.choke {
			p.chokeCond.Wait()
		}
		p.chokeCond.L.Unlock()

		err := p.doSendMessage(req.msgType, req.payload)
		if err != nil {
			fmt.Printf("Ошибка при отправке сообщения: %v\n", err)
		}
	}
}

func (p *Peer) messageListener() {

	for {
		msgType, payload, err := p.AwaitMessage()
		if err != nil {
			fmt.Printf("Ошибка при чтении сообщения: %v\n", err)
			p.conn.Close()
			return
		}

		if msgType == Piece {
			length := uint32(len(payload) - 8)
			pieceIndex := binary.BigEndian.Uint32(payload[:4])
			begin := binary.BigEndian.Uint32(payload[4:8])

			blockData := make([]byte, length)
			copy(blockData, payload[8:])

			fmt.Printf("Received block from piece %d, offset %d\n", pieceIndex, begin)

			p.blockCh <- Block{
				Index:  pieceIndex,
				Begin:  begin,
				Length: length,
				Data:   payload,
			}
		}
	}
}

func (p *Peer) Handshake(infoHash []byte) error {
	msg := bytes.Buffer{}

	// todo check errs when write
	msg.Write([]byte{19})
	msg.Write([]byte("BitTorrent protocol"))
	msg.Write(make([]byte, 8))
	msg.Write(infoHash)
	msg.Write([]byte("goibibi0000000000001")) // todo: move to const

	conn, err := net.Dial("tcp", p.Address)
	if err != nil {
		return err
	}
	p.conn = conn

	_, err = conn.Write(msg.Bytes())
	if err != nil {
		return err
	}

	peerResp := make([]byte, msg.Len())
	_, err = conn.Read(peerResp)
	if err != nil {
		return err
	}

	if peerResp[0] != 19 {
		return fmt.Errorf("peer responded with invalid protocol version")
	}

	if string(peerResp[1:20]) != "BitTorrent protocol" {
		return fmt.Errorf("peer responded with invalid protocol name")
	}

	fmt.Println("peer responded with valid handshake", string(peerResp[48:68]))

	go p.messageListener()

	return nil
}

// todo: возвращать payload
func (p *Peer) AwaitMessage() (peerMessageType, []byte, error) {
	var msgLen uint32
	err := binary.Read(p.conn, binary.BigEndian, &msgLen)
	if err != nil {
		return None, []byte{}, fmt.Errorf("failed to read packet length: %w", err)
	}

	var msgType byte
	err = binary.Read(p.conn, binary.BigEndian, &msgType)
	if err != nil {
		return None, []byte{}, fmt.Errorf("failed to read packet type: %w", err)
	}

	if msgType == byte(Choke) {
		p.chokeCond.L.Lock()
		p.choke = true
		p.chokeCond.L.Unlock()
	}

	if msgType == byte(Unchoke) {
		p.chokeCond.L.Lock()
		p.choke = false
		p.chokeCond.Signal()
		p.chokeCond.L.Unlock()
	}

	if msgLen == 1 {
		fmt.Printf("msgType: %v, no payload\n", peerMessageType(msgType))
		return peerMessageType(msgType), []byte{}, nil
	}

	payload := make([]byte, msgLen-1)
	n, err := io.ReadFull(p.conn, payload)
	if err != nil {
		fmt.Println("failed to read payload", err, n)
		os.Exit(1)
	}

	fmt.Printf("msgType: %v, payload len: %v\n", peerMessageType(msgType), len(payload))
	return peerMessageType(msgType), payload, nil
}

func (p *Peer) AwaitNonChokeMessage() (peerMessageType, error) {
	for {
		msgType, _, err := p.AwaitMessage()
		if err != nil {
			return None, err
		}

		if msgType == Choke {
			continue
		}

		if msgType == Unchoke {
			continue
		}

		return msgType, nil
	}
}

func (p *Peer) SendMessage(msgType peerMessageType, payload []byte) {
	p.sendCh <- sendRequest{
		msgType: msgType,
		payload: payload,
	}
}

func (p *Peer) doSendMessage(peerMessageType peerMessageType, payload []byte) error {
	fmt.Println("sending message", peerMessageType)
	msg := bytes.Buffer{}
	err := binary.Write(&msg, binary.BigEndian, uint32(len(payload)+1)) // +1 - тип сообщения
	if err != nil {
		return fmt.Errorf("failed to write packet length: %w", err)
	}

	err = binary.Write(&msg, binary.BigEndian, byte(peerMessageType))
	if err != nil {
		return fmt.Errorf("failed to write packet type: %w", err)
	}

	err = binary.Write(&msg, binary.BigEndian, payload)
	if err != nil {
		return fmt.Errorf("failed to write packet payload: %w", err)
	}

	_, err = p.conn.Write(msg.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write packet: %w", err)
	}

	return nil
}
