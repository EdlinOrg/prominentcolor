package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"io/ioutil"
	"log"
	"os"

	"strconv"

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

	str := "<table><tr>"
	for _, color := range colorRange {
		str += fmt.Sprintf("<td style=\"background-color: #%s;width:200px;height:50px;text-align:center;\">#%s %d</td>", color.AsString(), color.AsString(), color.Cnt)
	}
	str += "</tr></table>"
	return str
}

func outputTitle(str string) string {
	return "<h3>" + str + "</h3>"
}

func processBatch(k int, bitarr []int, img image.Image) string {

	str := ""
	prefix := "K=" + strconv.Itoa(k)

	resizeSize := uint(prominentcolor.DefaultSize)

	bgmasks := prominentcolor.GetDefaultMasks()

	for i := 0; i < len(bitarr); i++ {
		res, err := prominentcolor.KmeansWithAll(k, img, bitarr[i], resizeSize, bgmasks)
		if err != nil {
			log.Println(err)
			continue
		}
		str += outputTitle(prefix + bitInfo(bitarr[i]))
		str += outputColorRange(res)
	}

	return str
}

func bitInfo(bits int) string {

	str := ""

	if prominentcolor.IsBitSet(bits, prominentcolor.ArgumentSeedRandom) {
		str += ", Random seed"
	} else {
		str += ", Kmeans++"
	}

	if prominentcolor.IsBitSet(bits, prominentcolor.ArgumentAverageMean) {
		str += ", Mean"
	} else {
		str += ", Median"
	}

	if prominentcolor.IsBitSet(bits, prominentcolor.ArgumentLAB) {
		str += ", LAB"
	} else {
		str += ", RGB"
	}

	if prominentcolor.IsBitSet(bits, prominentcolor.ArgumentNoCropping) {
		str += ", No cropping"
	} else {
		str += ", Cropping center"
	}

	return str
}

func main() {

	outputDirectory := "./"
	dataDirectory := "./"

	itemIds := [...]int{28922730122, 28411051634, 27417460620, 28930160605, 25535163354, 26939476984, 6833735316, 8527042251}

	str := "<html><body><h1>Colors listed in order of dominance: hex color followed by number of entries</h1><table border=\"1\">"

	for _, itemId := range itemIds {

		kk := []int{prominentcolor.ArgumentAverageMean | prominentcolor.ArgumentNoCropping, prominentcolor.ArgumentNoCropping, prominentcolor.ArgumentDefault}

		filename := dataDirectory + strconv.Itoa(itemId) + ".jpg"
		img, err := loadImage(filename)

		if err != nil {
			log.Printf("Error loading image %s\n", filename)
			log.Println(err)
			continue
		}

		str += "<tr><td><img src=\"" + filename + "\" width=\"200\" border=\"1\"></td><td>"

		str += processBatch(3, kk, img)

		str += "</td></tr>"
	}

	str += "</table></body><html>"

	d1 := []byte(str)
	err := ioutil.WriteFile(outputDirectory+"output.html", d1, 0644)
	if err != nil {
		panic(err)
	}

}
