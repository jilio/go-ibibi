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

	t, err := torrent.ReadFromFile(file)
	if err != nil {
		fmt.Println("failed to create torrent", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		resultFile, err := os.Create(t.Name)
		if err != nil {
			fmt.Println("failed to create result file", err)
			os.Exit(1)
		}

		for {
			block := <-t.BlockCh
			fmt.Println("block received", block.Index, block.Begin, block.Length)
			resultFile.Write(block.Data)
		}
	}()

	peers, err := t.GetPeers()
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
	err = peer.Handshake(t.InfoHash)
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

	fmt.Println("torrent info", t.Info)

	for piece := 0; piece < len(t.Pieces); piece += 1 {
		for block := 0; block < t.Blocks; block += 1 {
			blockOffset := block * torrent.MaxBlockLen
			left := t.Length - (piece*t.PieceLength + block*torrent.MaxBlockLen)
			blockLength := min(torrent.MaxBlockLen, left)

			if blockLength <= 0 {
				break
			}

			peer.RequestPiece(uint32(piece), uint32(blockOffset), uint32(blockLength))
			fmt.Printf("progress: %.2f\n", float32(piece*t.PieceLength+blockOffset+blockLength)/float32(t.Length))

			time.Sleep(150 * time.Millisecond) // todo: ограничить кол-во запросов в секунду
		}
	}

	wg.Wait()
}
