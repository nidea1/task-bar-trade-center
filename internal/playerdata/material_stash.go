package playerdata

// Material stacks are read from the current player's named save-data list.
// Shared unique IDs use the same deduplication as inventory/equipped items.
func (resolver *Resolver) readMaterialStash(memory Memory, player uintptr, seen map[uint64]struct{}) ([]OwnedItem, bool) {
	offset := resolver.instanceFields(memory, readPtr(memory, player))["materialStashSaveDatas"]
	if offset == 0 {
		return nil, true
	}
	listPointer, pointerOK := memory.ReadUintptr(player + offset)
	if !pointerOK {
		return nil, false
	}
	if listPointer == 0 {
		return nil, true
	}
	list := readObjectListInfo(memory, player, offset, 10000)
	if !list.ok {
		return nil, false
	}
	var result []OwnedItem
	for i := 0; i < list.size; i++ {
		object, ok := listObjectAt(memory, list, i)
		if !ok {
			return nil, false
		}
		f := resolver.instanceFields(memory, readPtr(memory, object))
		if f["ItemKey"] == 0 || f["ItemUniqueId"] == 0 || f["Quantity"] == 0 {
			return nil, false
		}
		key, keyOK := memory.ReadInt32(object + f["ItemKey"])
		unique, uniqueOK := memory.ReadUint64(object + f["ItemUniqueId"])
		quantity, quantityOK := memory.ReadInt32(object + f["Quantity"])
		if !keyOK || !uniqueOK || !quantityOK {
			return nil, false
		}
		if key <= 0 || unique == 0 || quantity <= 0 {
			continue
		}
		if _, exists := seen[unique]; exists {
			continue
		}
		seen[unique] = struct{}{}
		item := resolver.ownedItem(int(key), unique, LocationMaterialStash, 0, -1)
		item.Quantity = int(quantity)
		result = append(result, item)
	}
	return result, true
}
