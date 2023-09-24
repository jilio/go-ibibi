package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/jilio/go-ibibi/internal/torrent"
	"github.com/jilio/go-ibibi/pkg/bencode"
)

func main() {
	file, err := os.Open("sample.torrent")
	if err != nil {
		fmt.Println("failed to open file", err)
		os.Exit(1)
	}

	buf := bufio.NewReader(file)
	decoded, err := bencode.Decode(buf)
	if err != nil {
		fmt.Println("failed to decode", err)
	}

	// 1. достаем инфу о торренте // todo: move to torrent package
	t := decoded.(map[string]interface{})
	fmt.Println("INFO", t["info"])

	if err != nil {
		fmt.Println("failed to read info string", err)
		os.Exit(1)
	}

	// todo: move to torrent package as newTorrent
	trnt := &torrent.Torrent{
		Announce: t["announce"].(string),
		Info:     t["info"].(map[string]interface{}),
	}

	peers, err := trnt.GetPeers()
	if err != nil {
		fmt.Println("failed to get peers", err)
		os.Exit(1)
	}

	fmt.Println("PEERS", peers)

	trnt.Handshake(peers[0])

	// 4. запрашиваем у него блоки
}
