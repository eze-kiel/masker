package handlers

import (
	"crypto/rand"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/eze-kiel/masker/processing"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const maxUploadSize = 15 * 1024 * 1024 // 5 Mo

// Transaction contains informations about the upload
type Transaction struct {
	Success      bool
	Error        bool
	ErrorMessage string
	ID           string
}

func Handle() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/", homePage).Methods("GET")
	r.HandleFunc("/", uploadPage).Methods("POST")
	r.HandleFunc("/download/{id}", downloadData).Methods("GET")

	r.NotFoundHandler = http.HandlerFunc(notFoundPage)

	r.PathPrefix("/js/").Handler(http.StripPrefix("/js/", http.FileServer(http.Dir("js/"))))
	r.PathPrefix("/style/").Handler(http.StripPrefix("/style/", http.FileServer(http.Dir("./style/"))))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("assets/"))))

	return r
}

func homePage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("views/home.html")
	if err != nil {
		log.Fatal(err)
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Fatal(err)
	}
}

// When "/" is hit with a post request, this function is called
func uploadPage(w http.ResponseWriter, r *http.Request) {
	authorizedMIME := []string{"image/bmp", "image/gif", "image/png", "image/jpeg", "image/jpg", "image/webp"}

	// Try to parse data from post form
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		fmt.Printf("Could not parse multipart form: %v\n", err)
		return
	}

	// Get the file from form
	file, fileHeader, err := r.FormFile("filename")
	if err != nil {
		logrus.Errorf("can not parse file from form : %v\nfile %s, file header : %v\n", err, file, fileHeader)
		uploadState := Transaction{
			Error:        true,
			ErrorMessage: "No file provided.",
		}

		// Parse templates to display file's id
		tmpl, err := template.ParseFiles("views/home.html")

		if err != nil {
			log.Fatal(err)
		}

		err = tmpl.Execute(w, uploadState)
		if err != nil {
			log.Fatalf("Can not execute templates for donwload page : %v", err)
		}
		return
	}
	defer file.Close()

	fileSize := fileHeader.Size

	// Check if the file's size is accepted
	if fileSize > maxUploadSize {
		logrus.Errorf("File is too big %d instead of %d : %v\n", fileSize, maxUploadSize, err)
		uploadState := Transaction{
			Error:        true,
			ErrorMessage: "File is too big (> 15Mo).",
		}

		// Parse templates to display file's id
		tmpl, err := template.ParseFiles("views/home.html")

		if err != nil {
			log.Fatal(err)
		}

		err = tmpl.Execute(w, uploadState)
		if err != nil {
			log.Fatalf("Can not execute templates for donwload page : %v", err)
		}
		return
	}

	// implement io.Reader
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		logrus.Errorf("error reading file bytes : %v\n", err)
		return
	}

	detectedFileType := http.DetectContentType(fileBytes)

	cont := false

	for _, MIMEType := range authorizedMIME {
		if MIMEType == detectedFileType {
			cont = true
		}
	}

	if !cont {
		logrus.Errorf("File is not a picture : %s\n", detectedFileType)
		uploadState := Transaction{
			Error:        true,
			ErrorMessage: "File is not a picture.",
		}

		// Parse templates to display file's id
		tmpl, err := template.ParseFiles("views/home.html")

		if err != nil {
			log.Fatal(err)
		}

		err = tmpl.Execute(w, uploadState)
		if err != nil {
			log.Fatalf("Can not execute templates for donwload page : %v", err)
		}
		return
	}

	// Create a new name based on a random token
	// should use UUID in production
	fileName := randToken(12)

	fileEndings, err := mime.ExtensionsByType(detectedFileType)
	if err != nil {
		logrus.Errorf("did not find mime extension : %v\n", err)
		return
	}

	path := "./uploads/" + fileName[0:2] + "/" + fileName[2:4] + "/"
	err = os.MkdirAll(path, 0700)
	if err != nil && !os.IsExist(err) {
		logrus.Errorf("error creating directory : %v\n", err)
		return
	}

	newPath := filepath.Join(path, fileName+fileEndings[0])
	fmt.Printf("FileType: %s, File: %s\n", detectedFileType, newPath)

	// write file
	newFile, err := os.Create(newPath)
	if err != nil {
		logrus.Errorf("can not write in new file on disk : %v\n", err)
		return
	}
	defer newFile.Close()

	if _, err := newFile.Write(fileBytes); err != nil || newFile.Close() != nil {
		return
	}

	uploadState := Transaction{
		Success: true,
		ID:      fileName,
	}

	path = "./uploads/" + fileName[0:2] + "/" + fileName[2:4] + "/"
	processing.BlurImage(findRealFilename(path, fileName))

	// Parse templates to display file's id
	tmpl, err := template.ParseFiles("views/home.html")

	if err != nil {
		log.Fatal(err)
	}

	err = tmpl.Execute(w, uploadState)
	if err != nil {
		log.Fatalf("Can not execute templates for donwload page : %v", err)
	}
}

// DownloadData handles the download page when the method is POST.
// When an ID is provided in the form, it walks the filesystem looking
// for a file with the corresponding name.
// If the file exists, it automatically fills the response headers to start
// the download.
func downloadData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	id := vars["id"]

	if len(id) < 12 {
		log.Printf("Error in id length : %d instead of 12\n", len(id))
		http.Error(w, "File not found.", 404)
		return
	}

	path := "./uploads/" + id[0:2] + "/" + id[2:4] + "/"

	// check if the file exists
	fileName := findRealFilename(path, id)

	if fileName == "" {
		log.Printf("Filename is %s\n", fileName)
		http.Error(w, "File not found.", 404)
		return
	}

	openfile, err := os.Open(fileName)
	if err != nil {
		logrus.Errorf("error opening file %s: %v", fileName, err)
		http.Error(w, "File not found.", 404)
		return
	}
	defer openfile.Close()

	// Read the 512 first bytes of the file's headers
	FileHeader := make([]byte, 512)
	openfile.Read(FileHeader)

	FileContentType := http.DetectContentType(FileHeader)

	// Get informations about to file for the headers
	FileStat, _ := openfile.Stat()
	FileSize := strconv.FormatInt(FileStat.Size(), 10)

	parts := strings.Split(fileName, "/")
	// Send the headers
	w.Header().Set("Content-Disposition", "attachment; filename="+parts[len(parts)-1])
	w.Header().Set("Content-Type", FileContentType)
	w.Header().Set("Content-Length", FileSize)

	// Send the file
	openfile.Seek(0, 0)
	io.Copy(w, openfile)

	// Once downloaded, wipe the picture from the disk
	os.Remove(fileName)
}

func notFoundPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("views/404.html")
	if err != nil {
		log.Fatal(err)
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Fatal(err)
	}
}

// randToken returns a random token
// it must be remplaced by UUID
func randToken(len int) string {
	b := make([]byte, len)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// findRealFilename tries to find a file associated to an id
// if the file exists, his entire name is returned
func findRealFilename(path, id string) string {
	items, _ := ioutil.ReadDir(path)
	for _, item := range items {
		if strings.HasPrefix(item.Name(), id) {
			return path + item.Name()
		}
	}

	return ""
}
