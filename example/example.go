package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"io/ioutil"
	"log"
	"os"
	"strings"

	prominentcolor ".."
)

func loadImage(fileInput string) (image.Image, error) {
	f, err := os.Open(fileInput)
	defer f.Close()
	if err != nil {
		log.Println("File not found:", fileInput)
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func outputColorRange(colorRange []prominentcolor.ColorItem) string {
	var buff strings.Builder
	buff.WriteString("<table><tr>")
	for _, color := range colorRange {
		buff.WriteString(fmt.Sprintf("<td style=\"background-color: #%s;width:200px;height:50px;text-align:center;\">#%s %d</td>", color.AsString(), color.AsString(), color.Cnt))
	}
	buff.WriteString("</tr></table>")
	return buff.String()
}

func outputTitle(str string) string {
	return "<h3>" + str + "</h3>"
}

func processBatch(k int, bitarr []int, img image.Image) string {
	var buff strings.Builder

	prefix := fmt.Sprintf("K=%d, ", k)
	resizeSize := uint(prominentcolor.DefaultSize)
	bgmasks := prominentcolor.GetDefaultMasks()

	for i := 0; i < len(bitarr); i++ {
		res, err := prominentcolor.KmeansWithAll(k, img, bitarr[i], resizeSize, bgmasks)
		if err != nil {
			log.Println(err)
			continue
		}
		buff.WriteString(outputTitle(prefix + bitInfo(bitarr[i])))
		buff.WriteString(outputColorRange(res))
	}

	return buff.String()
}

func bitInfo(bits int) string {
	list := make([]string, 0, 4)
	// random seed or Kmeans++
	if prominentcolor.IsBitSet(bits, prominentcolor.ArgumentSeedRandom) {
		list = append(list, "Random seed")
	} else {
		list = append(list, "Kmeans++")
	}
	// Mean or median
	if prominentcolor.IsBitSet(bits, prominentcolor.ArgumentAverageMean) {
		list = append(list, "Mean")
	} else {
		list = append(list, "Median")
	}
	// LAB or RGB
	if prominentcolor.IsBitSet(bits, prominentcolor.ArgumentLAB) {
		list = append(list, "LAB")
	} else {
		list = append(list, "RGB")
	}
	// Cropping or not
	if prominentcolor.IsBitSet(bits, prominentcolor.ArgumentNoCropping) {
		list = append(list, "No cropping")
	} else {
		list = append(list, "Cropping center")
	}
	// Done
	return strings.Join(list, ", ")
}

func main() {
	// Prepare
	outputDirectory := "./"
	dataDirectory := "./"

	var buff strings.Builder
	buff.WriteString("<html><body><h1>Colors listed in order of dominance: hex color followed by number of entries</h1><table border=\"1\">")

	// for each file within working directory
	files, err := ioutil.ReadDir(dataDirectory)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		filename := f.Name()
		// Only process jpg
		if !strings.HasSuffix(filename, ".jpg") {
			continue
		}
		// Define the differents sets of params
		kk := []int{
			prominentcolor.ArgumentAverageMean | prominentcolor.ArgumentNoCropping,
			prominentcolor.ArgumentNoCropping,
			prominentcolor.ArgumentDefault,
		}
		// Load the image
		img, err := loadImage(filename)
		if err != nil {
			log.Printf("Error loading image %s\n", filename)
			log.Println(err)
			continue
		}
		// Process & html output
		buff.WriteString("<tr><td><img src=\"" + filename + "\" width=\"200\" border=\"1\"></td><td>")
		buff.WriteString(processBatch(3, kk, img))
		buff.WriteString("</td></tr>")
	}

	// Finalize the html output
	buff.WriteString("</table></body><html>")

	// And write it to the disk
	if err = ioutil.WriteFile(outputDirectory+"output.html", []byte(buff.String()), 0644); err != nil {
		panic(err)
	}
}
