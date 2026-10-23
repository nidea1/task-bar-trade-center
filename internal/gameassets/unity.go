// Package gameassets reads the bounded subset of Unity assets used by the game
// catalog. Unsupported formats fail explicitly and leave the saved catalog intact.
package gameassets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/pierrec/lz4/v4"
)

const maxAssetBytes = 128 << 20

type cursor struct {
	b     []byte
	p     int
	order binary.ByteOrder
	err   error
}

func (c *cursor) take(n int) []byte {
	if c.err != nil {
		return nil
	}
	if n < 0 || n > len(c.b)-c.p {
		c.err = io.ErrUnexpectedEOF
		return nil
	}
	b := c.b[c.p : c.p+n]
	c.p += n
	return b
}
func (c *cursor) u8() byte {
	b := c.take(1)
	if len(b) == 0 {
		return 0
	}
	return b[0]
}
func (c *cursor) u16() uint16 {
	b := c.take(2)
	if len(b) != 2 {
		return 0
	}
	return c.order.Uint16(b)
}
func (c *cursor) u32() uint32 {
	b := c.take(4)
	if len(b) != 4 {
		return 0
	}
	return c.order.Uint32(b)
}
func (c *cursor) u64() uint64 {
	b := c.take(8)
	if len(b) != 8 {
		return 0
	}
	return c.order.Uint64(b)
}
func (c *cursor) align(n int) { c.take((n - c.p%n) % n) }
func (c *cursor) count(max int) int {
	n := c.u32()
	if n > uint32(max) {
		c.err = fmt.Errorf("asset count %d exceeds %d", n, max)
		return 0
	}
	return int(n)
}
func (c *cursor) zstr() string {
	if c.err != nil {
		return ""
	}
	end := bytes.IndexByte(c.b[c.p:], 0)
	if end < 0 || end > 4096 {
		c.err = fmt.Errorf("invalid asset header string")
		return ""
	}
	s := string(c.take(end))
	c.take(1)
	return s
}
func (c *cursor) str() string { n := c.count(1 << 20); s := string(c.take(n)); c.align(4); return s }

type object struct {
	id    int64
	class int32
	data  []byte
	order binary.ByteOrder
}

func readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxAssetBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxAssetBytes {
		return nil, fmt.Errorf("asset exceeds size limit: %s", path)
	}
	return b, nil
}

func serializedObjects(b []byte) ([]object, error) {
	c := cursor{b: b, order: binary.BigEndian}
	c.u32()
	c.u32()
	version := c.u32()
	offset := uint64(c.u32())
	endian := c.u8()
	c.take(3)
	if version < 17 || version > 22 {
		return nil, fmt.Errorf("unsupported Unity serialized version %d", version)
	}
	if version >= 22 {
		c.u32()
		c.u64()
		offset = c.u64()
		c.u64()
	}
	if endian == 0 {
		c.order = binary.LittleEndian
	} else if endian != 1 {
		return nil, fmt.Errorf("invalid asset byte order")
	}
	c.zstr()
	c.u32()
	tree := c.u8() != 0
	classes := make([]int32, c.count(10000))
	for i := range classes {
		classes[i] = int32(c.u32())
		c.u8()
		c.u16()
		if classes[i] == 114 {
			c.take(16)
		}
		c.take(16)
		if tree {
			nodes, strings := c.count(200000), c.count(16<<20)
			stride := 24
			if version >= 19 {
				stride = 32
			}
			c.take(nodes * stride)
			c.take(strings)
			if version >= 21 {
				c.take(c.count(10000) * 4)
			}
		}
	}
	objects := make([]object, 0, c.count(200000))
	count := cap(objects)
	for i := 0; i < count && c.err == nil; i++ {
		c.align(4)
		id := int64(c.u64())
		start := uint64(c.u32())
		if version >= 22 {
			c.p -= 4
			start = c.u64()
		}
		size, typ := uint64(c.u32()), c.u32()
		if typ >= uint32(len(classes)) || offset > uint64(len(b)) || start > uint64(len(b))-offset || size > uint64(len(b))-offset-start {
			return nil, fmt.Errorf("invalid Unity object %d", id)
		}
		objects = append(objects, object{id, classes[typ], b[offset+start : offset+start+size], c.order})
	}
	if c.err != nil {
		return nil, c.err
	}
	return objects, nil
}

