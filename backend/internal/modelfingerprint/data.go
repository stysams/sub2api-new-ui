package modelfingerprint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

func decode(data []byte) (*Artifact, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("detector header is missing")
	}
	size := int(binary.LittleEndian.Uint32(data))
	start := (4 + size + 3) / 4 * 4
	if size <= 0 || start > len(data) || (len(data)-start)%4 != 0 {
		return nil, fmt.Errorf("invalid detector header")
	}
	count := (len(data) - start) / 4
	var value any
	if err := json.Unmarshal(data[4:4+size], &value); err != nil {
		return nil, err
	}
	var expand func(any) (any, error)
	expand = func(v any) (any, error) {
		switch x := v.(type) {
		case map[string]any:
			if offset, ok := x["$f32"].(float64); ok {
				length, ok := x["n"].(float64)
				if !ok || offset < 0 || length < 0 || offset != math.Trunc(offset) || length != math.Trunc(length) || offset+length > float64(count) {
					return nil, fmt.Errorf("invalid float array bounds")
				}
				out := make([]float64, int(length))
				for i := range out {
					p := int(offset) + i
					bits := uint32(data[start+p]) | uint32(data[start+count+p])<<8 | uint32(data[start+2*count+p])<<16 | uint32(data[start+3*count+p])<<24
					out[i] = float64(math.Float32frombits(bits))
					if math.IsInf(out[i], 0) || math.IsNaN(out[i]) {
						return nil, fmt.Errorf("invalid float value")
					}
				}
				return out, nil
			}
			for k, a := range x {
				b, e := expand(a)
				if e != nil {
					return nil, e
				}
				x[k] = b
			}
			return x, nil
		case []any:
			for i, a := range x {
				b, e := expand(a)
				if e != nil {
					return nil, e
				}
				x[i] = b
			}
			return x, nil
		}
		return v, nil
	}
	expanded, err := expand(value)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(expanded)
	if err != nil {
		return nil, err
	}
	var a Artifact
	err = json.Unmarshal(raw, &a)
	return &a, err
}

type Snapshot struct {
	Bank     Bank
	Artifact *Artifact
	BankJSON []byte
	Detector []byte
	SHA256   string
}

func NewSnapshot(bankJSON, detector []byte) (*Snapshot, error) {
	var b Bank
	if err := json.Unmarshal(bankJSON, &b); err != nil {
		return nil, err
	}
	if b.Schema != "lmfpd-data-v1" {
		return nil, fmt.Errorf("unsupported bank schema")
	}
	file, ok := b.Files["shared_detector.bin"]
	if !ok {
		return nil, fmt.Errorf("detector manifest missing")
	}
	hash := sha256.Sum256(detector)
	digest := hex.EncodeToString(hash[:])
	if digest != file.SHA256 {
		return nil, fmt.Errorf("detector SHA256 does not match manifest")
	}
	a, err := Decode(detector)
	if err != nil {
		return nil, err
	}
	if _, err = a.Analyze(nil, b); err != nil {
		return nil, err
	}
	return &Snapshot{Bank: b, Artifact: a, BankJSON: bankJSON, Detector: detector, SHA256: digest}, nil
}

var Sources = []string{"https://lm.ikale.io/data/", "https://cdn.jsdmirror.com/npm/lmfpd-data@latest/"}

func Download(ctx context.Context) (*Snapshot, error) {
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(url string, limit int64) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("reference HTTP %d", res.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
		if int64(len(b)) > limit {
			return nil, fmt.Errorf("reference size limit exceeded")
		}
		return b, err
	}
	var failures []string
	for _, source := range Sources {
		snapshot, err := func() (*Snapshot, error) {
			bankJSON, err := get(source+"bundle.json", 4<<20)
			if err != nil {
				return nil, err
			}
			var bank Bank
			if err = json.Unmarshal(bankJSON, &bank); err != nil {
				return nil, err
			}
			file, ok := bank.Files["shared_detector.bin"]
			if !ok || len(file.Parts) == 0 || len(file.Parts) > 128 {
				return nil, fmt.Errorf("invalid reference manifest")
			}
			var compressed bytes.Buffer
			for _, part := range file.Parts {
				if strings.ContainsAny(part, "/\\?#") || part == "." || part == ".." || part == "" {
					return nil, fmt.Errorf("invalid chunk name")
				}
				chunk, err := get(source+"chunks/"+part, 8<<20)
				if err != nil {
					return nil, err
				}
				compressed.Write(chunk)
				if compressed.Len() > 32<<20 {
					return nil, fmt.Errorf("compressed reference size exceeded")
				}
			}
			decoder, err := zstd.NewReader(bytes.NewReader(compressed.Bytes()), zstd.WithDecoderMaxMemory(128<<20))
			if err != nil {
				return nil, err
			}
			defer decoder.Close()
			data, err := io.ReadAll(io.LimitReader(decoder, 64<<20+1))
			if err != nil {
				return nil, err
			}
			if len(data) > 64<<20 {
				return nil, fmt.Errorf("reference too large")
			}
			return NewSnapshot(bankJSON, data)
		}()
		if err == nil {
			return snapshot, nil
		}
		failures = append(failures, err.Error())
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf("reference download failed: %s", strings.Join(failures, "; "))
}
