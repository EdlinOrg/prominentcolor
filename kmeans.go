// Copyright 2016 Carl Asman. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package prominentcolor finds the K most dominant/prominent colors in an image
package prominentcolor

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math/rand"

	"sort"

	"time"

	"github.com/lucasb-eyer/go-colorful"
)

const (
	// ArgumentDefault default settings
	ArgumentDefault int = 0
	// ArgumentSeedRandom randomly pick initial values (instead of K-means++)
	ArgumentSeedRandom = 1 << iota
	// ArgumentAverageMean take the mean value when determining the centroid color (instead of median)
	ArgumentAverageMean
	// ArgumentNoCropping do not crop background that is considered "white"
	ArgumentNoCropping
	// ArgumentLAB (experimental, it seems to be buggy in some cases): uses LAB instead of RGB when measuring distance
	ArgumentLAB
	// ArgumentDebugImage saves a tmp file in /tmp/ where the area that has been cut away by the mask is marked pink
	// useful when figuring out what values to pick for the masks
	ArgumentDebugImage
)

const (
	// DefaultK is the k used as default
	DefaultK = 3
	// DefaultSize is the default size images are re-sized to
	DefaultSize = 80
)

var (
	// MaskWhite "constant" for white mask (for ease of re-use for other mask arrays)
	MaskWhite = ColorBackgroundMask{R: true, G: true, B: true, Treshold: uint32(0xc000)}
	// MaskBlack "constant" for black mask (for ease of re-use for other mask arrays)
	MaskBlack = ColorBackgroundMask{R: false, G: false, B: false, Treshold: uint32(0x5000)}
	// MaskGreen "constant" for green mask (for ease of re-use for other mask arrays)
	MaskGreen = ColorBackgroundMask{R: false, G: true, B: false, PercDiff: 0.9}
)

// ColorRGB contains the color values
type ColorRGB struct {
	R, G, B uint32
}

// ColorItem contains color and have many occurrences of this color found
type ColorItem struct {
	Color ColorRGB
	Cnt   int
}

// AsString gives back the color in hex as 6 character string
func (c *ColorItem) AsString() string {
	return fmt.Sprintf("%.2X%.2X%.2X", c.Color.R, c.Color.G, c.Color.B)
}

// createColor returns ColorItem struct unless it was a transparent color
func createColor(c color.Color) (ColorItem, bool) {
	r, g, b, a := c.RGBA()

	if a == 0 {
		// transparent pixels are ignored
		return ColorItem{}, true
	}

	divby := uint32(256.0)
	return ColorItem{Color: ColorRGB{R: r / divby, G: g / divby, B: b / divby}}, false
}

// IsBitSet check if "lookingfor" is set in "bitset"
func IsBitSet(bitset int, lookingfor int) bool {
	return lookingfor == (bitset & lookingfor)
}

// GetDefaultMasks returns the masks that are used for the default settings
func GetDefaultMasks() []ColorBackgroundMask {
	return []ColorBackgroundMask{MaskWhite, MaskBlack, MaskGreen}
}

// Kmeans uses the default: k=3, Kmeans++, Median, crop center, resize to 80 pixels, mask out white/black/green backgrounds
// It returns an array of ColorItem which are three centroids, sorted according to dominance (most frequent first).
func Kmeans(orgimg image.Image) (centroids []ColorItem, err error) {
	return KmeansWithAll(DefaultK, orgimg, ArgumentDefault, DefaultSize, GetDefaultMasks())
}

// KmeansWithArgs takes arguments which consists of the bits, see constants Argument*
func KmeansWithArgs(arguments int, orgimg image.Image) (centroids []ColorItem, err error) {
	return KmeansWithAll(DefaultK, orgimg, arguments, DefaultSize, GetDefaultMasks())
}

