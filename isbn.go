// Copyright 2020 The isbn Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package isbn provides tools to read an ISBN barcode.
package isbn

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
)

const (
	whiteBar = 0
	blackBar = 255
)

func Scan(src image.Image) (Barcode, error) {
	dec := NewDecoder()
	err := dec.Decode(src)
	if err != nil {
		return nil, fmt.Errorf("could not scan barcode image: %w", err)
	}
	return dec.Barcode, nil
}

// Barcode is an ISBN barcode.
type Barcode []int

// Decoder decodes an image containing an ISBN barcode.
type Decoder struct {
	src image.Image

	Y      int       // Y coordinate of line of sight.
	Line   []byte    // Line is the raw byte line of sight. (0=white,255=black)
	Guards [3][3]Bar // Guards is the set of ISBN guards that have been detected

	Barcode Barcode // Barcode is the decoded ISBN barcode.
}

func NewDecoder() *Decoder {
	return &Decoder{}
}

func (dec *Decoder) Decode(src image.Image) error {
	dec.src = src
	dec.Y = src.Bounds().Max.Y / 2
	err := dec.scan()
	if err != nil {
		return fmt.Errorf("could not decode barcode image: %w", err)
	}
	return nil
}

func (dec *Decoder) scan() error {
	var (
		err error
		b   = dec.src.Bounds()
		mid = dec.Y
		dst = image.NewGray(b)
		scn = make([]byte, b.Max.X)
	)

	for y := 0; y < b.Max.Y; y++ {
		for x := 0; x < b.Max.X; x++ {
			pix := color.GrayModel.Convert(dec.src.At(x, y)).(color.Gray)
			dst.Set(x, y, pix)
			if y == mid {
				switch {
				case pix.Y > 128: // white
					scn[x] = whiteBar
				default:
					scn[x] = blackBar
				}
				// log.Printf("img(x=%d,y=%d)= %v | %v", x, y, dst.At(x, y), scn[x])
				dst.Set(x, y, color.Black)
			}
		}
	}

	dec.Line = scn

	var (
		black = []byte{blackBar}
		white = []byte{whiteBar}
	)

	// ISBNs (EAN-13) consist of a left guard (3 bars: black,white,black),
	// a middle guard (b,w,b) and a right guard (b,w,b).
	// [bwb] [6 pairs (w,b)] [w] [bwb] [6 pairs (w,b)] [w] [bwb]
	//
	// we first try to find the left guard.
	var (
		gl1 = &dec.Guards[0][0] // black
		gl2 = &dec.Guards[0][1] // white
		gl3 = &dec.Guards[0][2] // black
	)

	gl1.Beg = bytes.Index(scn, black)
	gl1.End = bytes.Index(scn[gl1.Beg:], white) + gl1.Beg

	gl2.Beg = bytes.Index(scn[gl1.End:], white) + gl1.End
	gl2.End = bytes.Index(scn[gl2.Beg:], black) + gl2.Beg

	gl3.Beg = bytes.Index(scn[gl2.End:], black) + gl2.End
	gl3.End = bytes.Index(scn[gl3.Beg:], white) + gl3.Beg

	// now try to find the right guard.
	var (
		gr1 = &dec.Guards[2][0]
		gr2 = &dec.Guards[2][1]
		gr3 = &dec.Guards[2][2]
	)
	gr3.End = bytes.LastIndex(scn, black) - 1
	gr3.Beg = bytes.LastIndex(scn[:gr3.End], white)

	gr2.End = bytes.LastIndex(scn[:gr3.Beg], white) - 1
	gr2.Beg = bytes.LastIndex(scn[:gr2.End], black)

	gr1.End = bytes.LastIndex(scn[:gr2.Beg], black) - 1
	gr1.Beg = bytes.LastIndex(scn[:gr1.End], white)

	bars := dec.linearize(scn[gl1.Beg:gr3.End])

	dec.Barcode, err = dec.digitize(bars)
	if err != nil {
		return fmt.Errorf("could not digitize bars pattern: %w", err)
	}

	return nil
}

