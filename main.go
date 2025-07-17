package main

import (
	"image"
	"log"
	"net/http"
	"strconv"

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
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "Could not parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Missing 'image' file field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	srcImg, _, err := image.Decode(file)
	if err != nil {
		http.Error(w, "Invalid image: "+err.Error(), http.StatusBadRequest)
		return
	}

	dstImg := imaging.Fill(srcImg, 2048, 1536, imaging.Center, imaging.Lanczos)

	dstImg = imaging.AdjustContrast(dstImg, 20)
	dstImg = imaging.AdjustSaturation(dstImg, 20)
	dstImg = imaging.Sharpen(dstImg, 1.5)

	w.Header().Set("Content-Type", "image/png")
	err = imaging.Encode(w, dstImg, imaging.PNG)
	if err != nil {
		http.Error(w, "Failed to encode image: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
