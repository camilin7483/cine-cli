package resolver

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

type Megacloud struct {
	client *http.Client
}

func NewMegacloud() *Megacloud {
	return &Megacloud{
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    5,
				IdleConnTimeout: 30 * time.Second,
			},
		},
	}
}

func (m *Megacloud) Name() string    { return "megacloud" }
func (m *Megacloud) Available() bool { return true }

func (m *Megacloud) Resolve(ctx context.Context, url string) (*StreamResult, error) {
	return nil, fmt.Errorf("megacloud: embed URL required, use ResolveEmbed")
}

func (m *Megacloud) ResolveEmbed(ctx context.Context, embedID string) (*core.Stream, error) {
	embedURL := fmt.Sprintf("https://megacloud.tv/embed-2/e-1/%s", embedID)
	req, err := http.NewRequestWithContext(ctx, "GET", embedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Referer", "https://megacloud.tv/")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("megacloud fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}

	html := string(body)
	return m.extractFromHTML(html)
}

func (m *Megacloud) extractFromHTML(html string) (*core.Stream, error) {
	encryptedData, key, err := findMegacloudData(html)
	if err != nil {
		return nil, err
	}

	decrypted, err := decryptMegacloud(encryptedData, key)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	var sources []struct {
		File string `json:"file"`
		Type string `json:"type"`
	}
	var tracks []struct {
		File    string `json:"file"`
		Kind    string `json:"kind"`
		Label   string `json:"label"`
		Default bool   `json:"default"`
	}

	var data struct {
		Sources []struct {
			File string `json:"file"`
			Type string `json:"type"`
		} `json:"sources"`
		Tracks []struct {
			File    string `json:"file"`
			Kind    string `json:"kind"`
			Label   string `json:"label"`
			Default bool   `json:"default"`
		} `json:"tracks"`
	}

	if err := json.Unmarshal(decrypted, &data); err != nil {
		sources = append(sources, data.Sources...)
		tracks = append(tracks, data.Tracks...)
		_ = sources
		_ = tracks
	}

	var streamURL string
	if len(data.Sources) > 0 {
		streamURL = data.Sources[0].File
	}
	if streamURL == "" && len(data.Sources) > 0 {
		for _, s := range data.Sources {
			if s.File != "" {
				streamURL = s.File
				break
			}
		}
	}

	if streamURL == "" {
		return nil, fmt.Errorf("megacloud: no source found in decrypted data")
	}

	var subtitles []core.Subtitle
	for _, t := range data.Tracks {
		if t.Kind == "captions" {
			subtitles = append(subtitles, core.Subtitle{
				URL:  t.File,
				Lang: t.Label,
			})
		}
	}

	if !strings.HasPrefix(streamURL, "http") {
		if strings.Contains(string(decrypted), "http") {
			idx := strings.Index(string(decrypted), "http")
			end := strings.Index(string(decrypted)[idx:], `"`)
			if end > 0 {
				streamURL = string(decrypted)[idx : idx+end]
			} else {
				streamURL = string(decrypted)[idx:]
				streamURL = strings.Split(streamURL, `"`)[0]
			}
		}
	}

	return &core.Stream{
		URL:       streamURL,
		Referer:   "https://megacloud.tv/",
		UserAgent: httpUA,
		Subtitles: subtitles,
		IsM3U8:    strings.Contains(streamURL, ".m3u8"),
		Provider:  "megacloud",
	}, nil
}

var megaEncRe = regexp.MustCompile(`encrypted-([A-Za-z0-9+/=]+)`)
var megaKeyRe = regexp.MustCompile(`\b([0-9a-f]{64})\b`)
var megaJSONSource = regexp.MustCompile(`"file"\s*:\s*"([^"]+\.m3u8[^"]*)"`)

func findMegacloudData(html string) ([]byte, []byte, error) {
	if m := megaEncRe.FindStringSubmatch(html); len(m) > 1 {
		enc, _ := base64.StdEncoding.DecodeString(m[1])
		if enc != nil {
			for _, km := range megaKeyRe.FindAllStringSubmatch(html, -1) {
				key := hexToBytes(km[1])
				if len(key) >= 16 {
					return enc, key[:16], nil
				}
			}
		}
	}

	if m := megaJSONSource.FindStringSubmatch(html); len(m) > 1 {
		return nil, nil, fmt.Errorf("direct:m3u8:%s", m[1])
	}

	return nil, nil, fmt.Errorf("megacloud: no encrypted data found")
}

func decryptMegacloud(data, key []byte) ([]byte, error) {
	if len(key) < 16 {
		padded := make([]byte, 16)
		copy(padded, key)
		key = padded
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	ciphertext = pkcs7Unpad(ciphertext)
	if ciphertext == nil {
		var buf bytes.Buffer
		buf.Write(data[aes.BlockSize:])
		mode.CryptBlocks(buf.Bytes(), buf.Bytes())
		ciphertext = pkcs7Unpad(buf.Bytes())
	}
	return ciphertext, nil
}

func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	padLen := int(data[len(data)-1])
	if padLen > len(data) || padLen > aes.BlockSize {
		return nil
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil
		}
	}
	return data[:len(data)-padLen]
}

func hexToBytes(s string) []byte {
	s = strings.TrimSpace(s)
	if len(s)%2 != 0 {
		s = "0" + s
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s)/2; i++ {
		hi := hexVal(s[2*i])
		lo := hexVal(s[2*i+1])
		b[i] = hi<<4 | lo
	}
	return b
}

func hexVal(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
