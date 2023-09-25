package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/jilio/go-ibibi/internal/torrent"
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

	// склеиваем блоки из канала blockCh
	go func() {
		resultFile, err := os.Create("result.fb2")
		if err != nil {
			fmt.Println("failed to create result file", err)
			os.Exit(1)
		}

		for {
			block := <-trnt.BlockCh
			fmt.Println("block received", block.Index, block.Begin, block.Length)
			resultFile.Write(block.Data)
		}
	}()

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

	// for _, peer := range peers {
	// 	fmt.Println("peer:", peer)
	// }

	peer := peers[3]
	err = peer.Handshake(trnt.InfoHash)
	if err != nil {
		fmt.Println("failed to handshake", err)
		os.Exit(1)
	}

	m, _, err := peer.AwaitMessage()
	if err != nil {
		fmt.Println("failed to await bitfield", err)
		os.Exit(1)
	}
	if m != torrent.Bitfield {
		fmt.Println("expected bitfield")
		os.Exit(1)
	}
	fmt.Println("bitfield received")

	peer.SendMessage(torrent.Interested, nil)
	fmt.Println("interested sent")

	fmt.Println("torrent info", trnt.Info)
	pieceLength := trnt.Info["piece length"].(int)
	size := trnt.Info["length"].(int)
	blockNum := pieceLength / 16384

	// todo: итерировать еще по частям
	for i := 0; i < int(blockNum); i += 1 {
		blockSize := 16384
		offset := i * blockSize
		end := (i + 1) * blockSize
		blockLength := min(end, size) - offset

		msg := bytes.Buffer{}
		binary.Write(&msg, binary.BigEndian, uint32(0))           // piece index
		binary.Write(&msg, binary.BigEndian, uint32(offset))      // begin
		binary.Write(&msg, binary.BigEndian, uint32(blockLength)) // length, <2^14

		peer.SendMessage(torrent.Request, msg.Bytes())
		fmt.Println("request sent", i)

		time.Sleep(1 * time.Second)
	}

	time.Sleep(10 * time.Second)
}