// KmeansWithAll takes additional arguments to define k, arguments (see constants Argument*), size to resize and masks to use
func KmeansWithAll(k int, orgimg image.Image, arguments int, imageReSize uint, bgmasks []ColorBackgroundMask) ([]ColorItem, error) {

	img := prepareImg(arguments, bgmasks, imageReSize, orgimg)

	allColors, _ := extractColorsAsArray(img)

	numColors := len(allColors)

	if numColors == 0 {
		return nil, fmt.Errorf("Failed, no non-alpha pixels found (either fully transparent image, or the ColorBackgroundMask removed all pixels)")
	}

	if numColors == 1 {
		return allColors, nil
	}

	if numColors <= k {
		sortCentroids(allColors)
		return allColors, nil
	}

	centroids, err := kmeansSeed(k, allColors, arguments)
	if err != nil {
		return nil, err
	}

	cent := make([][]ColorItem, k)

	//initialize
	cent[0] = allColors
	for i := 1; i < k; i++ {
		cent[i] = []ColorItem{}
	}

	//rounds is a safety net to make sure we terminate if its a bug in our distance function (or elsewhere) that makes k-means not terminate
	rounds := 0
	maxRounds := 5000
	changes := 1

	for changes > 0 && rounds < maxRounds {
		changes = 0
		tmpCent := make([][]ColorItem, k)
		for i := 0; i < k; i++ {
			tmpCent[i] = []ColorItem{}
		}

		for i := 0; i < k; i++ {
			for _, aColor := range cent[i] {
				closestCentroid := findClosest(arguments, aColor, centroids)

				tmpCent[closestCentroid] = append(tmpCent[closestCentroid], aColor)
				if closestCentroid != i {
					changes++
				}
			}
		}
		cent = tmpCent
		centroids = calculateCentroids(cent, arguments)
		rounds++
	}

	if rounds >= maxRounds {
		log.Println("Warning: terminated k-means due to max number of iterations")
	}

	sortCentroids(centroids)
	return centroids, nil
}

// ByColorCnt makes the ColorItem sortable
type byColorCnt []ColorItem

func (a byColorCnt) Len() int      { return len(a) }
func (a byColorCnt) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byColorCnt) Less(i, j int) bool {
	if a[i].Cnt == a[j].Cnt {
		return a[i].AsString() < a[j].AsString()
	}
	return a[i].Cnt < a[j].Cnt
}

// sortCentroids sorts them from most dominant color descending
func sortCentroids(centroids []ColorItem) {
	sort.Sort(sort.Reverse(byColorCnt(centroids)))
}

func calculateCentroids(cent [][]ColorItem, arguments int) []ColorItem {
	var centroids []ColorItem

	for _, colors := range cent {

		var meanColor ColorItem
		if IsBitSet(arguments, ArgumentAverageMean) {
			meanColor = mean(colors)
		} else {
			meanColor = median(colors)
		}

		centroids = append(centroids, meanColor)
	}

	return centroids
}

// mean calculate the mean color values from an array of colors
func mean(colors []ColorItem) ColorItem {

	var r, g, b float64

	r, g, b = 0.0, 0.0, 0.0

	cntInThisBucket := 0
	for _, aColor := range colors {
		cntInThisBucket += aColor.Cnt
		r += float64(aColor.Color.R)
		g += float64(aColor.Color.G)
		b += float64(aColor.Color.B)
	}

	theSize := float64(len(colors))

	return ColorItem{Cnt: cntInThisBucket, Color: ColorRGB{R: uint32(r / theSize), G: uint32(g / theSize), B: uint32(b / theSize)}}
}

// median calculate the median color from an array of colors
func median(colors []ColorItem) ColorItem {

	var rValues, gValues, bValues []int

	cntInThisBucket := 0

	for _, aColor := range colors {
		cntInThisBucket += aColor.Cnt
		rValues = append(rValues, int(aColor.Color.R))
		gValues = append(gValues, int(aColor.Color.G))
		bValues = append(bValues, int(aColor.Color.B))
	}

	retR := 0
	if 0 != len(rValues) {
		sort.Ints(rValues)
		retR = rValues[int(len(rValues)/2)]
	}

	retG := 0
	if 0 != len(gValues) {
		sort.Ints(gValues)
		retG = gValues[int(len(gValues)/2)]
	}

	retB := 0
	if 0 != len(bValues) {
		sort.Ints(bValues)
		retB = bValues[int(len(bValues)/2)]
	}

	return ColorItem{Cnt: cntInThisBucket, Color: ColorRGB{R: uint32(retR), G: uint32(retG), B: uint32(retB)}}
}

// extractColorsAsArray counts the number of occurrences of each color in the image, returns array and numPixels
func extractColorsAsArray(img image.Image) ([]ColorItem, int) {
	m, numPixels := extractColors(img)
	v := make([]ColorItem, len(m))
	idx := 0
	for _, value := range m {
		v[idx] = value
		idx++
	}

	return v, numPixels
}