func (dec *Decoder) linearize(raw []byte) []Bar {
	var (
		bars []Bar
		cur  *Bar
	)
	for i := range raw {
		v := int(raw[i] / 255)
		if i == 0 {
			bars = append(bars, Bar{
				Beg: 0,
				End: 0,
				Col: v,
			})
			cur = &bars[0]
			continue
		}
		switch v {
		case cur.Col:
			// still within same bar
			continue
		default:
			cur.End = i
			bars = append(bars, Bar{
				Beg: i,
				End: 0,
				Col: v,
			})
			cur = &bars[len(bars)-1]
			continue
		}
	}
	cur.End = len(raw)
	return bars
}

func (dec *Decoder) digitize(bars []Bar) (Barcode, error) {
	const (
		white = 0
		black = 1
	)

	var (
		err error
		beg = 0
		end = beg + 3
	)

	err = validate(bars[beg:end+1], []int{black, white, black, white})
	if err != nil {
		return nil, err
	}

	{
		var (
			b1 = bars[0]
			b2 = bars[1]
			b3 = bars[2]
		)
		mod := float64(b1.len()+b2.len()+b3.len()) / 3
		sli := bars[end : end+24]
		for i := 0; i < len(sli); i += 4 {
			sub := sli[i : i+4]
			tot := 0.0
			for _, bar := range sub {
				tot += float64(bar.len())
			}
			tot /= mod
			tot = math.Round(tot)
			if tot != 7 {
				return nil, fmt.Errorf(
					"could not decode first sequence: invalid digitization: got=%v, want=%v",
					tot, 7,
				)
			}
			out := make([]byte, 0, 7)
			for _, bar := range sub {
				n := int(math.Round(float64(bar.len()) / mod))
				for j := 0; j < n; j++ {
					out = append(out, byte(bar.Col))
				}
			}
			_, err := dec.decodeA(out)
			if err != nil {
				return nil, fmt.Errorf("could not decode first sequence: %w", err)
			}
			// log.Printf("bar[%d]: %v -> %d", i/4+1, out, v)
		}
	}

	beg = end + 24
	end = beg + 5
	err = validate(bars[beg:end], []int{white, black, white, black, white})
	if err != nil {
		return nil, err
	}
	{
		var (
			b1 = bars[beg+1]
			b2 = bars[beg+2]
			b3 = bars[beg+3]
		)
		mod := float64(b1.len()+b2.len()+b3.len()) / 3
		sli := bars[end : end+24]
		for i := 0; i < len(sli); i += 4 {
			sub := sli[i : i+4]
			tot := 0.0
			for _, bar := range sub {
				tot += float64(bar.len())
			}
			tot /= mod
			tot = math.Round(tot)
			if tot != 7 {
				return nil, fmt.Errorf(
					"could not decode second sequence: invalid digitization: got=%v, want=%v",
					tot, 7,
				)
			}
			out := make([]byte, 0, 7)
			for _, bar := range sub {
				n := int(math.Round(float64(bar.len()) / mod))
				for j := 0; j < n; j++ {
					out = append(out, byte(bar.Col))
				}
			}
			_, err := dec.decodeB(out)
			if err != nil {
				return nil, fmt.Errorf("could not decode second sequence: %w", err)
			}
			// log.Printf("bar[%d]: %v -> %d", i/4+1, out, v)
		}
	}

	err = validate(bars[len(bars)-4:], []int{white, black, white, black})
	if err != nil {
		return nil, err
	}

	return Barcode(dec.Barcode), nil
}

