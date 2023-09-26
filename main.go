package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jilio/go-ibibi/internal/torrent"
)

func main() {
	filename := os.Args[1]
	file, err := os.Open(filename)
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
	fmt.Println("peers:", len(peers))

	if len(peers) == 0 {
		fmt.Println("no peers")
		os.Exit(1)
	}

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

	pieces := len(trnt.Info["pieces"].(string)) / 20
	pieceLength := trnt.Info["piece length"].(int)
	filesize := trnt.Info["length"].(int)
	maxBlockLength := 16384
	blocks := pieceLength / maxBlockLength

	for piece := 0; piece < pieces; piece += 1 {
		for block := 0; block < blocks; block += 1 {
			blockOffset := block * maxBlockLength
			left := filesize - (piece*pieceLength + block*maxBlockLength)
			blockLength := min(maxBlockLength, left)

			if blockLength <= 0 {
				break
			}

			peer.RequestPiece(uint32(piece), uint32(blockOffset), uint32(blockLength))
			fmt.Printf("progress: %.2f\n", float32(piece*pieceLength+blockOffset+blockLength)/float32(filesize))

			time.Sleep(100 * time.Millisecond) // todo: ограничить кол-во запросов в секунду
		}
	}

	wg.Wait()
}
