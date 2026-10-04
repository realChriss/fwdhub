//go:build ignore

package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
)

var sizes = []int{256, 64, 48, 32, 24, 16}

const (
	rtIcon      = 3
	rtGroupIcon = 14
	langEnUS    = 0x409

	relAMD64Addr32NB = 0x3
	relARM64Addr32NB = 0x2
	symClassStatic   = 3
	emptyStringTable = uint32(4)
)

type dirHeader struct {
	Characteristics, TimeDateStamp         uint32
	MajorVersion, MinorVersion, Named, IDs uint16
}
type dirEntry struct{ ID, Offset uint32 }
type dataEntry struct{ Offset, Size, CodePage, Reserved uint32 }

type groupHeader struct{ Reserved, Type, Count uint16 }
type groupEntry struct {
	Width, Height, Colors, Reserved uint8
	Planes, BitCount                uint16
	Size                            uint32
	ID                              uint16
}

type resType struct {
	id    uint32
	items [][]byte
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "winicon:", err)
		os.Exit(1)
	}
}

func run() error {
	f, err := os.Open("assets/icon.png")
	if err != nil {
		return err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return err
	}
	b := img.Bounds()
	if b.Dx() != b.Dy() || b.Dx() < sizes[0] {
		return fmt.Errorf("assets/icon.png must be square and at least %dpx", sizes[0])
	}
	src := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)

	var icons [][]byte
	group := put(nil, groupHeader{Type: 1, Count: uint16(len(sizes))})
	for i, n := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, resize(src, n)); err != nil {
			return err
		}
		icons = append(icons, buf.Bytes())
		group = put(group, groupEntry{Width: uint8(n), Height: uint8(n), Planes: 1, BitCount: 32, Size: uint32(buf.Len()), ID: uint16(i + 1)})
	}
	sec, relocs := rsrc([]resType{{rtIcon, icons}, {rtGroupIcon, [][]byte{group}}})

	for _, t := range []struct {
		arch             string
		machine, relType uint16
	}{
		{"amd64", pe.IMAGE_FILE_MACHINE_AMD64, relAMD64Addr32NB},
		{"arm64", pe.IMAGE_FILE_MACHINE_ARM64, relARM64Addr32NB},
	} {
		if err := os.WriteFile("src/rsrc_windows_"+t.arch+".syso", object(sec, relocs, t.machine, t.relType), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func rsrc(types []resType) (sec []byte, relocs []uint32) {
	const hdr, ent, langDir, dataEnt = 16, 8, 16 + 8, 16
	n := 0
	for _, t := range types {
		n += len(t.items)
	}
	idDirs := hdr + ent*len(types)
	langDirs := idDirs
	for _, t := range types {
		langDirs += hdr + ent*len(t.items)
	}
	entries := langDirs + n*langDir
	data := entries + n*dataEnt

	sec = put(sec, dirHeader{IDs: uint16(len(types))})
	off := idDirs
	for _, t := range types {
		sec = put(sec, dirEntry{t.id, 1<<31 | uint32(off)})
		off += hdr + ent*len(t.items)
	}
	k := 0
	for _, t := range types {
		sec = put(sec, dirHeader{IDs: uint16(len(t.items))})
		for i := range t.items {
			sec = put(sec, dirEntry{uint32(i + 1), 1<<31 | uint32(langDirs+k*langDir)})
			k++
		}
	}
	for i := range n {
		sec = put(sec, dirHeader{IDs: 1})
		sec = put(sec, dirEntry{langEnUS, uint32(entries + i*dataEnt)})
	}
	off = data
	for _, t := range types {
		for _, b := range t.items {
			relocs = append(relocs, uint32(len(sec)))
			sec = put(sec, dataEntry{Offset: uint32(off), Size: uint32(len(b))})
			off += align8(len(b))
		}
	}
	for _, t := range types {
		for _, b := range t.items {
			sec = append(sec, b...)
			sec = append(sec, make([]byte, align8(len(b))-len(b))...)
		}
	}
	return sec, relocs
}

func object(sec []byte, relocs []uint32, machine, relType uint16) []byte {
	const fileHdr, secHdr, reloc = 20, 40, 10
	name := [8]uint8{'.', 'r', 's', 'r', 'c'}
	out := put(nil, pe.FileHeader{
		Machine:              machine,
		NumberOfSections:     1,
		PointerToSymbolTable: uint32(fileHdr + secHdr + len(sec) + reloc*len(relocs)),
		NumberOfSymbols:      1,
	})
	out = put(out, pe.SectionHeader32{
		Name:                 name,
		SizeOfRawData:        uint32(len(sec)),
		PointerToRawData:     fileHdr + secHdr,
		PointerToRelocations: uint32(fileHdr + secHdr + len(sec)),
		NumberOfRelocations:  uint16(len(relocs)),
		Characteristics:      pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ,
	})
	out = append(out, sec...)
	for _, r := range relocs {
		out = put(out, pe.Reloc{VirtualAddress: r, Type: relType})
	}
	out = put(out, pe.COFFSymbol{Name: name, SectionNumber: 1, StorageClass: symClassStatic})
	return put(out, emptyStringTable)
}

func resize(src *image.RGBA, n int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, n, n))
	s := float64(src.Bounds().Dx()) / float64(n)
	for y := range n {
		for x := range n {
			var r, g, b, a, w float64
			for sy := int(float64(y) * s); float64(sy) < float64(y+1)*s; sy++ {
				wy := math.Min(float64(sy+1), float64(y+1)*s) - math.Max(float64(sy), float64(y)*s)
				for sx := int(float64(x) * s); float64(sx) < float64(x+1)*s; sx++ {
					wx := math.Min(float64(sx+1), float64(x+1)*s) - math.Max(float64(sx), float64(x)*s)
					c := src.RGBAAt(sx, sy)
					r, g, b, a, w = r+float64(c.R)*wx*wy, g+float64(c.G)*wx*wy, b+float64(c.B)*wx*wy, a+float64(c.A)*wx*wy, w+wx*wy
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r/w + .5), uint8(g/w + .5), uint8(b/w + .5), uint8(a/w + .5)})
		}
	}
	return dst
}

func align8(n int) int { return (n + 7) &^ 7 }

func put(b []byte, v any) []byte {
	b, err := binary.Append(b, binary.LittleEndian, v)
	if err != nil {
		panic(err)
	}
	return b
}
