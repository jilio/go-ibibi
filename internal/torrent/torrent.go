package torrent

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jilio/go-ibibi/pkg/bencode"
)

type Torrent struct {
	Announce string
	Info     map[string]interface{}
}

type Peer string

func (t *Torrent) GetPeers() ([]Peer, error) {
	hash, err := t.GetInfoHash()
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second} // todo: move to torrent
	params := url.Values{
		"info_hash":  []string{string(hash)},
		"peer_id":    []string{"go-ibibi000000000000"},
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

	peers := []Peer{}
	peersStr := tr.(map[string]interface{})["peers"].(string) // todo: check
	for i := 0; i < len(peersStr); i += 20 {
		ip := fmt.Sprintf("%d.%d.%d.%d", peersStr[i], peersStr[i+1], peersStr[i+2], peersStr[i+3])
		port := int(peersStr[i+4])<<8 + int(peersStr[i+5])

		peers = append(peers, Peer(fmt.Sprintf("%s:%d", ip, port)))
	}

	return peers, nil
}

func (t *Torrent) GetInfoHash() ([]byte, error) {
	infoHash := sha1.New()
	infoHash.Write([]byte(fmt.Sprintf("%v", t.Info)))
	return infoHash.Sum(nil), nil
}

func (t *Torrent) Handshake(peer Peer) error {
	hash, err := t.GetInfoHash() // todo DRY
	if err != nil {
		return err
	}

	msg := bytes.Buffer{}
	// todo check errs
	msg.Write([]byte{19})
	msg.Write([]byte("BitTorrent protocol"))
	msg.Write(make([]byte, 8))
	msg.Write(hash)
	msg.Write([]byte("go-ibibi000000000000")) // todo: move to const

	fmt.Println("MSG", msg.Bytes())

	// надо как-то соединиться с пиром

	return nil
}
