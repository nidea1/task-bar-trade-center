package game

import (
	"encoding/binary"
	"math"
)

// UnityRectBounds contains screen-space canvas coordinates with Y pointing up.
type UnityRectBounds struct {
	MinX, MinY, MaxX, MaxY float64
}

// ReadRectTransformBounds reconstructs RectTransform.GetWorldCorners from the
// game's native transform hierarchy. It reads memory without invoking game code.
// These native offsets were verified against TaskBarHero 1.3.0 (metadata v31).
func ReadRectTransformBounds(processHandle, transform uintptr) (UnityRectBounds, bool) {
	var rect [16]byte
	var access [12]byte
	if !readProcessBytes(processHandle, transform+0xA8, rect[:]) ||
		!readProcessBytes(processHandle, transform+0x28, access[:]) {
		return UnityRectBounds{}, false
	}
	x, y := unityFloat(rect[:], 0), unityFloat(rect[:], 4)
	w, h := unityFloat(rect[:], 8), unityFloat(rect[:], 12)
	if !finiteUnityValues(x, y, w, h) || w <= 0 || h <= 0 || w > 10000 || h > 10000 {
		return UnityRectBounds{}, false
	}
	data := uintptr(binary.LittleEndian.Uint64(access[:8]))
	index := int32(binary.LittleEndian.Uint32(access[8:]))
	var hierarchy [24]byte
	if data == 0 || !readProcessBytes(processHandle, data+0x10, hierarchy[:]) {
		return UnityRectBounds{}, false
	}
	count := int32(binary.LittleEndian.Uint32(hierarchy[:4]))
	matrices := uintptr(binary.LittleEndian.Uint64(hierarchy[8:16]))
	parents := uintptr(binary.LittleEndian.Uint64(hierarchy[16:24]))
	if count <= 0 || count > 1<<20 || index < 0 || index >= count || matrices == 0 || parents == 0 {
		return UnityRectBounds{}, false
	}
	points := [4][3]float64{{x, y, 0}, {x, y + h, 0}, {x + w, y + h, 0}, {x + w, y, 0}}
	for depth := 0; depth < 64; depth++ {
		var trs [48]byte
		var parentBytes [4]byte
		if !readProcessBytes(processHandle, matrices+uintptr(index)*48, trs[:]) ||
			!readProcessBytes(processHandle, parents+uintptr(index)*4, parentBytes[:]) {
			return UnityRectBounds{}, false
		}
		tx, ty, tz := unityFloat(trs[:], 0), unityFloat(trs[:], 4), unityFloat(trs[:], 8)
		qx, qy, qz, qw := unityFloat(trs[:], 16), unityFloat(trs[:], 20), unityFloat(trs[:], 24), unityFloat(trs[:], 28)
		sx, sy, sz := unityFloat(trs[:], 32), unityFloat(trs[:], 36), unityFloat(trs[:], 40)
		if !finiteUnityValues(tx, ty, tz, qx, qy, qz, qw, sx, sy, sz) ||
			math.Abs(qx*qx+qy*qy+qz*qz+qw*qw-1) > 0.01 {
			return UnityRectBounds{}, false
		}
		for i, p := range points {
			px, py, pz := p[0]*sx, p[1]*sy, p[2]*sz
			// Rotate the scaled point by the unit quaternion, then translate.
			cx, cy, cz := qy*pz-qz*py, qz*px-qx*pz, qx*py-qy*px
			points[i] = [3]float64{
				px + 2*(qw*cx+qy*cz-qz*cy) + tx,
				py + 2*(qw*cy+qz*cx-qx*cz) + ty,
				pz + 2*(qw*cz+qx*cy-qy*cx) + tz,
			}
		}
		parent := int32(binary.LittleEndian.Uint32(parentBytes[:]))
		if parent == -1 {
			bounds := UnityRectBounds{points[0][0], points[0][1], points[0][0], points[0][1]}
			for _, p := range points {
				if !finiteUnityValues(p[:]...) {
					return UnityRectBounds{}, false
				}
				bounds.MinX, bounds.MinY = math.Min(bounds.MinX, p[0]), math.Min(bounds.MinY, p[1])
				bounds.MaxX, bounds.MaxY = math.Max(bounds.MaxX, p[0]), math.Max(bounds.MaxY, p[1])
			}
			return bounds, bounds.MaxX-bounds.MinX >= 1 && bounds.MaxY-bounds.MinY >= 1
		}
		if parent < 0 || parent >= count || parent == index {
			return UnityRectBounds{}, false
		}
		index = parent
	}
	return UnityRectBounds{}, false
}

func unityFloat(data []byte, offset int) float64 {
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(data[offset : offset+4])))
}

func finiteUnityValues(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e7 {
			return false
		}
	}
	return true
}
