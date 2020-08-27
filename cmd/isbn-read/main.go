package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"

	_ "image/jpeg"
	_ "image/png"

	"github.com/sbinet/isbn"
)

func main() {
	log.SetPrefix("isbn-read: ")
	log.SetFlags(0)

	doDbg := flag.Bool("dbg", false, "enable debug output file")

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		log.Printf("missing intput file")
		os.Exit(1)
	}

	for _, fname := range flag.Args() {
		scan(fname, *doDbg)
	}
}

func scan(fname string, dbg bool) {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatalf("could not open file: %+v", err)
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		log.Fatalf("could not decode image file: %+v", err)
	}

	dec := isbn.NewDecoder()
	err = dec.Decode(src)
	if err != nil {
		debug(src, dec)
		log.Fatalf("could not decode barcode: %+v", err)
	}
	log.Printf("barcode: %v", dec.Barcode)

	if dbg {
		debug(src, dec)
	}
}

func debug(src image.Image, dec *isbn.Decoder) {
	var (
		red  = color.RGBA{R: 255, A: 255}
		blue = color.RGBA{B: 255, A: 255}
	)

	b := src.Bounds()
	mid := dec.Y
	tst := image.NewRGBA(src.Bounds())
	draw.Draw(tst, b, src, image.Point{}, draw.Src)
	for x, v := range dec.Line {
		var col color.Color
		switch {
		case v < 128:
			col = red
		default:
			col = blue
		}
		for ii := dec.Y - 10; ii < dec.Y+10; ii++ {
			tst.Set(x, ii, col)
		}
		// log.Printf("img(x=%d,y=%d)= %v | %v", x, y, dst.At(x, y), scn[x])
	}

	patch(tst, mid-20, dec.Guards[0][0])
	patch(tst, mid-40, dec.Guards[0][1])
	patch(tst, mid-60, dec.Guards[0][2])

	patch(tst, mid-20, dec.Guards[2][0])
	patch(tst, mid-40, dec.Guards[2][1])
	patch(tst, mid-60, dec.Guards[2][2])

	o, err := os.Create("out.png")
	if err != nil {
		log.Fatalf("could not create output file: %+v", err)
	}
	defer o.Close()

	err = png.Encode(o, tst)
	if err != nil {
		log.Fatalf("could not encode image to file: %+v", err)
	}
	err = o.Close()
	if err != nil {
		log.Fatalf("could not close output file: %+v", err)
	}
}

func patch(img draw.Image, y int, bar isbn.Bar) {
	c := color.RGBA{G: 255, A: 255}
	for j := y - 10; j < y+10; j++ {
		for i := bar.Beg; i < bar.End; i++ {
			img.Set(i, j, c)
		}
	}
}
