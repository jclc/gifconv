package main

import (
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/jclc/gifconv"
	"golang.org/x/image/draw"
)

const usage = `usage: %s INPUT [OUTPUT]
  Convert GIFs into a series of PNGs or vice versa.

  If INPUT is a GIF, a folder named OUTPUT will be created for the output PNGs.
  If OUTPUT is not defined, the output folder's name will be derived from the
  input file.

  If INPUT is a folder containing PNGs, a GIF will be generated in OUTPUT.
`

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Printf(usage, os.Args[0])
		return
	}

	// f, _ := os.Open(os.Args[1])
	// img, _, _ := image.Decode(f)
	// switch img.(type) {
	// case *image.NRGBA:
	// 	fmt.Println("NRGBA")
	// case *image.RGBA:
	// 	fmt.Println("RGBA")
	// case *image.NRGBA64:
	// 	fmt.Println("NRGBA64")
	// case *image.RGBA64:
	// 	fmt.Println("RGBA64")
	// }

	gifToPng := true
	input := os.Args[1]

	info, err := os.Lstat(input)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if info.IsDir() {
		gifToPng = false
	} else if strings.ToLower(filepath.Ext(input)) != ".gif" {
		fmt.Println("Input file must be a GIF")
		os.Exit(1)
	}

	var output string
	if len(os.Args) == 3 {
		output = os.Args[2]
		if input == output {
			fmt.Println("Input and output must not be the same")
			os.Exit(1)
		}
	} else {
		if gifToPng {
			output = input[:len(input)-4]
		} else {
			output = input + ".gif"
		}
	}

	if gifToPng {
		makePNG(input, output)
	} else {
		makeGIF(input, output)
	}
}

func makeGIF(in, out string) {
	contents, err := os.ReadDir(in)
	if err != nil {
		fmt.Println("Error reading input directory:", err)
		os.Exit(1)
	}
	inputs := make([]string, 0, len(contents))
	for _, entry := range contents {
		if !entry.IsDir() {
			inputs = append(inputs, entry.Name())
		}
	}

	// Decode images and convert to RGBA if necessary
	images := make([]*image.RGBA, len(inputs))
	for i := range inputs {
		f, err := os.Open(filepath.Join(in, inputs[i]))
		if err != nil {
			fmt.Println("Error opening input file:", err)
			os.Exit(1)
		}
		defer f.Close()

		img, err := png.Decode(f)
		if err != nil {
			fmt.Println("Error decoding image:", err)
			os.Exit(1)
		}

		rgba, ok := img.(*image.RGBA)
		if !ok {
			rgba = image.NewRGBA(img.Bounds())
			draw.Draw(rgba, rgba.Rect, img, image.Point{}, draw.Src)
		}
		images[i] = rgba
	}

	delays := make([]int, len(images))
	for i := range delays {
		delays[i] = 100
	}
	g := gifconv.RgbaToGif(images, delays)

	f, err := os.Create(out)
	if err != nil {
		fmt.Println("Error creating output file:", err)
		os.Exit(1)
	}
	defer f.Close()

	err = gif.EncodeAll(f, g)
	if err != nil {
		fmt.Println("Error encoding GIF:", err)
		os.Exit(1)
	}
}

func makePNG(in, out string) {
	infile, err := os.Open(in)
	if err != nil {
		fmt.Println("Error opening GIF:", err)
		os.Exit(1)
	}
	defer infile.Close()

	g, err := gif.DecodeAll(infile)
	if err != nil {
		fmt.Println("Error decoding GIF:", err)
		os.Exit(1)
	}

	err = os.MkdirAll(out, 0744)
	if err != nil {
		fmt.Println("Error creating output directory:", err)
		os.Exit(1)
	}

	images := gifconv.GifToRgba(g)

	for i := range images {
		f, err := os.Create(filepath.Join(out, fmt.Sprintf("%03d.png", i)))
		if err != nil {
			fmt.Println("Error creating output file:", err)
			os.Exit(1)
		}
		defer f.Close()
		err = png.Encode(f, images[i])
		if err != nil {
			fmt.Println("Error encoding PNG:", err)
			os.Exit(1)
		}
	}
}
