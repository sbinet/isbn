// Copyright 2020 The isbn Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package isbn

import (
	"image/png"
	"os"
	"reflect"
	"testing"
)

func TestScan(t *testing.T) {
	for _, tc := range []struct {
		name string
		want Barcode
	}{
		//		{
		//			name: "testdata/test-apue.png",
		//			want: Barcode{9, 7, 8, 0, 2, 0, 1, 4, 3, 3, 0, 7, 4},
		//		},
		{
			name: "testdata/test-gopl.png",
			want: Barcode{9, 7, 8, 0, 1, 3, 4, 1, 9, 0, 4, 4, 0},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.Open(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			img, err := png.Decode(f)
			if err != nil {
				t.Fatalf("could not decode PNG image: %+v", err)
			}

			bc, err := Scan(img)
			if err != nil {
				t.Fatalf("could not scan image: %+v", err)
			}

			if got, want := bc, tc.want; !reflect.DeepEqual(got, want) {
				t.Fatalf("invalid barcode:\ngot= %v\nwant=%v", got, want)
			}
		})
	}
}
