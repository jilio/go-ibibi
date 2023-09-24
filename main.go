package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/jilio/go-ibibi/internal/torrent"
)

// 0 - choke
// 1 - unchoke
// 2 - interested
// 3 - not interested
// 4 - have
// 5 - bitfield
// 6 - request
// 7 - piece
// 8 - cancel

type peerMessageType byte

const (
	choke peerMessageType = iota
	unchoke
	interested
	notInterested
	have
	bitfield
	request
	piece
	cancel
)

func main() {
	file, err := os.Open("sample.torrent")
	if err != nil {
		fmt.Println("failed to open file", err)
		os.Exit(1)
	}

	trnt, err := torrent.ReadFromFile(file)
	if err != nil {
		fmt.Println("failed to create torrent", err)
		os.Exit(1)
	}

	peers, err := trnt.GetPeers()
	if err != nil {
		fmt.Println("failed to get peers", err)
		os.Exit(1)
	}
	fmt.Println("peers:", peers)

	if len(peers) == 0 {
		fmt.Println("no peers")
		os.Exit(1)
	}

	conn, err := trnt.Handshake(peers[1])
	if err != nil {
		fmt.Println("failed to handshake", err)
		os.Exit(1)
	}

	err = awaitPeerMessage(conn, bitfield)
	if err != nil {
		fmt.Println("failed to await bitfield", err)
		os.Exit(1)
	}
	fmt.Println("bitfield received")

	err = sendPeerMessage(conn, interested, nil)
	if err != nil {
		fmt.Println("failed to send interested", err)
		os.Exit(1)
	}
	fmt.Println("interested sent")

	err = awaitPeerMessage(conn, unchoke)
	if err != nil {
		fmt.Println("failed to await unchoke", err)
		os.Exit(1)
	}
	fmt.Println("unchoke received, eto yspeh")

	msg := bytes.Buffer{}
	binary.Write(&msg, binary.BigEndian, uint32(0))     // 0 - index
	binary.Write(&msg, binary.BigEndian, uint32(0))     // 0 - begin
	binary.Write(&msg, binary.BigEndian, uint32(16384)) // 16384 - length

	err = sendPeerMessage(conn, request, msg.Bytes())
	if err != nil {
		fmt.Println("failed to send request", err)
		os.Exit(1)
	}
	fmt.Println("request sent")

	// 4. запрашиваем у него блоки
}

// todo: возвращать payload
func awaitPeerMessage(conn net.Conn, peerMessageType peerMessageType) error {
	var msgLen uint32
	err := binary.Read(conn, binary.BigEndian, &msgLen)
	if err != nil {
		return fmt.Errorf("failed to read packet length: %w", err)
	}
	fmt.Println("packet length:", msgLen, peerMessageType)

	var msgType byte
	err = binary.Read(conn, binary.BigEndian, &msgType)
	if err != nil {
		return fmt.Errorf("failed to read packet type: %w", err)
	}

	if msgType == byte(choke) {
		for {
			fmt.Println("choke received")
			time.Sleep(11 * time.Second)
			awaitPeerMessage(conn, unchoke) // todo: нужно чтобы за choke следил torrent
		}
	}

	if msgType != byte(peerMessageType) {
		return fmt.Errorf("unexpected peer message type: %d", msgType)
	}

	if msgLen == 1 {
		return nil
	}

	payload := make([]byte, msgLen)
	_, err = conn.Read(payload)
	if err != nil {
		fmt.Println("failed to read payload", err)
		os.Exit(1)
	}

	fmt.Println("payload:", len(payload))

	return nil
}

func sendPeerMessage(conn net.Conn, peerMessageType peerMessageType, payload []byte) error {
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

	_, err = conn.Write(msg.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write packet: %w", err)
	}

	return nil
}