var codeL = map[string]int{
	string([]byte{0, 0, 0, 1, 1, 0, 1}): 0,
	string([]byte{0, 0, 1, 1, 0, 0, 1}): 1,
	string([]byte{0, 0, 1, 0, 0, 1, 1}): 2,
	string([]byte{0, 1, 1, 1, 1, 0, 1}): 3,
	string([]byte{0, 1, 0, 0, 0, 1, 1}): 4,
	string([]byte{0, 1, 1, 0, 0, 0, 1}): 5,
	string([]byte{0, 1, 0, 1, 1, 1, 1}): 6,
	string([]byte{0, 1, 1, 1, 0, 1, 1}): 7,
	string([]byte{0, 1, 1, 0, 1, 1, 1}): 8,
	string([]byte{0, 0, 0, 1, 0, 1, 1}): 9,
}

var codeG = map[string]int{
	string([]byte{0, 1, 0, 0, 1, 1, 1}): 0,
	string([]byte{0, 1, 1, 0, 0, 1, 1}): 1,
	string([]byte{0, 0, 1, 1, 0, 1, 1}): 2,
	string([]byte{0, 1, 0, 0, 0, 0, 1}): 3,
	string([]byte{0, 0, 1, 1, 1, 0, 1}): 4,
	string([]byte{0, 1, 1, 1, 0, 0, 1}): 5,
	string([]byte{0, 0, 0, 0, 1, 0, 1}): 6,
	string([]byte{0, 0, 1, 0, 0, 0, 1}): 7,
	string([]byte{0, 0, 0, 1, 0, 0, 1}): 8,
	string([]byte{0, 0, 1, 0, 1, 1, 1}): 9,
}

var codeR = map[string]int{
	string([]byte{1, 1, 1, 0, 0, 1, 0}): 0,
	string([]byte{1, 1, 0, 0, 1, 1, 0}): 1,
	string([]byte{1, 1, 0, 1, 1, 0, 0}): 2,
	string([]byte{1, 0, 0, 0, 0, 1, 0}): 3,
	string([]byte{1, 0, 1, 1, 1, 0, 0}): 4,
	string([]byte{1, 0, 0, 1, 1, 1, 0}): 5,
	string([]byte{1, 0, 1, 0, 0, 0, 0}): 6,
	string([]byte{1, 0, 0, 0, 1, 0, 0}): 7,
	string([]byte{1, 0, 0, 1, 0, 0, 0}): 8,
	string([]byte{1, 1, 1, 0, 1, 0, 0}): 9,
}

var codecs = []map[string]int{
	codeL, codeG, codeG, codeL, codeG, codeL,
}

func (dec *Decoder) decodeA(v []byte) (int, error) {
	var (
		i     = len(dec.Barcode)
		o, ok = codecs[i][string(v)]
	)
	if !ok {
		return 0, fmt.Errorf("invalid codec/value %q", v)
	}
	dec.Barcode = append(dec.Barcode, o)
	return o, nil
}

func (dec *Decoder) decodeB(v []byte) (int, error) {
	o, ok := codeR[string(v)]
	if !ok {
		return 0, fmt.Errorf("invalid codec/value %q", v)
	}
	dec.Barcode = append(dec.Barcode, o)
	return o, nil
}

func validate(bars []Bar, cols []int) error {
	for i := range bars {
		bar := bars[i]
		col := cols[i]
		if bar.Col != col {
			return fmt.Errorf("invalid bar[%d] color: got=%d, want=%d", i, bar.Col, col)
		}
	}
	return nil
}

type Bar struct {
	Beg int
	End int
	Col int
}

func newBar(raw []byte, off int, col int) Bar {
	var (
		left  []byte
		right []byte
	)
	switch col {
	case whiteBar:
		left = []byte{255}
		right = []byte{0}
	default:
		left = []byte{0}
		right = []byte{255}
	}

	beg := bytes.Index(raw[off:], left) + off
	end := bytes.Index(raw[beg:], right) + beg
	return Bar{Beg: beg, End: end, Col: col}
}

func (b Bar) len() int { return b.End - b.Beg }
