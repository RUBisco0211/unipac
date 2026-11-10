//go:build ignore

// icongen derives the macOS icon layer from the app's flat icon.
//
// assets/icon.png is the icon as it ships to Windows and Linux: a finished
// drawing of a cream background with the package on it. An Icon Composer
// document, assets/AppIcon.icon, needs that artwork without its background,
// since the document paints the background itself and paints it dark when
// the system asks for the dark appearance.
//
// icongen cuts the background away and writes the artwork alone to
// assets/AppIcon.icon/Assets/package.png. Run it from the repository root
// after changing assets/icon.png:
//
//	go run scripts/icongen/main.go
//
// It grows the background inward from the border of the image, following the
// background's own gradient and stopping where the colour jumps, which is the
// artwork's edge. The tape over the package stays: it is enclosed by the
// package, and its edge against the lid is a jump of its own.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

// neighbours are the four pixels the background grows into.
var neighbours = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

func main() {
	src := flag.String("src", "assets/icon.png", "the flat icon to cut the background from")
	dst := flag.String("dst", "assets/AppIcon.icon/Assets/package.png", "the artwork alone")
	tol := flag.Int("tolerance", 5, "how far one pixel of the background may differ from its neighbour")
	flag.Parse()

	artwork, err := cutBackground(*src, *tol)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(*dst)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, artwork); err != nil {
		log.Fatal(err)
	}
	b := artwork.Bounds()
	kept := 0
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if _, _, _, a := artwork.At(x, y).RGBA(); a > 0 {
				kept++
			}
		}
	}
	fmt.Printf("%s: %d of %d pixels kept (%.1f%%)\n", *dst, kept, b.Dx()*b.Dy(), 100*float64(kept)/float64(b.Dx()*b.Dy()))
}

// cutBackground returns img's artwork on transparency: the pixels the
// background, grown from the border, did not reach.
func cutBackground(path string, tol int) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	px := make([]color.NRGBA, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px[y*w+x] = color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
		}
	}

	// The background is a light, smooth gradient, and the artwork is not:
	// a candidate joins the background when it is light and differs from
	// the pixel it grows from by no more than tol, channel by channel.
	light := func(c color.NRGBA) bool { return c.R >= 250 && c.G >= 220 && c.B >= 180 }
	near := func(a, b color.NRGBA) bool {
		return abs(a.R, b.R) <= tol && abs(a.G, b.G) <= tol && abs(a.B, b.B) <= tol
	}
	bg := make([]bool, w*h)
	var queue []int
	push := func(i int) {
		if !bg[i] {
			bg[i] = true
			queue = append(queue, i)
		}
	}
	for x := 0; x < w; x++ {
		push(x)
		push((h-1)*w + x)
	}
	for y := 0; y < h; y++ {
		push(y * w)
		push(y*w + w - 1)
	}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		x, y := i%w, i/w
		for _, d := range neighbours {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			j := ny*w + nx
			if bg[j] {
				continue
			}
			if px[j].A == 0 { // already transparent, outside the rounded square
				push(j)
				continue
			}
			if px[i].A == 0 || !light(px[j]) || !near(px[i], px[j]) {
				continue
			}
			push(j)
		}
	}

	artwork := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i, c := range px {
		if !bg[i] {
			artwork.SetNRGBA(i%w, i/w, c)
		}
	}
	return artwork, nil
}

func abs(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}