func decompress(raw []byte, size int, flags uint32) ([]byte, error) {
	if size < 0 || size > maxAssetBytes {
		return nil, fmt.Errorf("invalid Unity block size")
	}
	switch flags & 63 {
	case 0:
		if len(raw) != size {
			return nil, fmt.Errorf("Unity stored block size mismatch")
		}
		return raw, nil
	case 2, 3:
		out := make([]byte, size)
		n, err := lz4.UncompressBlock(raw, out)
		if err != nil {
			return nil, err
		}
		if n != size {
			return nil, fmt.Errorf("Unity LZ4 block size mismatch")
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported Unity compression %d", flags&63)
	}
}

func bundleObjects(b []byte) ([]object, error) {
	c := cursor{b: b, order: binary.BigEndian}
	if c.zstr() != "UnityFS" {
		return nil, fmt.Errorf("not a UnityFS bundle")
	}
	version := c.u32()
	if version < 6 || version > 8 {
		return nil, fmt.Errorf("unsupported UnityFS version %d", version)
	}
	c.zstr()
	c.zstr()
	length := c.u64()
	compressed, size, flags := c.u32(), c.u32(), c.u32()
	if length != uint64(len(b)) || compressed > uint32(len(b)) || size > maxAssetBytes || flags&0x100 != 0 {
		return nil, fmt.Errorf("invalid or encrypted UnityFS bundle")
	}
	if version >= 7 {
		c.align(16)
	}
	var raw []byte
	if flags&0x80 != 0 {
		raw = b[len(b)-int(compressed):]
	} else {
		raw = c.take(int(compressed))
	}
	if flags&0x200 != 0 {
		c.align(16)
	}
	if c.err != nil {
		return nil, c.err
	}
	info, err := decompress(raw, int(size), flags)
	if err != nil {
		return nil, err
	}
	ic := cursor{b: info, order: binary.BigEndian}
	ic.take(16)
	type block struct {
		size, compressed uint32
		flags            uint16
	}
	blocks := make([]block, ic.count(10000))
	total := uint64(0)
	for i := range blocks {
		blocks[i] = block{ic.u32(), ic.u32(), ic.u16()}
		total += uint64(blocks[i].size)
	}
	if total > maxAssetBytes {
		return nil, fmt.Errorf("UnityFS expanded size exceeds limit")
	}
	type node struct {
		offset, size uint64
		name         string
	}
	nodes := make([]node, ic.count(10000))
	for i := range nodes {
		nodes[i].offset, nodes[i].size = ic.u64(), ic.u64()
		ic.u32()
		nodes[i].name = ic.zstr()
	}
	if ic.err != nil {
		return nil, ic.err
	}
	data := make([]byte, 0, int(total))
	for _, block := range blocks {
		raw = c.take(int(block.compressed))
		if c.err != nil {
			return nil, c.err
		}
		part, err := decompress(raw, int(block.size), uint32(block.flags))
		if err != nil {
			return nil, err
		}
		data = append(data, part...)
	}
	var result []object
	for _, node := range nodes {
		if node.offset > uint64(len(data)) || node.size > uint64(len(data))-node.offset {
			return nil, fmt.Errorf("UnityFS node out of bounds")
		}
		if node.size < 20 || bytes.HasSuffix([]byte(node.name), []byte(".resS")) {
			continue
		}
		objects, err := serializedObjects(data[node.offset : node.offset+node.size])
		if err != nil {
			return nil, err
		}
		result = append(result, objects...)
	}
	return result, nil
}

func TextAssets(path string) (map[string][]byte, error) {
	b, err := readFile(path)
	if err != nil {
		return nil, err
	}
	objects, err := serializedObjects(b)
	if err != nil {
		return nil, err
	}
	result := map[string][]byte{}
	for _, o := range objects {
		if o.class != 49 {
			continue
		}
		c := cursor{b: o.data, order: o.order}
		name := c.str()
		n := c.count(16 << 20)
		text := c.take(n)
		if c.err != nil {
			return nil, c.err
		}
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("duplicate text asset %s", name)
		}
		result[name] = text
	}
	return result, nil
}

func monoHeader(o object) (string, *cursor) {
	c := &cursor{b: o.data, order: o.order}
	c.take(12)
	c.u8()
	c.align(4)
	c.take(12)
	return c.str(), c
}

// ItemNames joins Unity Localization shared IDs with ItemTable locale entries.
func ItemNames(sharedPath string, localePaths []string) (map[string]map[string]string, error) {
	b, err := readFile(sharedPath)
	if err != nil {
		return nil, err
	}
	objects, err := bundleObjects(b)
	if err != nil {
		return nil, err
	}
	keys := map[int64]string{}
	var sharedID int64
	for _, o := range objects {
		if o.class != 114 {
			continue
		}
		name, c := monoHeader(o)
		if name != "ItemTable Shared Data" {
			continue
		}
		if c.str() != "ItemTable" {
			return nil, fmt.Errorf("invalid shared ItemTable")
		}
		c.str()
		sharedID = o.id
		for n := c.count(100000); n > 0; n-- {
			id, key := int64(c.u64()), c.str()
			c.take(c.count(10000) * 8)
			if _, dup := keys[id]; dup {
				return nil, fmt.Errorf("duplicate localization ID")
			}
			keys[id] = key
		}
		if c.err != nil {
			return nil, c.err
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("ItemTable shared keys missing")
	}
	result := map[string]map[string]string{}
	englishSeen := false
	for _, path := range localePaths {
		b, err := readFile(path)
		if err != nil {
			return nil, err
		}
		objects, err := bundleObjects(b)
		if err != nil {
			return nil, err
		}
		for _, o := range objects {
			if o.class != 114 {
				continue
			}
			name, c := monoHeader(o)
			if len(name) < 10 || name[:10] != "ItemTable_" {
				continue
			}
			locale := c.str()
			if locale == "en-US" {
				englishSeen = true
			}
			c.u32()
			id := int64(c.u64())
			if id != sharedID || name != "ItemTable_"+locale {
				return nil, fmt.Errorf("ItemTable shared reference mismatch")
			}
			c.take(c.count(10000) * 8)
			for n := c.count(100000); n > 0; n-- {
				id, value := int64(c.u64()), c.str()
				c.take(c.count(10000) * 8)
				key, ok := keys[id]
				if !ok {
					continue // Removed localization entries can remain in older locale bundles.
				}
				if value == "" {
					continue
				}
				if result[key] == nil {
					result[key] = map[string]string{}
				}
				if previous, exists := result[key][locale]; exists && previous != value {
					return nil, fmt.Errorf("conflicting localization for %s/%s", key, locale)
				}
				result[key][locale] = value
			}
			if c.err != nil {
				return nil, c.err
			}
		}
	}
	if !englishSeen {
		return nil, fmt.Errorf("English ItemTable is missing")
	}
	return result, nil
}
