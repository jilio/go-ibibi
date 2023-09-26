package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

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

	peer := peers[2]
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

	pieces := len(trnt.Info["pieces"].(string)) / 20
	pieceLength := trnt.Info["piece length"].(int)
	size := trnt.Info["length"].(int)
	maxBlockLength := 16384
	blockNum := pieceLength / maxBlockLength

	for i := 0; i < pieces; i += 1 {
		for j := 0; j < blockNum; j += 1 {
			offset := j * maxBlockLength
			left := size - (i*pieceLength + j*maxBlockLength)
			blockLength := min(maxBlockLength, left)

			if left <= 0 {
				break
			}

			msg := bytes.Buffer{}
			binary.Write(&msg, binary.BigEndian, uint32(i))           // piece index
			binary.Write(&msg, binary.BigEndian, uint32(offset))      // begin
			binary.Write(&msg, binary.BigEndian, uint32(blockLength)) // length, <2^14

			peer.SendMessage(torrent.Request, msg.Bytes())
			fmt.Printf("progress: %.2f\n", float32(i*pieceLength+offset+blockLength)/float32(size))

			time.Sleep(100 * time.Millisecond) // todo: ограничить кол-во запросов в секунду
		}
	}

	wg.Wait()
}