// extractColors counts the number of occurrences of each color in the image, returns map
func extractColors(img image.Image) (map[string]ColorItem, int) {

	m := make(map[string]ColorItem)

	numPixels := 0
	data := img.Bounds()
	for x := data.Min.X; x < data.Max.X; x++ {
		for y := data.Min.Y; y < data.Max.Y; y++ {
			colorAt := img.At(x, y)
			colorItem, ignore := createColor(colorAt)
			if ignore {
				continue
			}
			numPixels++
			asString := colorItem.AsString()
			value, ok := m[asString]
			if ok {
				value.Cnt++
				m[asString] = value
			} else {
				colorItem.Cnt = 1
				m[asString] = colorItem
			}
		}
	}
	return m, numPixels
}

// findClosest returns the index of the closest centroid to the color "c"
func findClosest(arguments int, c ColorItem, centroids []ColorItem) int {

	centLen := len(centroids)

	closestIdx := 0
	closestDistance := distance(arguments, c, centroids[0])

	for i := 1; i < centLen; i++ {
		distance := distance(arguments, c, centroids[i])
		if distance < closestDistance {
			closestIdx = i
			closestDistance = distance
		}
	}
	return closestIdx
}

// distance returns the distance between two colors
func distance(arguments int, c ColorItem, p ColorItem) float64 {
	if IsBitSet(arguments, ArgumentLAB) {
		return distanceLAB(c, p)
	}
	return distanceRGB(c, p)
}

func distanceLAB(c ColorItem, p ColorItem) float64 {
	errmsg := "Warning: LAB failed, fallback to RGB"

	a, err := colorful.Hex("#" + c.AsString())
	if err != nil {
		log.Fatal(err)
		log.Println(errmsg)
		return distanceRGB(c, p)
	}

	b, err2 := colorful.Hex("#" + p.AsString())
	if err2 != nil {
		log.Fatal(err2)
		log.Println(errmsg)
		return distanceRGB(c, p)
	}

	return a.DistanceLab(b)
}

func distanceRGB(c ColorItem, p ColorItem) float64 {
	r := c.Color.R
	g := c.Color.G
	b := c.Color.B

	r2 := p.Color.R
	g2 := p.Color.G
	b2 := p.Color.B

	//sqrt not needed since we just want to compare distances to each other
	return float64((r-r2)*(r-r2) + (g-g2)*(g-g2) + (b-b2)*(b-b2))
}

// kmeansSeed calculates the initial cluster centroids
func kmeansSeed(k int, allColors []ColorItem, arguments int) ([]ColorItem, error) {
	if k > len(allColors) {
		return nil, fmt.Errorf("Failed, k larger than len(allColors): %d vs %d\n", k, len(allColors))
	}

	rand.Seed(time.Now().UnixNano())

	if IsBitSet(arguments, ArgumentSeedRandom) {
		return kmeansSeedRandom(k, allColors), nil
	}
	return kmeansPlusPlusSeed(k, arguments, allColors), nil
}

// kmeansSeedRandom picks k random points as initial centroids
func kmeansSeedRandom(k int, allColors []ColorItem) []ColorItem {
	var centroids []ColorItem

	taken := make(map[int]bool)

	for i := 0; i < k; i++ {
		idx := rand.Intn(len(allColors))

		//check if we already taken this one
		_, ok := taken[idx]
		if ok {
			i--
			continue
		}
		taken[idx] = true
		centroids = append(centroids, allColors[idx])
	}
	return centroids
}

// kmeansPlusPlusSeed picks initial centroids using K-Means++
func kmeansPlusPlusSeed(k int, arguments int, allColors []ColorItem) []ColorItem {
	var centroids []ColorItem

	taken := make(map[int]bool)

	initIdx := rand.Intn(len(allColors))
	centroids = append(centroids, allColors[initIdx])
	taken[initIdx] = true

	for kk := 1; kk < k; kk++ {

		totaldistances := 0.0
		var point2distance []float64

		for j := 0; j < len(allColors); j++ {

			_, ok := taken[j]
			if ok {
				point2distance = append(point2distance, 0.0)
				continue
			}

			minDistanceToCluster := -1.0
			for i := 0; i < len(centroids); i++ {
				d := distance(arguments, centroids[i], allColors[j])
				if minDistanceToCluster == -1.0 || d < minDistanceToCluster {
					minDistanceToCluster = d
				}
			}

			squareDistance := minDistanceToCluster * minDistanceToCluster
			totaldistances += squareDistance
			point2distance = append(point2distance, squareDistance)
		}

		rndpoint := rand.Float64() * totaldistances

		sofar := 0.0
		for j := 0; j < len(point2distance); j++ {
			if rndpoint <= sofar {
				centroids = append(centroids, allColors[j])
				taken[j] = true
				break
			}
			sofar += point2distance[j]
		}
	}

	return centroids
}
