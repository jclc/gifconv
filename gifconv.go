package gifconv

import (
	"image"
	"image/color"
	"image/gif"
	"sync"

	"github.com/ericpauley/go-quantize/quantize"
	"golang.org/x/exp/constraints"
	"golang.org/x/image/draw"
)

func GifToRgba(g *gif.GIF) []*image.RGBA {
	job := func(area image.Rectangle, tpindex uint8, in *image.Paletted, background, out *image.RGBA, wg *sync.WaitGroup) {
		for x := area.Min.X; x < area.Max.X; x++ {
			for y := area.Min.Y; y < area.Max.Y; y++ {
				ind := in.ColorIndexAt(x, y)
				if ind == tpindex {
					// pixel is transparent; copy value from background
					out.SetRGBA(x, y, background.RGBAAt(x, y))
				} else {
					// pixel is opaque; copy pixel value from in to out and background
					col := in.At(x, y)
					out.Set(x, y, col)
					background.Set(x, y, col)
				}
			}
		}
		if wg != nil {
			wg.Done()
		}
	}

	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	background := image.NewRGBA(bounds)
	outputs := make([]*image.RGBA, 0, len(g.Image))

	for i := range g.Image {

		// Find the transparent index if it exists
		tpindex := g.BackgroundIndex
		// if g.Disposal[i] == gif.DisposalPrevious {
		// 	tpindex = int(g.BackgroundIndex)
		// }
		// for i, col := range g.Image[i].Palette {
		// 	if _, _, _, a := col.RGBA(); a == 0 {
		// 		tpindex = i
		// 	}
		// }
		// fmt.Printf("tpindex@%d: %d\n", i, tpindex)
		// fmt.Printf("color@%d: %v\n", i, g.Image[i].At(0, 0))
		// fmt.Printf("index@%d: %v\n", i, g.Image[i].ColorIndexAt(0, 0))
		// fmt.Printf("disposal@%d, %v\n", i, g.Disposal[i])

		outputs = append(outputs, image.NewRGBA(bounds))
		inBounds := g.Image[i].Bounds()
		if inBounds != bounds {
			// copy parts of the background that are outside the current frame
			if topHalf := image.Rect(
				bounds.Min.X, bounds.Min.Y,
				bounds.Max.X, inBounds.Min.Y); !topHalf.Empty() {
				draw.Draw(outputs[i], topHalf, background, bounds.Min, draw.Src)
			}
			if leftSide := image.Rect(
				bounds.Min.X, inBounds.Min.Y,
				inBounds.Min.X, inBounds.Max.Y); !leftSide.Empty() {
				draw.Draw(outputs[i], leftSide, background, image.Pt(bounds.Min.X, inBounds.Min.Y), draw.Src)
			}
			if rightSide := image.Rect(
				inBounds.Max.X, inBounds.Min.Y,
				bounds.Max.X, inBounds.Max.Y); !rightSide.Empty() {
				draw.Draw(outputs[i], rightSide, background, image.Pt(inBounds.Max.X, inBounds.Min.Y), draw.Src)
			}
			if bottomHalf := image.Rect(
				bounds.Min.X, inBounds.Max.Y,
				bounds.Max.X, bounds.Max.Y); !bottomHalf.Empty() {
				draw.Draw(outputs[i], bottomHalf, background, image.Pt(bounds.Min.X, inBounds.Max.Y), draw.Src)
			}
			// draw.Copy(&outputs[i], image.Pt(0, 0), g.Image[i], g.Image[i].Bounds(), draw.Src, nil)
			// continue
			// draw.Draw(&outputs[i], bounds, background, image.Pt(0, 0), draw.Src)
		}

		// Draw the new frame on the new image and the background
		if inBounds.Dx() >= 8 && inBounds.Dy() >= 8 {
			// Split to jobs
			areas := []image.Rectangle{
				{
					Min: inBounds.Min,
					Max: image.Pt(inBounds.Min.X+inBounds.Dx()/2, inBounds.Min.Y+inBounds.Dy()/2),
				},
				{
					Min: image.Pt(inBounds.Min.X, inBounds.Min.Y+inBounds.Dy()/2),
					Max: image.Pt(inBounds.Min.X+inBounds.Dx()/2, inBounds.Min.Y+inBounds.Dy()),
				},
				{
					Min: image.Pt(inBounds.Min.X+inBounds.Dx()/2, inBounds.Min.Y),
					Max: image.Pt(inBounds.Min.X+inBounds.Dx(), inBounds.Min.Y+inBounds.Dy()/2),
				},
				{
					Min: image.Pt(inBounds.Min.X+inBounds.Dx()/2, inBounds.Min.Y+inBounds.Dy()/2),
					Max: inBounds.Max,
				},
			}

			var wg sync.WaitGroup
			wg.Add(len(areas))
			for _, area := range areas {
				go job(area, tpindex, g.Image[i], background, outputs[i], &wg)
			}
			wg.Wait()
		} else {
			job(inBounds, tpindex, g.Image[i], background, outputs[i], nil)
		}
	}

	return outputs
}

