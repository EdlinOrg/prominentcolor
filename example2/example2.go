package main

import (
	"image"
	_ "image/jpeg"
	"log"
	"os"

	"path/filepath"

	"fmt"

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

// Process images in a directory, for each image it picks out the dominant color and
// prints out an imagemagick call to resize image and use the dominant color as padding for the background
// it saves tmp files in /tmp/ with the masked bit marked as pink
func main() {

	inputPattern := "../example/*.jpg"
	outputDirectory := "/tmp/"

	files, err := filepath.Glob(inputPattern)

	if nil != err {
		log.Println(err)
		log.Println("Error: failed glob")
		return
	}

	for _, file := range files {
		img, err := loadImage(file)
		if nil != err {
			log.Println(err)
			log.Printf("Error: failed loading %s\n", file)
			continue
		}
		cols, err := prominentcolor.KmeansWithArgs(prominentcolor.ArgumentNoCropping|prominentcolor.ArgumentDebugImage, img)
		if err != nil {
			log.Println(err)
			continue
		}
		col := cols[0].AsString()
		base := filepath.Base(file)
		fmt.Printf("convert %s -resize 800x356 -background '#%s' -gravity center -extent 800x356 %s%s\n", base, col, outputDirectory, base)
	}

}
