// Copyright 2016 Carl Asman. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prominentcolor

import (
	"image"
	"image/color"
	"image/draw"
	"log"

	"image/jpeg"
	"os"

	"time"

	"fmt"

	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
)

// ColorBackgroundMask defines which color channels to look for color to ignore
type ColorBackgroundMask struct {
	// Setting them all to true or all to false; Treshold is used, otherwise PercDiff
	R, G, B bool

	// Treshold is the lower limit to check against for each r,g,b value, when all R,G,B that has true set should be above to be ignored (upper if all set to false)
	Treshold uint32

	// PercDiff if any of R,G,B is true (but not all), any of the other colors divided by the color value that is true, must be below PercDiff
	PercDiff float32
}

// ProcessImg process the image and mark unwanted pixels transparent.
// It checks the corners, if not all of them match the mask, we conclude it's not a clipart/solid background and do nothing
func ProcessImg(arguments int, bgmasks []ColorBackgroundMask, img image.Image) draw.Image {
	imgDraw := createDrawImage(img)
	rect := imgDraw.Bounds()

	//loop through the masks, and the first one that matches on the four corners is the one that will be used
	foundMaskThatmatched := false
	var bgmaskToUse ColorBackgroundMask
	for _, bgmask := range bgmasks {
		// Check the corners, if not all of them are the color of the mask,
		// we conclude it's not a solid background and do nothing special
		if !ignorePixel(rect.Min.X, rect.Min.Y, bgmask, &imgDraw) || !ignorePixel(rect.Min.X, rect.Max.Y-1, bgmask, &imgDraw) || !ignorePixel(rect.Max.X-1, rect.Min.Y, bgmask, &imgDraw) || !ignorePixel(rect.Max.X-1, rect.Max.Y-1, bgmask, &imgDraw) {
			continue
		}
		foundMaskThatmatched = true
		bgmaskToUse = bgmask
	}

	// no mask that we can apply
	if !foundMaskThatmatched {
		return imgDraw
	}

	ProcessImgOutline(bgmaskToUse, &imgDraw)

	// if debug argument is set, save a tmp file to be able to view what was masked out
	if IsBitSet(arguments, ArgumentDebugImage) {
		tmpFilename := fmt.Sprintf("/tmp/tmp%d.jpg", time.Now().UnixNano()/1000000)
		toimg, _ := os.Create(tmpFilename)
		defer toimg.Close()
		jpeg.Encode(toimg, imgDraw, &jpeg.Options{Quality: 100})
	}

	return imgDraw
}

// ProcessImgOutline follow the outline of the image and mark all "white" pixels as transparent
func ProcessImgOutline(bgmask ColorBackgroundMask, imgDraw *draw.Image) {

	rect := (*imgDraw).Bounds()

	var pointsToProcess []image.Point

	// points to add to start processing: corners only
	pointsToProcess = append(pointsToProcess, image.Point{X: rect.Min.X, Y: rect.Min.Y})
	pointsToProcess = append(pointsToProcess, image.Point{X: rect.Min.X, Y: rect.Max.Y - 1})
	pointsToProcess = append(pointsToProcess, image.Point{X: rect.Max.X - 1, Y: rect.Min.Y})
	pointsToProcess = append(pointsToProcess, image.Point{X: rect.Max.X - 1, Y: rect.Max.Y - 1})

	var p image.Point
	for len(pointsToProcess) > 0 {
		//pop from slice
		p, pointsToProcess = pointsToProcess[len(pointsToProcess)-1], pointsToProcess[:len(pointsToProcess)-1]

		if !isPixelTransparent(p.X, p.Y, imgDraw) && ignorePixel(p.X, p.Y, bgmask, (imgDraw)) {

			//Mark the pixel
			markPixel(p.X, p.Y, (imgDraw))
			if !isPixelTransparent(p.X, p.Y, imgDraw) {
				log.Println("ERROR: marking")
			}

			//add pixels above, below, left,right
			//unless its transparent
			if rect.Min.X < p.X {
				if !isPixelTransparent(p.X-1, p.Y, imgDraw) {
					pointsToProcess = append(pointsToProcess, image.Point{X: p.X - 1, Y: p.Y})
				}
			}

			if p.X < rect.Max.X-1 {
				if !isPixelTransparent(p.X+1, p.Y, imgDraw) {
					pointsToProcess = append(pointsToProcess, image.Point{X: p.X + 1, Y: p.Y})
				}
			}

			if rect.Min.Y < p.Y {
				if !isPixelTransparent(p.X, p.Y-1, imgDraw) {
					pointsToProcess = append(pointsToProcess, image.Point{X: p.X, Y: p.Y - 1})
				}
			}

			if p.Y < rect.Max.Y-1 {
				if !isPixelTransparent(p.X, p.Y+1, imgDraw) {
					pointsToProcess = append(pointsToProcess, image.Point{X: p.X, Y: p.Y + 1})
				}
			}
		}
	}
}

