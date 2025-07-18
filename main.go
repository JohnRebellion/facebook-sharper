package main

import (
	"bytes"
	"image"
	"image/color"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/disintegration/imaging"
)

func main() {
	http.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		processHandler(w, r)
	})

	port := 8000
	log.Printf("Starting server on :%d…", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}

func processHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodPost {
		http.Error(w, "POST only ", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "Could not parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("ParseMultipartForm: %v", time.Since(start))

	// Parse fields with defaults
	width, _ := strconv.Atoi(r.FormValue("width"))
	height, _ := strconv.Atoi(r.FormValue("height"))
	contrast, _ := strconv.ParseFloat(r.FormValue("contrast"), 64)
	sharpness, _ := strconv.ParseFloat(r.FormValue("sharpness"), 64)
	aspect := r.FormValue("aspect")
	resize := r.FormValue("resize")
	fillColourR := r.FormValue("fillColourR")
	fillColourG := r.FormValue("fillColourG")
	fillColourB := r.FormValue("fillColourB")

	log.Printf("Parsed fields: width=%d, height=%d, contrast=%.2f, sharpness=%.2f, aspect=%s, resize=%s, fillColorRGB=(%s) (%s) (%s)",
		width, height, contrast, sharpness, aspect, resize, fillColourR, fillColourG, fillColourB)

	// Set defaults if not provided
	if width <= 0 {
		width = 2048
	}
	if height <= 0 {
		height = 1536
	}
	if contrast == 0 {
		contrast = 20
	}
	if sharpness == 0 {
		sharpness = 1.5
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Missing 'image' file field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	log.Printf("FormFile: %v", time.Since(start))

	srcImg, _, err := image.Decode(file)
	if err != nil {
		http.Error(w, "Invalid image: "+err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("Decode: %v", time.Since(start))

	// Handle aspect ratio lock
	origBounds := srcImg.Bounds()
	origW := origBounds.Dx()
	origH := origBounds.Dy()
	targetW, targetH := width, height

	// Clamp targetW and targetH to a maximum of 2048
	const maxDim = 2048

	switch aspect {
	case "16:9":
		targetW, targetH = min(width, height*16/9), min(height, width*9/16)
	case "9:16":
		targetW, targetH = min(width, height*9/16), min(height, width*16/9)
	case "4:3":
		targetW, targetH = min(width, height*4/3), min(height, width*3/4)
	case "3:4":
		targetW, targetH = min(width, height*3/4), min(height, width*4/3)
	case "1:1":
		targetW, targetH = min(width, height), min(height, width)
	case "original":
		// Keep original aspect ratio
		origRatio := float64(origW) / float64(origH)
		reqRatio := float64(width) / float64(height)
		if reqRatio > origRatio {
			targetW = int(float64(height) * origRatio)
			targetH = height
		} else {
			targetW = width
			targetH = int(float64(width) / origRatio)
		}
	}

	// Clamp to maxDim
	if targetW > maxDim {
		targetH = int(float64(targetH) * float64(maxDim) / float64(targetW))
		targetW = maxDim
	}
	if targetH > maxDim {
		targetW = int(float64(targetW) * float64(maxDim) / float64(targetH))
		targetH = maxDim
	}

	dstImg := new(image.NRGBA)

	switch resize {
	case "fit":
		dstImg = imaging.Fill(srcImg, targetW, targetH, imaging.Center, imaging.Lanczos)
	default:
		// create a background color if fillColor is provided
		var fillColor image.Image
		if fillColourR != "" || fillColourG != "" || fillColourB != "" {
			r, _ := strconv.Atoi(fillColourR)
			g, _ := strconv.Atoi(fillColourG)
			b, _ := strconv.Atoi(fillColourB)
			fillColor = imaging.New(targetW, targetH, image.NewRGBA(image.Rect(0, 0, targetW, targetH)).ColorModel().Convert(image.NewUniform(color.RGBA{uint8(r), uint8(g), uint8(b), 255})))
			// dstImg is now resized srcImg on top of the fillColor
			dstImg = imaging.Fit(srcImg, targetW, targetH, imaging.Lanczos)
			dstImg = imaging.Overlay(fillColor, dstImg, image.Pt(targetW/2-dstImg.Bounds().Dx()/2, targetH/2-dstImg.Bounds().Dy()/2), 1.0)
		} else {
			// Default to resizing without filling
			dstImg = imaging.Resize(srcImg, targetW, targetH, imaging.Lanczos)
		}
		if fillColor == nil {
			dstImg = imaging.Resize(srcImg, targetW, targetH, imaging.Lanczos)
		}
	}

	log.Printf("Resize parameters: targetW=%d, targetH=%d", targetW, targetH)

	log.Printf("Resize: %v", time.Since(start))

	dstImg = imaging.AdjustContrast(dstImg, contrast)
	dstImg = imaging.Sharpen(dstImg, 1+sharpness/100.0)
	log.Printf("Adjustments: %v", time.Since(start))

	var buf bytes.Buffer
	err = imaging.Encode(&buf, dstImg, imaging.PNG)
	if err != nil {
		http.Error(w, "Failed to encode image: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, err = w.Write(buf.Bytes())
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
	log.Printf("Encode: %v", time.Since(start))
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
