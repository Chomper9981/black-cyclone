//go:build ignore

// genicons draws the tray icons procedurally (build/windows/icon.ico is still
// the stock Wails "W", black on white, so a greyscale variant of it would be
// indistinguishable):
//
//	connected.ico    - red disc with a white three-arm cyclone
//	disconnected.ico - the same in grey
//
// Each file holds 16/20/24/32/40/48/64 px PNG-compressed images.
// Run from internal/tray: go generate (or go run genicons.go)
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

var sizes = []int{16, 20, 24, 32, 40, 48, 64}

func main() {
	write("connected.ico", color.NRGBA{0xE5, 0x39, 0x35, 0xFF})
	write("disconnected.ico", color.NRGBA{0x8A, 0x8F, 0x96, 0xFF})
}

// shade returns coverage of (disc, white mark) at normalised coords x,y in [-1,1].
func shade(x, y float64) (disc, mark bool) {
	r := math.Hypot(x, y)
	if r > 0.96 {
		return false, false
	}
	if r < 0.17 {
		return true, true // eye
	}
	if r > 0.80 {
		return true, false
	}
	th := math.Atan2(y, x)
	// Three arms spiralling inward: phase advances with angle and log-radius.
	ph := 3*th/(2*math.Pi) + 1.6*math.Log(r)
	f := ph - math.Floor(ph)
	return true, f < 0.42
}

func draw(s int, bg color.NRGBA) image.Image {
	const ss = 8 // supersampling per axis
	img := image.NewNRGBA(image.Rect(0, 0, s, s))
	for py := 0; py < s; py++ {
		for px := 0; px < s; px++ {
			var nDisc, nMark int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px)+(float64(sx)+0.5)/ss)/float64(s)*2 - 1
					y := (float64(py)+(float64(sy)+0.5)/ss)/float64(s)*2 - 1
					d, m := shade(x, y)
					if d {
						nDisc++
						if m {
							nMark++
						}
					}
				}
			}
			if nDisc == 0 {
				continue
			}
			// Blend white mark over bg within the disc; alpha = disc coverage.
			t := float64(nMark) / float64(nDisc)
			mix := func(c uint8) uint8 { return uint8(float64(c)*(1-t) + 255*t + 0.5) }
			img.SetNRGBA(px, py, color.NRGBA{mix(bg.R), mix(bg.G), mix(bg.B), uint8(255 * nDisc / (ss * ss))})
		}
	}
	return img
}

// write encodes the icon at every size as a PNG-compressed .ico.
func write(name string, bg color.NRGBA) {
	var pngs [][]byte
	for _, s := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, draw(s, bg)); err != nil {
			log.Fatal(err)
		}
		pngs = append(pngs, buf.Bytes())
	}
	var out bytes.Buffer
	le := func(v any) { _ = binary.Write(&out, binary.LittleEndian, v) }
	le(uint16(0))
	le(uint16(1)) // type: icon
	le(uint16(len(sizes)))
	off := 6 + 16*len(sizes)
	for i, s := range sizes {
		out.WriteByte(uint8(s))
		out.WriteByte(uint8(s))
		out.WriteByte(0) // palette
		out.WriteByte(0) // reserved
		le(uint16(1))    // planes
		le(uint16(32))   // bpp
		le(uint32(len(pngs[i])))
		le(uint32(off))
		off += len(pngs[i])
	}
	for _, p := range pngs {
		out.Write(p)
	}
	if err := os.WriteFile(name, out.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", name, out.Len(), "bytes")
}
