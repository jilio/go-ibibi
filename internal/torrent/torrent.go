package torrent

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jilio/go-ibibi/pkg/bencode"
)

type Torrent struct {
	Announce string
	Info     map[string]interface{}
	InfoHash []byte
}

type Peer string

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

	return &Torrent{
		Announce: t["announce"].(string),
		Info:     info,
		InfoHash: infoHash,
	}, nil
}

func (t *Torrent) GetPeers() ([]Peer, error) {
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

	peers := []Peer{}
	peersStr := tr.(map[string]interface{})["peers"].(string) // todo: check

	for i := 0; i < len(peersStr); i += 6 {
		ip := fmt.Sprintf("%d.%d.%d.%d", peersStr[i], peersStr[i+1], peersStr[i+2], peersStr[i+3])
		port := int(peersStr[i+4])<<8 + int(peersStr[i+5])

		peers = append(peers, Peer(fmt.Sprintf("%s:%d", ip, port)))
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

func (t *Torrent) Handshake(peer Peer) (net.Conn, error) {
	msg := bytes.Buffer{}

	// todo check errs when write
	msg.Write([]byte{19})
	msg.Write([]byte("BitTorrent protocol"))
	msg.Write(make([]byte, 8))
	msg.Write(t.InfoHash)
	msg.Write([]byte("goibibi0000000000001")) // todo: move to const

	conn, err := net.Dial("tcp", string(peer))
	if err != nil {
		return nil, err
	}

	_, err = conn.Write(msg.Bytes())
	if err != nil {
		return nil, err
	}

	peerResp := make([]byte, msg.Len())
	_, err = conn.Read(peerResp)
	if err != nil {
		return nil, err
	}

	if peerResp[0] != 19 {
		return nil, fmt.Errorf("peer responded with invalid protocol version")
	}

	if string(peerResp[1:20]) != "BitTorrent protocol" {
		return nil, fmt.Errorf("peer responded with invalid protocol name")
	}

	fmt.Println("peer responded with valid handshake", string(peerResp[48:68]))

	return conn, nil
}
