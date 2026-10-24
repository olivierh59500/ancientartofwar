// Package disk reads the original DOS disk without an operating-system mount.
package disk

import (
	"encoding/binary"
	"fmt"
	"strings"
)

type File struct {
	Name string
	Data []byte
}

// Read decodes a FAT12 root directory and follows each file's cluster chain.
// The input is read-only; returned files own their bytes.
func Read(image []byte) ([]File, error) {
	if len(image) < 512 {
		return nil, fmt.Errorf("FAT12: truncated boot sector")
	}
	u16 := func(p int) int { return int(binary.LittleEndian.Uint16(image[p:])) }
	sector, cluster, reserved, fats, roots, fatSize := u16(11), int(image[13]), u16(14), int(image[16]), u16(17), u16(22)
	if sector < 128 || sector > 4096 || sector&(sector-1) != 0 || cluster == 0 || cluster&(cluster-1) != 0 || fats == 0 || fatSize == 0 || roots == 0 {
		return nil, fmt.Errorf("FAT12: invalid BIOS parameter block")
	}
	root := (reserved + fats*fatSize) * sector
	data := root + ((roots*32+sector-1)/sector)*sector
	fatStart, fatEnd := reserved*sector, (reserved+fatSize)*sector
	if root < fatEnd || data > len(image) || fatEnd > len(image) || root+roots*32 > len(image) {
		return nil, fmt.Errorf("FAT12: directory outside disk")
	}
	fat, clusterSize := image[fatStart:fatEnd], sector*cluster
	var files []File
	for i := 0; i < roots; i++ {
		e := image[root+i*32 : root+(i+1)*32]
		if e[0] == 0 {
			break
		}
		if e[0] == 0xe5 || e[11]&0x18 != 0 || e[11] == 0x0f {
			continue
		}
		name := strings.TrimRight(string(e[:8]), " ")
		if ext := strings.TrimRight(string(e[8:11]), " "); ext != "" {
			name += "." + ext
		}
		size := int(binary.LittleEndian.Uint32(e[28:]))
		if size > len(image) {
			return nil, fmt.Errorf("FAT12: %s is larger than disk", name)
		}
		buf := make([]byte, 0, size)
		c, seen := int(binary.LittleEndian.Uint16(e[26:])), make(map[int]bool)
		for len(buf) < size {
			if c < 2 || c >= 0xff0 || seen[c] {
				return nil, fmt.Errorf("FAT12: invalid cluster chain for %s at %03x", name, c)
			}
			seen[c] = true
			off := data + (c-2)*clusterSize
			if off < data || off+clusterSize > len(image) || c*3/2+1 >= len(fat) {
				return nil, fmt.Errorf("FAT12: cluster outside disk for %s", name)
			}
			buf = append(buf, image[off:off+min(clusterSize, size-len(buf))]...)
			v := int(binary.LittleEndian.Uint16(fat[c*3/2:]))
			if c&1 == 0 {
				c = v & 0xfff
			} else {
				c = v >> 4
			}
		}
		files = append(files, File{Name: name, Data: buf})
	}
	return files, nil
}