func RgbaToGif(imgs []*image.RGBA, delays []int) *gif.GIF {
	bounds := imgs[0].Rect
	deltaMasks := make([]*image.Alpha, len(imgs)-1)
	deltas := make([]*image.RGBA, len(imgs)-1)
	palettes := make([]color.Palette, len(imgs))
	disposals := make([]byte, len(imgs)) // disposal modes for each frame
	disposals[0] = gif.DisposalBackground
	for i := 1; i < len(disposals); i++ {
		// Default disposal mode for each frame should keep the previous frame in the background.
		// If any opaque pixels are turned transparent in the new frame, the disposal method
		// should be changed to DisposalBackground
		disposals[i] = gif.DisposalPrevious
	}

	// Generate delta masks
	var wg sync.WaitGroup
	wg.Add(len(deltaMasks))
	for i := range deltaMasks {
		go func(i int) {
			defer wg.Done()

			delta := image.NewAlpha(bounds)
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
					a := imgs[i].RGBAAt(x, y)   // previous frame's pixel value
					b := imgs[i+1].RGBAAt(x, y) // current frame's pixel value
					if b.A < a.A {
						// Pixel turned from opaque to transparent; clear the entire frame
						disposals[i] = gif.DisposalBackground
						for i := range delta.Pix {
							delta.Pix[i] = 255
						}
					}
					if a.R != b.R ||
						a.G != b.G ||
						a.B != b.B ||
						a.A != b.A {
						delta.SetAlpha(x, y, color.Alpha{255})
					}
				}
			}

			if disposals[i] != gif.DisposalBackground {
				if cropped := crop(delta); !cropped.Eq(bounds) {
					delta = delta.SubImage(cropped).(*image.Alpha)
				}
			}
			deltaMasks[i] = delta
		}(i)
	}
	wg.Wait()

	// Generate delta images
	wg.Add(len(deltas))
	for i := range deltas {
		go func(i int) {
			defer wg.Done()

			deltas[i] = image.NewRGBA(deltaMasks[i].Rect)
			draw.DrawMask(deltas[i], deltas[i].Rect, imgs[i], image.Point{}, deltaMasks[i], image.Point{}, draw.Src)
		}(i)
	}
	wg.Wait()

	// Quantise palettes
	wg.Add(len(palettes))
	for i := range palettes {
		go func(i int) {
			quantizer := quantize.MedianCutQuantizer{
				AddTransparent: true,
				Aggregation:    quantize.Mean,
			}
			palettes[i] = quantizer.Quantize(make([]color.Color, 0, 256), imgs[i])
			wg.Done()
		}(i)
	}
	wg.Wait()

	gifImgs := make([]*image.Paletted, len(imgs))

	wg.Add(len(gifImgs))
	gifImgs[0] = image.NewPaletted(bounds, palettes[0])
	go rgbaToPaletted(gifImgs[0], imgs[0], &wg)
	for i := 1; i < len(gifImgs); i++ {
		gifImgs[i] = image.NewPaletted(deltas[i-1].Rect, palettes[i])
		go rgbaToPaletted(gifImgs[i], deltas[i-1], &wg)
	}
	wg.Wait()

	return &gif.GIF{
		Image:    gifImgs,
		Delay:    delays,
		Disposal: disposals,
		Config: image.Config{
			ColorModel: color.Palette{},
			Width:      bounds.Dx(),
			Height:     bounds.Dy(),
		},
	}
}

func rgbaToPaletted(dst *image.Paletted, src *image.RGBA, wg *sync.WaitGroup) {
	draw.Draw(dst, dst.Rect, src, image.Point{}, draw.Src)
	wg.Done()
}

// crop restricts the bounds of an image.Alpha to an area with opaque pixels
func crop(img *image.Alpha) image.Rectangle {
	newBounds := image.Rectangle{
		Min: img.Rect.Max,
		Max: img.Rect.Min,
	}

	for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
		for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
			if img.AlphaAt(x, y).A > 0 {
				// Found an opaque pixel
				newBounds.Min.X = min(newBounds.Min.X, x)
				newBounds.Min.Y = min(newBounds.Min.Y, y)
				newBounds.Max.X = max(newBounds.Max.X, x)
				newBounds.Max.Y = max(newBounds.Max.Y, y)
			}
		}
	}

	if newBounds.Min.X > newBounds.Max.X {
		// No opaque pixels were found; return an empty rect
		return image.Rectangle{}
	}
	return newBounds
}

func min[N constraints.Ordered](a, b N) N {
	if a < b {
		return a
	}
	return b
}

func max[N constraints.Ordered](a, b N) N {
	if a > b {
		return a
	}
	return b
}
