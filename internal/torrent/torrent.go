package torrent

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jilio/go-ibibi/pkg/bencode"
)

const (
	PieceHashLen = 20
	MaxBlockLen  = 16384
)

type Torrent struct {
	Announce    string
	BlockCh     chan Block
	Blocks      int
	Info        map[string]interface{}
	InfoHash    []byte
	Length      int
	Name        string
	Pieces      [][]byte
	PieceLength int
}

type Block struct {
	Index  uint32
	Begin  uint32
	Length uint32
	Data   []byte
}

func ReadFromFile(file *os.File) (*Torrent, error) {
	buf := bufio.NewReader(file)
	decoded, err := bencode.Decode(buf)
	if err != nil {
		return nil, err
	}

	t := decoded.(map[string]interface{})
	info, ok := t["info"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to read info string")
	}

	infoHash, err := getInfoHash(info)
	if err != nil {
		return nil, err
	}

	announce, ok := t["announce"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to read announce string")
	}

	length, ok := info["length"].(int)
	if !ok {
		return nil, fmt.Errorf("failed to read length")
	}

	name, ok := info["name"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to read name")
	}

	pieceLength, ok := info["piece length"].(int)
	if !ok {
		return nil, fmt.Errorf("failed to read piece length")
	}
	blocks := pieceLength / MaxBlockLen

	piecesStr, ok := info["pieces"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to read pieces string")
	}

	pieces := [][]byte{}
	for i := 0; i < len(piecesStr); i += PieceHashLen {
		pieces = append(pieces, []byte(piecesStr[i:i+PieceHashLen]))
	}

	return &Torrent{
		Announce:    announce,
		BlockCh:     make(chan Block, 10), // why 10?
		Blocks:      blocks,
		Info:        info,
		InfoHash:    infoHash,
		Length:      length,
		Name:        name,
		PieceLength: pieceLength,
		Pieces:      pieces,
	}, nil
}

func (t *Torrent) GetPeers() ([]*Peer, error) {
	client := &http.Client{Timeout: 10 * time.Second} // todo: move to torrent
	params := url.Values{
		"info_hash":  []string{string(t.InfoHash)},
		"peer_id":    []string{"goibibi0000000000001"}, // todo: move to const
		"port":       []string{"6881"},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{"0"},
		"compact":    []string{"1"},
	}
	uri := fmt.Sprintf("%s?%s", t.Announce, params.Encode())

	resp, err := client.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBuf := bufio.NewReader(resp.Body)
	tr, err := bencode.Decode(bodyBuf)
	if err != nil {
		fmt.Println("failed to decode response body", err)
		os.Exit(1)
	}

	peers := []*Peer{}
	peersStr := tr.(map[string]interface{})["peers"].(string) // todo: check

	for i := 0; i < len(peersStr); i += 6 {
		ip := fmt.Sprintf("%d.%d.%d.%d", peersStr[i], peersStr[i+1], peersStr[i+2], peersStr[i+3])
		port := int(peersStr[i+4])<<8 + int(peersStr[i+5])

		peers = append(peers, NewPeer(fmt.Sprintf("%s:%d", ip, port), t.BlockCh))
	}

	return peers, nil
}

func getInfoHash(info interface{}) ([]byte, error) {
	hash := sha1.New()
	err := bencode.Marshal(hash, info)
	if err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}
