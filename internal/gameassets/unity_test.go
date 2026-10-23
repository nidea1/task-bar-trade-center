package gameassets

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/pierrec/lz4/v4"
)

func put32(b *bytes.Buffer, n uint32) { binary.Write(b, binary.LittleEndian, n) }
func put64(b *bytes.Buffer, n uint64) { binary.Write(b, binary.LittleEndian, n) }
func putString(b *bytes.Buffer, s string) {
	put32(b, uint32(len(s)))
	b.WriteString(s)
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
}

func serializedFixture(class int32, payload []byte) []byte {
	b := new(bytes.Buffer)
	b.WriteString("6000.0.62f1\x00")
	put32(b, 5)
	b.WriteByte(0)
	put32(b, 1)
	put32(b, uint32(class))
	b.WriteByte(0)
	binary.Write(b, binary.LittleEndian, int16(-1))
	if class == 114 {
		b.Write(make([]byte, 16))
	}
	b.Write(make([]byte, 16))
	put32(b, 1)
	for (48+b.Len())%4 != 0 {
		b.WriteByte(0)
	}
	put64(b, 42)
	put64(b, 0)
	put32(b, uint32(len(payload)))
	put32(b, 0)
	header := new(bytes.Buffer)
	for _, v := range []uint32{0, 0, 22, 0} {
		binary.Write(header, binary.BigEndian, v)
	}
	header.Write(make([]byte, 4))
	binary.Write(header, binary.BigEndian, uint32(b.Len()))
	binary.Write(header, binary.BigEndian, uint64(256+len(payload)))
	binary.Write(header, binary.BigEndian, uint64(256))
	binary.Write(header, binary.BigEndian, uint64(0))
	header.Write(b.Bytes())
	header.Write(make([]byte, 256-header.Len()))
	header.Write(payload)
	return header.Bytes()
}

func bundleFixture(payload []byte) []byte {
	info := new(bytes.Buffer)
	info.Write(make([]byte, 16))
	binary.Write(info, binary.BigEndian, uint32(1))
	binary.Write(info, binary.BigEndian, uint32(len(payload)))
	binary.Write(info, binary.BigEndian, uint32(len(payload)))
	binary.Write(info, binary.BigEndian, uint16(0))
	binary.Write(info, binary.BigEndian, uint32(1))
	binary.Write(info, binary.BigEndian, uint64(0))
	binary.Write(info, binary.BigEndian, uint64(len(payload)))
	binary.Write(info, binary.BigEndian, uint32(0))
	info.WriteString("CAB-test\x00")
	h := new(bytes.Buffer)
	h.WriteString("UnityFS\x00")
	binary.Write(h, binary.BigEndian, uint32(6))
	h.WriteString("6000\x006000\x00")
	lengthOffset := h.Len()
	binary.Write(h, binary.BigEndian, uint64(0))
	binary.Write(h, binary.BigEndian, uint32(info.Len()))
	binary.Write(h, binary.BigEndian, uint32(info.Len()))
	binary.Write(h, binary.BigEndian, uint32(0))
	h.Write(info.Bytes())
	h.Write(payload)
	out := h.Bytes()
	binary.BigEndian.PutUint64(out[lengthOffset:], uint64(len(out)))
	return out
}

func monoFixture(name string) *bytes.Buffer {
	b := new(bytes.Buffer)
	b.Write(make([]byte, 12))
	b.WriteByte(1)
	b.Write(make([]byte, 3))
	b.Write(make([]byte, 12))
	putString(b, name)
	return b
}

func TestReadUnityTextAssetAndLocalizedNames(t *testing.T) {
	dir := t.TempDir()
	text := new(bytes.Buffer)
	putString(text, "ItemInfoData")
	put32(text, 3)
	text.WriteString("csv")
	path := filepath.Join(dir, "sharedassets0.assets")
	os.WriteFile(path, serializedFixture(49, text.Bytes()), 0600)
	assets, err := TextAssets(path)
	if err != nil || string(assets["ItemInfoData"]) != "csv" {
		t.Fatalf("assets=%v err=%v", assets, err)
	}
	shared := monoFixture("ItemTable Shared Data")
	putString(shared, "ItemTable")
	putString(shared, "guid")
	put32(shared, 1)
	put64(shared, 1007)
	putString(shared, "ItemName_200")
	put32(shared, 0)
	locale := monoFixture("ItemTable_en-US")
	putString(locale, "en-US")
	put32(locale, 2)
	put64(locale, 42)
	put32(locale, 0)
	put32(locale, 1)
	put64(locale, 1007)
	putString(locale, "New Gem")
	put32(locale, 0)
	sharedPath, localePath := filepath.Join(dir, "shared.bundle"), filepath.Join(dir, "en.bundle")
	os.WriteFile(sharedPath, bundleFixture(serializedFixture(114, shared.Bytes())), 0600)
	os.WriteFile(localePath, bundleFixture(serializedFixture(114, locale.Bytes())), 0600)
	names, err := ItemNames(sharedPath, []string{localePath})
	if err != nil || names["ItemName_200"]["en-US"] != "New Gem" {
		t.Fatalf("names=%v err=%v", names, err)
	}
}

func TestLZ4AndMalformedAssets(t *testing.T) {
	source := bytes.Repeat([]byte("localized item name "), 500)
	compressed := make([]byte, lz4.CompressBlockBound(len(source)))
	n, err := lz4.CompressBlock(source, compressed, nil)
	if err != nil || n == 0 {
		t.Fatal("fixture compression failed", err)
	}
	actual, err := decompress(compressed[:n], len(source), 2)
	if err != nil || !bytes.Equal(actual, source) {
		t.Fatal("LZ4 roundtrip failed", err)
	}
	if _, err := decompress(compressed[:n], len(source)-1, 2); err == nil {
		t.Fatal("accepted wrong expanded size")
	}
	for _, data := range [][]byte{nil, []byte("UnityFS\x00"), bytes.Repeat([]byte{255}, 80), serializedFixture(49, nil)[:70]} {
		if _, err := serializedObjects(data); err == nil {
			t.Fatal("accepted malformed serialized asset")
		}
		if _, err := bundleObjects(data); err == nil {
			t.Fatal("accepted malformed bundle")
		}
	}
}

func FuzzUnityAssetReaders(f *testing.F) {
	f.Add([]byte("UnityFS\x00"))
	f.Add(serializedFixture(49, []byte{0, 0, 0, 0, 0, 0, 0, 0}))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		serializedObjects(data)
		bundleObjects(data)
	})
}
