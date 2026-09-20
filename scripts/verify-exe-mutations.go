// Command verify-exe-mutations applies structural PE mutations used by
// scripts/verify-asset-mutations.py. It is deliberately standard-library only.
package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

const (
	rtIcon  = 3
	dirFlag = uint32(0x80000000)
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: verify-exe-mutations <strip-icon|hue-ico> <path>")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "strip-icon":
		err = stripIcon(os.Args[2])
	case "hue-ico":
		err = hueICO(os.Args[2])
	default:
		err = fmt.Errorf("unknown mutation %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func hueICO(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if len(raw) < 6 || binary.LittleEndian.Uint16(raw[2:4]) != 1 {
		return fmt.Errorf("%s is not an ICO file", path)
	}
	count := int(binary.LittleEndian.Uint16(raw[4:6]))
	type icoEntry struct {
		header  []byte
		payload []byte
	}
	entries := make([]icoEntry, 0, count)
	found := false
	for index := 0; index < count; index++ {
		entry := 6 + index*16
		if entry+16 > len(raw) {
			return fmt.Errorf("%s ICO directory is truncated", path)
		}
		size := int(binary.LittleEndian.Uint32(raw[entry+8 : entry+12]))
		offset := int(binary.LittleEndian.Uint32(raw[entry+12 : entry+16]))
		width := int(raw[entry])
		if width == 0 {
			width = 256
		}
		if offset > len(raw) || size > len(raw)-offset {
			return fmt.Errorf("%s %dpx ICO payload is out of range", path, width)
		}
		payload := append([]byte(nil), raw[offset:offset+size]...)
		header := append([]byte(nil), raw[entry:entry+16]...)
		if width != 64 {
			entries = append(entries, icoEntry{header: header, payload: payload})
			continue
		}
		found = true
		decoded, err := png.Decode(bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("decode %s 64px payload: %w", path, err)
		}
		mutated := image.NewNRGBA(decoded.Bounds())
		for y := decoded.Bounds().Min.Y; y < decoded.Bounds().Max.Y; y++ {
			for x := decoded.Bounds().Min.X; x < decoded.Bounds().Max.X; x++ {
				pixel := color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA)
				mutated.SetNRGBA(x, y, color.NRGBA{R: pixel.B, G: pixel.R, B: pixel.G, A: pixel.A})
			}
		}
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, mutated); err != nil {
			return fmt.Errorf("encode mutated 64px payload: %w", err)
		}
		entries = append(entries, icoEntry{header: header, payload: encoded.Bytes()})
	}
	if !found {
		return fmt.Errorf("%s has no 64px ICO entry", path)
	}
	var rebuilt bytes.Buffer
	rebuilt.Write(raw[:6])
	offset := 6 + 16*len(entries)
	for _, entry := range entries {
		header := append([]byte(nil), entry.header...)
		binary.LittleEndian.PutUint32(header[8:12], uint32(len(entry.payload)))
		binary.LittleEndian.PutUint32(header[12:16], uint32(offset))
		rebuilt.Write(header)
		offset += len(entry.payload)
	}
	for _, entry := range entries {
		rebuilt.Write(entry.payload)
	}
	return os.WriteFile(path, rebuilt.Bytes(), 0o644)
}

func stripIcon(path string) error {
	file, err := pe.Open(path)
	if err != nil {
		return fmt.Errorf("open %s as PE: %w", path, err)
	}
	section := file.Section(".rsrc")
	if section == nil {
		file.Close()
		return fmt.Errorf("%s carries no .rsrc section", path)
	}
	sectionOffset := int64(section.Offset)
	blob, err := section.Data()
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("read %s .rsrc: %w", path, err)
	}

	child, err := resourceTypeChild(blob, rtIcon)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if child+16 > uint32(len(blob)) {
		return fmt.Errorf("RT_ICON directory at %d runs past .rsrc", child)
	}
	idCount := binary.LittleEndian.Uint16(blob[child+14 : child+16])
	if idCount < 2 {
		return fmt.Errorf("RT_ICON has %d id entries; refusing to strip its last usable entry", idCount)
	}

	raw, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s for mutation: %w", path, err)
	}
	defer raw.Close()
	var encoded [2]byte
	binary.LittleEndian.PutUint16(encoded[:], idCount-1)
	if _, err := raw.WriteAt(encoded[:], sectionOffset+int64(child)+14); err != nil {
		return fmt.Errorf("write stripped RT_ICON count: %w", err)
	}
	return nil
}

func resourceTypeChild(blob []byte, want uint32) (uint32, error) {
	if len(blob) < 16 {
		return 0, fmt.Errorf("resource root is truncated")
	}
	count := int(binary.LittleEndian.Uint16(blob[12:14])) + int(binary.LittleEndian.Uint16(blob[14:16]))
	for index := 0; index < count; index++ {
		off := 16 + index*8
		if off+8 > len(blob) {
			return 0, fmt.Errorf("resource root entry %d is truncated", index)
		}
		name := binary.LittleEndian.Uint32(blob[off : off+4])
		child := binary.LittleEndian.Uint32(blob[off+4 : off+8])
		if name&dirFlag == 0 && name == want {
			if child&dirFlag == 0 {
				return 0, fmt.Errorf("RT_ICON root entry is not a directory")
			}
			return child &^ dirFlag, nil
		}
	}
	return 0, fmt.Errorf("RT_ICON resource directory is absent")
}
