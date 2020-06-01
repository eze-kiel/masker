package processing

import (
	"fmt"
	"image"

	"gocv.io/x/gocv"
)

func BlurImage(name string) {

	// xmlFile := "xml/haarcascade_frontalface_default.xml"

	var classifierList = []string{"xml/haarcascade_frontalface_default.xml", "xml/haarcascade_profileface.xml"}

	img := gocv.IMRead(name, gocv.IMReadColor)
	if img.Empty() {
		fmt.Printf("Error reading image from: %v\n", name)
		return
	}
	defer img.Close()

	classifier := gocv.NewCascadeClassifier()
	defer classifier.Close()

	for _, class := range classifierList {
		if !classifier.Load(class) {
			fmt.Printf("Error reading cascade file: %v\n", class)
			return
		}

		// detect faces
		rects := classifier.DetectMultiScale(img)
		for _, r := range rects {
			imgFace := img.Region(r)

			gocv.GaussianBlur(imgFace, &imgFace, image.Pt(75, 75), 0, 0, gocv.BorderDefault)
			gocv.Blur(imgFace, &imgFace, image.Pt(75, 75))
			imgFace.Close()
		}

		gocv.IMWrite(name, img)
	}

	fmt.Printf("Successfully blurred %s\n", name)
}