// createDrawImage creates a draw.Image so we can work with the single pixels
func createDrawImage(img image.Image) draw.Image {
	b := img.Bounds()
	cimg := image.NewRGBA(b)
	draw.Draw(cimg, b, img, b.Min, draw.Src)
	return cimg
}

// prepareImg resizes to a smaller size and remove any "white" background pixels for isolated/clipart images
func prepareImg(arguments int, bgmasks []ColorBackgroundMask, imageSize uint, orgimg image.Image) image.Image {

	if !IsBitSet(arguments, ArgumentNoCropping) {
		// crop to remove 25% on all sides
		croppedimg, err := cutter.Crop(orgimg, cutter.Config{
			Width:  int(orgimg.Bounds().Dx() / 2),
			Height: int(orgimg.Bounds().Dy() / 2),
			Mode:   cutter.Centered,
		})

		if err != nil {
			log.Println("Warning: failed cropping")
			log.Println(err)
		} else {
			orgimg = croppedimg
		}
	}

	// Don't resize if the image is smaller than imageSize
	rec := orgimg.Bounds()

	if uint(rec.Dx()) > imageSize || uint(rec.Dy()) > imageSize {
		img := resize.Resize(imageSize, 0, orgimg, resize.Lanczos3)
		return ProcessImg(arguments, bgmasks, img)
	}

	return ProcessImg(arguments, bgmasks, orgimg)
}

// markPixel sets a purple color (to make it stick out if we want to look at the image) and makes the pixel transparent
func markPixel(x, y int, img *draw.Image) {
	(*img).Set(x, y, color.RGBA{255, 0, 255, 0})
}

// isPixelTransparent returns bool if the pixel is transparent (alpha==0)
func isPixelTransparent(x, y int, img *draw.Image) bool {
	colorAt := (*img).At(x, y)
	_, _, _, a := colorAt.RGBA()
	return a == 0
}

// ignorePixel checks if the pixel should be ignored (i.e. being transparent or white)
func ignorePixel(x, y int, bgmask ColorBackgroundMask, img *draw.Image) bool {
	colorAt := (*img).At(x, y)

	r, g, b, a := colorAt.RGBA()

	if a == 0 {
		return true
	}

	//if looking for black
	if !(bgmask.R || bgmask.G || bgmask.B) {
		if r > bgmask.Treshold {
			return false
		}

		if g > bgmask.Treshold {
			return false
		}

		if b > bgmask.Treshold {
			return false
		}

		return true
	}

	//if not looking for white
	if !(bgmask.R && bgmask.G && bgmask.B) {

		var aArr, baseArr []float32

		if bgmask.R {
			baseArr = append(baseArr, float32(r))
		} else {
			aArr = append(aArr, float32(r))
		}
		if bgmask.G {
			baseArr = append(baseArr, float32(g))
		} else {
			aArr = append(aArr, float32(g))
		}
		if bgmask.B {
			baseArr = append(baseArr, float32(b))
		} else {
			aArr = append(aArr, float32(b))
		}

		for _, val := range aArr {
			for _, base := range baseArr {
				if val/base > bgmask.PercDiff {
					return false
				}
			}
		}

		return true
	}

	// Checking for white

	if bgmask.R && r < bgmask.Treshold {
		return false
	}

	if bgmask.G && g < bgmask.Treshold {
		return false
	}

	if bgmask.B && b < bgmask.Treshold {
		return false
	}

	return true
}
